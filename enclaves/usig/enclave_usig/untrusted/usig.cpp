#include <stdio.h>
#include <string.h>
#include <assert.h>
#include <signal.h>

#include <unistd.h>
#include <pwd.h>
#define MAX_PATH FILENAME_MAX

#include <memory>

#include "sgx_urts.h"

#include "usig.h"
#include "usig_enclave_u.h"

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

// Wraps an ECall invocation assuming the convention of 'ecall_usig_'
// prefix of the function name and sgx_status_t its return value type
#define ECALL_USIG(enclave_id, fn, ...)                         \
        ({                                                      \
                sgx_status_t sgx_ret, call_ret;                      \
                sgx_ret = ecall_usig_##fn(enclave_id, &call_ret,     \
                                          ##__VA_ARGS__);       \
                sgx_ret != SGX_SUCCESS ? sgx_ret : call_ret;         \
        })

void savesigsegv() {
    struct sigaction action;
    struct sigaction old_action;
    sigaction(SIGSEGV, NULL, &action);
    action.sa_flags = action.sa_flags | SA_ONSTACK;
    sigaction(SIGSEGV, &action, &old_action);
}


/* Initialize the enclave:
 *   Call sgx_create_enclave to initialize an enclave instance
 */
sgx_status_t usig_init(
    const char *enclave_file_path,
    const char *skey, unsigned skey_len,
    sgx_ec256_private_t _privkey,
    uint32_t num_counters,
    const char *skey_edd25519, unsigned skey_edd25519_len,
    uint64_t *enclave_id)
{
    sgx_enclave_id_t eid;
    sgx_status_t ret;
    
    printf("opening enclave file %s\n", enclave_file_path);
    fflush(stdout);
    ret = sgx_create_enclave(enclave_file_path, SGX_DEBUG_FLAG, NULL, NULL, &eid, NULL);
    if (ret != SGX_SUCCESS)
    {
        goto err_out;
    }

    ret = ECALL_USIG(eid, init, skey, skey_len, _privkey, num_counters, skey_edd25519, skey_edd25519_len);
    if (ret != SGX_SUCCESS)
    {
        goto err_enclave_created;
    }

    *enclave_id = eid;

    savesigsegv();

    return SGX_SUCCESS;

err_enclave_created:
    sgx_destroy_enclave(*enclave_id);
err_out:
    return ret;
}

sgx_status_t usig_create_ui_sig_edd25519(
    uint64_t eid,
    sgx_sha256_hash_t digest,
    uint32_t counter_id,
    uint64_t *counter,
    uint8_t *signature,
    uint32_t signature_len)
{
    return ECALL_USIG(eid, create_ui_sig_edd25519, digest, counter_id, counter, signature, signature_len);
}

sgx_status_t usig_create_ui_sig(
    uint64_t eid,
    sgx_sha256_hash_t digest,
    uint64_t *counter,
    sgx_ec256_signature_t *signature)
{
    return ECALL_USIG(eid, create_ui_sig, digest, counter, signature);
}

sgx_status_t usig_create_ui_mac(
    uint64_t eid,
    sgx_sha256_hash_t digest,
    uint64_t *counter,
    uint8_t *cert, uint32_t cert_size)
{
    return ECALL_USIG(eid, create_ui_mac, digest, counter, cert, cert_size);
}

sgx_status_t usig_verify_ui_mac(
    uint64_t eid,
    sgx_sha256_hash_t digest,
    uint64_t counter,
    const uint8_t *cert, uint32_t cert_size,
    uint8_t *identical)
{
    return ECALL_USIG(eid, verify_ui_mac, digest, counter, cert, cert_size, identical);
}

sgx_status_t usig_destroy(uint64_t eid)
{
    return sgx_destroy_enclave(eid);
}

/* OCall functions */
void ocall_print_string(const char *str)
{
    /* Proxy/Bridge will check the length and null-terminate 
     * the input string to prevent buffer overflow. 
     */
    printf("%s", str);
    fflush(stdout);
}
