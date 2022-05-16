/*
 * Copyright (C) 2011-2020 Intel Corporation. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 *   * Redistributions of source code must retain the above copyright
 *     notice, this list of conditions and the following disclaimer.
 *   * Redistributions in binary form must reproduce the above copyright
 *     notice, this list of conditions and the following disclaimer in
 *     the documentation and/or other materials provided with the
 *     distribution.
 *   * Neither the name of Intel Corporation nor the names of its
 *     contributors may be used to endorse or promote products derived
 *     from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

#include "usig_enclave.h"
#include "usig_enclave_t.h" /* print_string */

#include <stdarg.h>
#include <stdio.h> /* vsnprintf */
#include <string.h>
#include <atomic>
#include <vector>
#include <memory>

#include <sgx_trts.h>
#include <sgx_tseal.h>

#include "edd25519/crypto_api.h"

#define BUF_LEN 1024

/* 
 * printf: 
 *   Invokes OCALL to display the enclave buffer to the terminal.
 */
int printf(const char *fmt, ...)
{
    char buf[BUFSIZ] = {'\0'};
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(buf, BUFSIZ, fmt, ap);
    va_end(ap);
    ocall_print_string(buf);
    return (int)strnlen(buf, BUFSIZ - 1) + 1;
}

static void debug(const char *fmt, ...)
    __attribute__((__format__(printf, 1, 2)));

static void
debug(const char *fmt, ...)
{
    char buf[BUFSIZ];
    va_list ap;

    va_start(ap, fmt);
    vsnprintf(buf, BUFSIZ, fmt, ap);
    va_end(ap);
    ocall_print_string(buf);
}

static void
dump(const char *preamble, const void *sv, size_t l)
{
    const uint8_t *s = (const uint8_t *)sv;
    size_t i;

    debug("%s (len %zu):\n", preamble, l);
    for (i = 0; i < l; i++)
    {
        if (i % 16 == 0)
            debug("%04zu: ", i);
        debug("%02x", s[i]);
        if (i % 16 == 15 || i == l - 1)
            debug("\n");
    }
}

static sgx_ec256_private_t usig_priv_key;
static unsigned char usig_mac_key[BUF_LEN];
static unsigned usig_mackey_len;
static unsigned char edd25519_sk[BUF_LEN];
static unsigned edd25519_sk_len;
static bool initialized;

static std::vector<std::atomic<uint64_t>*> usig_counters;

typedef struct __attribute__((__packed__))
{
    sgx_sha256_hash_t digest;
    uint64_t counter;
} usig_cert_data_t;

sgx_status_t ecall_usig_init(const char *_mackey, uint32_t _mackey_len,
                             sgx_ec256_private_t _privkey,
                             uint32_t num_counters,
                             const char *_edd25519_sk, uint32_t _edd25519_sk_len)
{
    sgx_status_t ret = SGX_SUCCESS;

    if (initialized)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto out;
    }

    usig_counters.resize(num_counters);
    for (auto& p : usig_counters) {
        p = new std::atomic<uint64_t>(1);
    }

    strncpy((char *)usig_mac_key, _mackey, _mackey_len);
    usig_mackey_len = _mackey_len;

    strncpy((char *)edd25519_sk, _edd25519_sk, _edd25519_sk_len);
    edd25519_sk_len = _edd25519_sk_len;
    edd25519_sk[edd25519_sk_len] = '\0';

    memcpy(&usig_priv_key, (void *)&_privkey, sizeof(sgx_ec256_private_t));

    initialized = true;

out:
    return ret;
}

sgx_status_t ecall_usig_create_ui_sig(sgx_sha256_hash_t digest,
                                      uint64_t *counter,
                                      sgx_ec256_signature_t *signature)
{
    sgx_status_t ret;
    sgx_ecc_state_handle_t ecc_handle;
    sgx_ec256_signature_t signature_buf;
    usig_cert_data_t data;

    if (!initialized)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto out;
    }

    ret = sgx_ecc256_open_context(&ecc_handle);
    if (ret != SGX_SUCCESS)
    {
        goto out;
    }

    memcpy(data.digest, digest, sizeof(data.digest));
    *counter = data.counter = usig_counters[0]->fetch_add(1, std::memory_order_relaxed);

    ret = sgx_ecdsa_sign((uint8_t *)&data, sizeof(data),
                         &usig_priv_key, &signature_buf, ecc_handle);
    if (ret != SGX_SUCCESS)
    {
        goto close_context;
    }

    memcpy(signature, &signature_buf, sizeof(signature_buf));

close_context:
    sgx_ecc256_close_context(ecc_handle);
out:
    return ret;
}

sgx_status_t ecall_usig_create_ui_mac(sgx_sha256_hash_t digest,
                                      uint64_t *counter,
                                      uint8_t *cert, uint32_t cert_size)
{
    sgx_status_t ret;
    usig_cert_data_t data;

    if (!initialized)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto out;
    }

    memcpy(data.digest, digest, sizeof(data.digest));
    *counter = data.counter = usig_counters[0]->fetch_add(1, std::memory_order_relaxed);

    ret = sgx_hmac_sha256_msg((uint8_t *)&data, sizeof(data),
                              usig_mac_key, usig_mackey_len,
                              cert, cert_size);
    if (ret != SGX_SUCCESS)
    {
        goto out;
    }

out:
    return ret;
}

sgx_status_t ecall_usig_verify_ui_mac(sgx_sha256_hash_t digest,
                                      uint64_t counter,
                                      const uint8_t *cert, uint32_t cert_size,
                                      uint8_t *identical)
{
    sgx_status_t ret;
    usig_cert_data_t data;
    unsigned char *cert_buf;

    if (!initialized)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto out;
    }

    cert_buf = (unsigned char *)calloc(cert_size, sizeof(char));

    memcpy(data.digest, digest, sizeof(data.digest));
    data.counter = counter;

    ret = sgx_hmac_sha256_msg((uint8_t *)&data, sizeof(data),
                              usig_mac_key, usig_mackey_len,
                              cert_buf, cert_size);
    if (ret != SGX_SUCCESS)
    {
        goto close_context;
    }

    *identical = ((uint8_t)memcmp(cert, cert_buf, cert_size) == 0);

close_context:
    free(cert_buf);
out:
    return ret;
}

sgx_status_t ecall_usig_create_ui_sig_edd25519(
    sgx_sha256_hash_t digest,
    uint32_t ctr_id,
    uint64_t *counter,
    uint8_t *signature, uint32_t signature_len)
{
    size_t o = 0;
    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    uint8_t signbuf[sizeof(ctr_id) + sizeof(*counter) + crypto_hash_sha256_BYTES];
    uint8_t sign[crypto_sign_ed25519_BYTES + sizeof(signbuf)];
    unsigned long long smlen;

    if (!initialized)
    {
        goto out;
    }

    *counter = usig_counters[ctr_id]->fetch_add(1, std::memory_order_relaxed);

    signbuf[o++] = (uint8_t)(ctr_id & 0xff);
	signbuf[o++] = (uint8_t)((ctr_id >> 8) & 0xff);
	signbuf[o++] = (uint8_t)((ctr_id >> 16) & 0xff);
	signbuf[o++] = (uint8_t)((ctr_id >> 24) & 0xff);

	signbuf[o++] = (uint8_t)(*counter & 0xff);
	signbuf[o++] = (uint8_t)((*counter >> 8) & 0xff);
	signbuf[o++] = (uint8_t)((*counter >> 16) & 0xff);
	signbuf[o++] = (uint8_t)((*counter >> 24) & 0xff);
	signbuf[o++] = (uint8_t)((*counter >> 32) & 0xff);
	signbuf[o++] = (uint8_t)((*counter >> 40) & 0xff);
	signbuf[o++] = (uint8_t)((*counter >> 48) & 0xff);
    signbuf[o++] = (uint8_t)((*counter >> 56) & 0xff);
	memcpy(signbuf + o, digest, crypto_hash_sha256_BYTES);
	o += crypto_hash_sha256_BYTES;
    if (o != sizeof(signbuf)) {
		debug("%s: bad sign buf len %zu, expected %zu",
		    __func__, o, sizeof(signbuf));
		goto out;
	}

    /* create and encode signature */
    smlen = sizeof(signbuf);
    if (crypto_sign_ed25519(sign, &smlen, signbuf, sizeof(signbuf), edd25519_sk) != 0)
    {
        debug("%s: crypto_sign_ed25519 failed", __func__);
        goto out;
    }
    if (smlen <= sizeof(signbuf)) {
		debug("%s: bad sign smlen %llu, expected min %zu", __func__,
		    smlen, sizeof(signbuf) + 1);
		goto out;
	}
	if (signature_len != (size_t)(smlen - sizeof(signbuf))) {
		debug("%s: sig_len wrong", __func__);
		goto out;
	}

    memcpy(signature, &sign, signature_len);
    ret = SGX_SUCCESS;
out:
    return ret;
}