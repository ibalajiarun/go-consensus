#ifndef _UNTRUSTED_H_
#define _UNTRUSTED_H_

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <stdbool.h> 

#include "sgx_error.h" /* sgx_status_t */
#include "sgx_eid.h"   /* sgx_enclave_id_t */

#if defined(__cplusplus)
extern "C"
{
#endif

    struct threshsign;
    typedef struct threshsign threshsign_t;

    sgx_status_t threshsign2_init(
        const char *_enclave_file_path,
        uint32_t _threshold,
        uint32_t _total,
        uint32_t _node_index,
        const char *_secret_key,
        const char *_public_key,
        _Bool _fast_lagrange,
        threshsign_t **ts);

    sgx_status_t threshsign2_sign(
        threshsign_t *ts,
        const char *hash,
        uint64_t counter,
        char *sig,
        unsigned *sig_len);

    sgx_status_t threshsign2_sign_using_enclave(
        threshsign_t *ts,
        const char *hash,
        uint64_t counter,
        char *sig,
        unsigned *sig_len);

    sgx_status_t threshsign2_aggregate_and_verify(
        threshsign_t *ts,
        const char *hash,
        uint64_t counter,
        unsigned sigshare_count,
        const char *sigshares,
        const unsigned *sigshare_lens,
        const int *sigshare_idx,
        char *sig,
        unsigned *sig_len);

    sgx_status_t threshsign2_verify(
        threshsign_t *ts,
        const char *hash,
        uint64_t counter,
        const char *sig,
        unsigned sig_len);

    sgx_status_t threshsign2_destroy(threshsign_t *ts);

#if defined(__cplusplus)
}
#endif

#endif /* !_UNTRUSTED_H_ */
