#ifndef _TRINX_H_
#define _TRINX_H_

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>

#include "sgx_error.h" /* sgx_status_t */
#include "sgx_eid.h"   /* sgx_enclave_id_t */

#ifndef TRUE
#define TRUE 1
#endif

#ifndef FALSE
#define FALSE 0
#endif

#if defined(__cplusplus)
extern "C"
{
#endif

    int trinx_init(
        const char *enclave_file_path,
        const char *skey, unsigned skey_len,
        uint32_t num_counters,
        uint32_t vec_len,
        uint64_t* enclave_id);

    int trinx_create_independent_counter_certificate(
        uint64_t eid,
        uint32_t ctr_id,
        uint64_t new_value,
        const uint8_t *hash, uint32_t hash_size,
        uint8_t *cert, uint32_t cert_size);

    int trinx_create_independent_counter_certificate_vector(
        uint64_t eid,
        uint32_t ctr_id,
        uint64_t new_value,
        const uint8_t *hash, uint32_t hash_size,
        uint8_t *cert, uint32_t cert_size);

    int trinx_create_continuing_counter_certificate(
        uint64_t eid,
        uint32_t ctr_id,
        uint64_t new_value,
        const uint8_t *hash, uint32_t hash_size,
        uint8_t *cert, uint32_t cert_size);

    int trinx_verify_independent_counter_certificate(
        uint64_t eid,
        uint32_t ctr_id,
        uint64_t new_value,
        const uint8_t *hash, uint32_t hash_size,
        const uint8_t *cert, uint32_t cert_size,
        uint8_t *identical);

    int trinx_verify_continuing_counter_certificate(
        uint64_t eid,
        uint32_t ctr_id,
        uint64_t old_value,
        uint64_t new_value,
        const uint8_t *hash, uint32_t hash_size,
        const uint8_t *cert, uint32_t cert_size,
        uint8_t *identical);

    int trinx_destroy(uint64_t eid);

#if defined(__cplusplus)
}
#endif

#endif /* !_TRINX_H_ */
