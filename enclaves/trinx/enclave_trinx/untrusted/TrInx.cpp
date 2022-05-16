#include <stdio.h>
#include <string.h>
#include <assert.h>

#include <unistd.h>
#include <pwd.h>
#define MAX_PATH FILENAME_MAX

#include <memory>

#include "sgx_urts.h"

#include "TrInx.h"
#include "TrInxEnclave_u.h"

typedef struct _sgx_errlist_t
{
    sgx_status_t err;
    const char *msg;
    const char *sug; /* Suggestion */
} sgx_errlist_t;

/* Error code returned by sgx_create_enclave */
static sgx_errlist_t sgx_errlist[] = {
    {SGX_ERROR_UNEXPECTED,
     "Unexpected error occurred.",
     NULL},
    {SGX_ERROR_INVALID_PARAMETER,
     "Invalid parameter.",
     NULL},
    {SGX_ERROR_OUT_OF_MEMORY,
     "Out of memory.",
     NULL},
    {SGX_ERROR_ENCLAVE_LOST,
     "Power transition occurred.",
     "Please refer to the sample \"PowerTransition\" for details."},
    {SGX_ERROR_INVALID_ENCLAVE,
     "Invalid enclave image.",
     NULL},
    {SGX_ERROR_INVALID_ENCLAVE_ID,
     "Invalid enclave identification.",
     NULL},
    {SGX_ERROR_INVALID_SIGNATURE,
     "Invalid enclave signature.",
     NULL},
    {SGX_ERROR_OUT_OF_EPC,
     "Out of EPC memory.",
     NULL},
    {SGX_ERROR_NO_DEVICE,
     "Invalid SGX device.",
     "Please make sure SGX module is enabled in the BIOS, and install SGX driver afterwards."},
    {SGX_ERROR_MEMORY_MAP_CONFLICT,
     "Memory map conflicted.",
     NULL},
    {SGX_ERROR_INVALID_METADATA,
     "Invalid enclave metadata.",
     NULL},
    {SGX_ERROR_DEVICE_BUSY,
     "SGX device was busy.",
     NULL},
    {SGX_ERROR_INVALID_VERSION,
     "Enclave version was invalid.",
     NULL},
    {SGX_ERROR_INVALID_ATTRIBUTE,
     "Enclave was not authorized.",
     NULL},
    {SGX_ERROR_ENCLAVE_FILE_ACCESS,
     "Can't open enclave file.",
     NULL},
};

/* Check error conditions for loading enclave */
void print_error_message(sgx_status_t ret)
{
    size_t idx = 0;
    size_t ttl = sizeof sgx_errlist / sizeof sgx_errlist[0];

    for (idx = 0; idx < ttl; idx++)
    {
        if (ret == sgx_errlist[idx].err)
        {
            if (NULL != sgx_errlist[idx].sug)
                printf("Info: %s\n", sgx_errlist[idx].sug);
            printf("Error: %s\n", sgx_errlist[idx].msg);
            break;
        }
    }

    if (idx == ttl)
        printf("Error code is 0x%X. Please refer to the \"Intel SGX SDK Developer Reference\" for more details.\n", ret);
}

/* Initialize the enclave:
 *   Call sgx_create_enclave to initialize an enclave instance
 */
int trinx_init(
    const char *enclave_file_path,
    const char *skey, unsigned skey_len,
    uint32_t num_counters,
    uint32_t vec_len,
    uint64_t *enclave_id)
{
    sgx_enclave_id_t eid;
    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    
    printf("opening enclave file %s\n", enclave_file_path);
    fflush(stdout);
    ret = sgx_create_enclave(enclave_file_path, SGX_DEBUG_FLAG, NULL, NULL, &eid, NULL);
    if (ret != SGX_SUCCESS)
    {
        print_error_message(ret);
        fflush(stdout);
        return ret;
    }

    ret = ecall_trinx_init(eid, skey, skey_len, num_counters, vec_len);
    if (ret != SGX_SUCCESS)
    {
        print_error_message(ret);
        return ret;
    }

    *enclave_id = eid;

    return 0;
}

int trinx_create_independent_counter_certificate(
    uint64_t eid,
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    uint8_t *cert, uint32_t cert_size)
{
    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    uint32_t ecall_return = 0;

    ret = ecall_trinx_create_independent_counter_certificate(
        eid, &ecall_return,
        ctr_id, 
        new_value,
        msg, msg_size, cert, cert_size);
    if (ret != SGX_SUCCESS)
    {
        return ret;
    }

    return ecall_return;
}

int trinx_create_independent_counter_certificate_vector(
    uint64_t eid,
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    uint8_t *cert, uint32_t cert_size)
{
    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    uint32_t ecall_return = 0;

    ret = ecall_trinx_create_independent_counter_certificate_vector(
        eid, &ecall_return,
        ctr_id,
        new_value,
        msg, msg_size, cert, cert_size);
    if (ret != SGX_SUCCESS)
    {
        return ret;
    }

    return ecall_return;
}

int trinx_create_continuing_counter_certificate(
    uint64_t eid,
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    uint8_t *cert, uint32_t cert_size)
{
    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    uint32_t ecall_return = 0;

    ret = ecall_trinx_create_continuing_counter_certificate(
        eid, &ecall_return,
        ctr_id, 
        new_value,
        msg, msg_size,
        cert, cert_size);
    if (ret != SGX_SUCCESS)
    {
        return ret;
    }

    return ecall_return;
}

int trinx_verify_independent_counter_certificate(
    uint64_t eid,
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    const uint8_t *cert, uint32_t cert_size,
    uint8_t *identical)
{
    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    uint32_t ecall_return = 0;

    ret = ecall_trinx_verify_independent_counter_certificate(
        eid, &ecall_return,
        ctr_id,
        new_value,
        msg, msg_size,
        cert, cert_size, identical);
    if (ret != SGX_SUCCESS)
    {
        return ret;
    }

    return ecall_return;
}

int trinx_verify_continuing_counter_certificate(
    uint64_t eid,
    uint32_t ctr_id,
    uint64_t old_value,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    const uint8_t *cert, uint32_t cert_size,
    uint8_t *identical)
{
    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    uint32_t ecall_return = 0;

    ret = ecall_trinx_verify_continuing_counter_certificate(
        eid, &ecall_return,
        ctr_id,
        old_value, new_value,
        msg, msg_size,
        cert, cert_size, identical);
    if (ret != SGX_SUCCESS)
    {
        return ret;
    }

    return ecall_return;
}

int trinx_destroy(uint64_t eid)
{
    sgx_destroy_enclave(eid);
    return 0;
}

/* OCall functions */
void ocall_print_string(const char *str)
{
    /* Proxy/Bridge will check the length and null-terminate 
     * the input string to prevent buffer overflow. 
     */
    printf("%s", str);
}
