/*
 * Copyright (C) 2011-2019 Intel Corporation. All rights reserved.
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

#include "sgx_error.h" /* sgx_status_t */
#include "sgx_eid.h"   /* sgx_enclave_id_t */

#if defined(__cplusplus)
extern "C"
{
#endif

    struct threshsign;
    typedef struct threshsign threshsign_t;

    int threshsign_setup(const char *enclave_file,
                         unsigned enclave_file_len,
                         uint32_t threshold,
                         uint32_t total,
                         uint32_t node_index,
                         const char *skey,
                         unsigned skey_len,
                         const char **pkey,
                         unsigned pkey_count,
                         char *cpkey,
                         unsigned *cpkey_len,
                         uint32_t num_counters,
                         threshsign_t **ts);

    int threshsign_sign(threshsign_t *ts,
                        const char *hash,
                        unsigned hash_len,
                        uint32_t ctr_id,
                        uint64_t new_value,
                        char *sig,
                        unsigned *sig_len);

    int threshsign_sign_using_enclave(threshsign_t *ts,
                                      const char *hash,
                                      unsigned hash_len,
                                      uint32_t ctr_id,
                                      uint64_t new_value,
                                      char *sig,
                                      unsigned *sig_len);

    int threshsign_aggregate_and_verify(threshsign_t *ts,
                                        const char *hash,
                                        unsigned hash_len,
                                        uint32_t ctr_id,
                                        uint64_t new_value,
                                        unsigned sigshare_count,
                                        const char *sigshares,
                                        const unsigned *sigshare_lens,
                                        const int *sigshare_idx,
                                        char *sig,
                                        unsigned *sig_len);

    int threshsign_verify(threshsign_t *ts,
                          const char *hash,
                          unsigned hash_len,
                          uint32_t ctr_id,
                          uint64_t new_value,
                          const char *sig,
                          unsigned sig_len);

#if defined(__cplusplus)
}
#endif

#endif /* !_APP_H_ */
