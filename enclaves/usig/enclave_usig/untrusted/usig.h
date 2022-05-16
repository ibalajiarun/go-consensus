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


#ifndef _APP_H_
#define _APP_H_

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>

#include <sgx_urts.h>
#include <sgx_tcrypto.h>

#if defined(__cplusplus)
extern "C" {
#endif

    sgx_status_t usig_init(
        const char *enclave_file_path,
        const char *skey, unsigned skey_len,
        sgx_ec256_private_t _privkey,
        uint32_t num_counters,
        const char *skey_edd25519, unsigned skey_edd25519_len,
        uint64_t* enclave_id);

    sgx_status_t usig_create_ui_sig_edd25519(
        uint64_t eid,
        sgx_sha256_hash_t digest,
        uint32_t counter_id,
        uint64_t *counter,
        uint8_t *signature,
        uint32_t signature_len);

    sgx_status_t usig_create_ui_sig(
        uint64_t eid,
        sgx_sha256_hash_t digest,
        uint64_t *counter,
        sgx_ec256_signature_t *signature);

    sgx_status_t usig_create_ui_mac(
        uint64_t eid,
        sgx_sha256_hash_t digest,
        uint64_t *counter,
        uint8_t *cert, uint32_t cert_size);

    sgx_status_t usig_verify_ui_mac(
        uint64_t eid,
        sgx_sha256_hash_t digest,
        uint64_t counter,
        const uint8_t *cert, uint32_t cert_size,
        uint8_t *identical);

    sgx_status_t usig_destroy(uint64_t eid);

#if defined(__cplusplus)
}
#endif

#endif /* !_APP_H_ */
