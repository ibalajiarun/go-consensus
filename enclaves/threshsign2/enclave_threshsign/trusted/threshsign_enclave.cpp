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
#include <stdarg.h>
#include <stdio.h> /* vsnprintf */
#include <string.h>
#include <memory>
#include <array>

#include <sgx_tgmp.h>

#include "threshsign_enclave.h"
#include "threshsign_enclave_t.h" /* print_string */

#include <trusted_libff/libff/algebra/curves/alt_bn128/alt_bn128_init.hpp>
#include <trusted_libff/libff/algebra/curves/alt_bn128/alt_bn128_pp.hpp>

#define BUF_LEN 256

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

libff::alt_bn128_G1 shareSign(
    const libff::alt_bn128_Fr &shareKey,
    const libff::alt_bn128_G1 &msgHash)
{
    // TODO: assuming message is really H(message)
    return shareKey * msgHash;
}

std::string *stringFromFq(libff::alt_bn128_Fq *_fq)
{
    mpz_t t;
    mpz_init(t);

    _fq->as_bigint().to_mpz(t);

    char arr[mpz_sizeinbase(t, 10) + 2];

    char *tmp = mpz_get_str(arr, 10, t);
    mpz_clear(t);

    return new std::string(tmp);
}

std::string *stringFromG1(libff::alt_bn128_G1 *_g1)
{
    _g1->to_affine_coordinates();

    auto sX = stringFromFq(&_g1->X);
    auto sY = stringFromFq(&_g1->Y);

    auto sG1 = new std::string(*sX + ":" + *sY);

    delete (sX);
    delete (sY);

    return sG1;
}

bool initialized;
std::shared_ptr<libff::alt_bn128_Fr> shareKey;

sgx_status_t ecall_tss_init(const char *key)
{
    sgx_status_t ret = SGX_SUCCESS;
    if (initialized)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto out;
    }

    libff::init_alt_bn128_params();
    shareKey = std::make_shared<libff::alt_bn128_Fr>(key);

    initialized = true;
out:
    return ret;
}

sgx_status_t ecall_tss_sign(
    const char *_hashXStr,
    const char *_hashYStr,
    char *signature,
    unsigned *sig_len)
{
    sgx_status_t ret = SGX_SUCCESS;
    if (!initialized)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto out;
    }

    {
        libff::alt_bn128_Fq hashX(_hashXStr);
        libff::alt_bn128_Fq hashY(_hashYStr);
        libff::alt_bn128_Fq hashZ = 1;

        libff::alt_bn128_G1 msgHash(hashX, hashY, hashZ);

        libff::alt_bn128_G1 sig = shareSign(*shareKey, msgHash);

        auto sigStr = stringFromG1(&sig);

        memset(signature, 0, BUF_LEN);
        strncpy(signature, sigStr->c_str(), BUF_LEN);
        *sig_len = sigStr->size();

        delete sigStr;
    }

out:
    return ret;
}