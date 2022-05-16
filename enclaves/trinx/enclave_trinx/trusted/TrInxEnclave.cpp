#include <stdarg.h>
#include <stdio.h> /* vsnprintf */

#include "string.h"

#include "ipp/ippcp.h"

#include "TrInxEnclave.h"
#include "TrInxEnclave_t.h" /* print_string */

#include "../include/common.h"

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

uint64_t *counters;
uint32_t vec_len;
unsigned char MACKEY[BUF_LEN];
unsigned MACKEYLEN;

void ecall_trinx_init(const char *_key, uint32_t _key_len,
                      uint32_t _num_counters, uint32_t _vec_len)
{
    strncpy((char *)MACKEY, _key, _key_len);
    MACKEYLEN = _key_len;
    vec_len = _vec_len;
    counters = (uint64_t *)calloc(_num_counters, sizeof(uint64_t));
}

int generate_hmac(
    const uint8_t *ctr_id,
    const uint8_t *old_value_bytes,
    const uint8_t *new_value_bytes,
    const uint8_t *msg, uint32_t msg_size,
    uint8_t *cert, uint32_t cert_size)
{
    IppsHMACState *ctx;
    IppStatus status;
    int psize = 0;

    status = ippsHMAC_GetSize(&psize);
    if (status == ippStsNullPtrErr)
        return 1;

    ctx = (IppsHMACState *)malloc(psize);
    status = ippsHMAC_Init(MACKEY, MACKEYLEN, ctx, ippHashAlg_SHA256);
    if (status != ippStsNoErr)
        return 2;

    status = ippsHMAC_Update(ctr_id, 4, ctx);
    if (status != ippStsNoErr)
        return 3;

    if (old_value_bytes != NULL)
    {
        status = ippsHMAC_Update(old_value_bytes, 8, ctx);
        if (status != ippStsNoErr)
            return 4;
    }

    status = ippsHMAC_Update(new_value_bytes, 8, ctx);
    if (status != ippStsNoErr)
        return 5;

    status = ippsHMAC_Update(msg, msg_size, ctx);
    if (status != ippStsNoErr)
        return 6;

    uint8_t hmac[HMAC_LENGTH];
    memset(hmac, '\0', HMAC_LENGTH);
    status = ippsHMAC_Final(hmac, HMAC_LENGTH, ctx);

    if (status != ippStsNoErr)
        return 7;

    memcpy(cert, hmac, cert_size);

    free(ctx);

    return 0;
}

uint32_t ecall_trinx_create_independent_counter_certificate(
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    uint8_t *cert, uint32_t cert_size)
{
    if (new_value <= counters[ctr_id])
    {
        memcpy(cert, &counters[ctr_id], sizeof(uint64_t));
        return INVALID_COUNTER_VALUE;
    }

    uint8_t new_ctr_bytes[8], ctr_id_bytes[4];
    memcpy(ctr_id_bytes, &ctr_id, sizeof(uint32_t));
    memcpy(new_ctr_bytes, &new_value, sizeof(uint64_t));

    counters[ctr_id] = new_value;
    return generate_hmac(ctr_id_bytes, NULL, new_ctr_bytes, msg, msg_size, cert, cert_size);
}

uint32_t ecall_trinx_create_independent_counter_certificate_vector(
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    uint8_t *cert, uint32_t cert_size)
{
    if (new_value <= counters[ctr_id])
    {
        memcpy(cert, &counters[ctr_id], sizeof(uint64_t));
        return INVALID_COUNTER_VALUE;
    }

    uint8_t new_ctr_bytes[8], ctr_id_bytes[4];
    memcpy(ctr_id_bytes, &ctr_id, sizeof(uint32_t));
    memcpy(new_ctr_bytes, &new_value, sizeof(uint64_t));

    counters[ctr_id] = new_value;

    for (uint32_t idx = 0; idx < vec_len; ++idx)
    {
        int32_t ret = generate_hmac(ctr_id_bytes, NULL, new_ctr_bytes,
                                    msg, msg_size, cert + (32 * idx), 32);
        if (ret != SGX_SUCCESS)
        {
            return ret;
        }
    }
    return SGX_SUCCESS;
}

uint32_t ecall_trinx_create_continuing_counter_certificate(
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    uint8_t *cert, uint32_t cert_size)
{
    if (new_value < counters[ctr_id])
    {
        return INVALID_COUNTER_VALUE;
    }

    uint8_t old_ctr_bytes[8], new_ctr_bytes[8], ctr_id_bytes[4];
    memcpy(ctr_id_bytes, &ctr_id, sizeof(uint32_t));
    memcpy(old_ctr_bytes, &counters[ctr_id], sizeof(uint64_t));
    memcpy(new_ctr_bytes, &new_value, sizeof(uint64_t));

    counters[ctr_id] = new_value;
    return generate_hmac(ctr_id_bytes, old_ctr_bytes, new_ctr_bytes, msg, msg_size, cert, cert_size);
}

uint32_t ecall_trinx_verify_independent_counter_certificate(
    uint32_t ctr_id,
    uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    const uint8_t *cert, uint32_t cert_size,
    uint8_t *identical)
{
    int status = 0;
    uint8_t hmac[HMAC_LENGTH];
    uint8_t new_ctr_bytes[8], ctr_id_bytes[4];

    memset(hmac, '\0', HMAC_LENGTH);
    memcpy(new_ctr_bytes, &new_value, sizeof(new_value));
    memcpy(ctr_id_bytes, &ctr_id, sizeof(uint32_t));

    status = generate_hmac(ctr_id_bytes, NULL, new_ctr_bytes, msg, msg_size, hmac, cert_size);
    if (status == 0)
    {
        if (memcmp(cert, hmac, cert_size) == 0)
        {
            *identical = 1;
        }
        else
        {
            *identical = 0;
        }
    }

    return status;
}

uint32_t ecall_trinx_verify_continuing_counter_certificate(
    uint32_t ctr_id,
    uint64_t old_value, uint64_t new_value,
    const uint8_t *msg, uint32_t msg_size,
    const uint8_t *cert, uint32_t cert_size,
    uint8_t *identical)
{
    int status = 0;
    uint8_t hmac[HMAC_LENGTH];
    uint8_t new_ctr_bytes[8], old_ctr_bytes[8], ctr_id_bytes[8];

    memset(hmac, '\0', HMAC_LENGTH);
    memcpy(ctr_id_bytes, &ctr_id, sizeof(uint32_t));
    memcpy(old_ctr_bytes, &old_value, sizeof(old_value));
    memcpy(new_ctr_bytes, &new_value, sizeof(new_value));

    status = generate_hmac(ctr_id_bytes, old_ctr_bytes, new_ctr_bytes, msg, msg_size, hmac, cert_size);
    if (status == 0)
    {
        if (memcmp(cert, hmac, cert_size) == 0)
        {
            *identical = 1;
        }
        else
        {
            *identical = 0;
        }
    }

    return status;
}
