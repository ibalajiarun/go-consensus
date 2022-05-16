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

#include "Enclave.h"
#include "Enclave_t.h" /* print_string */
#include <stdarg.h>
#include <stdio.h> /* vsnprintf */
#include <string.h>
#include <array>
#include <memory>
#include <utility>

#include <trusted_libff/libff/algebra/curves/alt_bn128/alt_bn128_init.hpp>
#include <trusted_libff/libff/algebra/curves/alt_bn128/alt_bn128_pp.hpp>

#define BUF_LEN 1024
#define INVALID_COUNTER_VALUE 0xF004

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

int inited = 0;

void init()
{
    if (inited == 1)
        return;
    inited = 1;
    libff::init_alt_bn128_params();
    // libff::inhibit_profiling_info = false;
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

libff::alt_bn128_Fq HashToFq(
    std::shared_ptr<std::array<uint8_t, 32>> hash_byte_arr)
{
    libff::bigint<libff::alt_bn128_q_limbs> from_hex;

    std::vector<uint8_t> hex(64);
    for (size_t i = 0; i < 32; ++i)
    {
        hex[2 * i] = static_cast<int>(hash_byte_arr->at(i)) / 16;
        hex[2 * i + 1] = static_cast<int>(hash_byte_arr->at(i)) % 16;
    }
    mpn_set_str(from_hex.data, hex.data(), 64, 16);

    libff::alt_bn128_Fq ret_val(from_hex);

    return ret_val;
}

std::pair<libff::alt_bn128_G1, std::string*> HashtoG1WithHint(
    std::shared_ptr<std::array<uint8_t, 32>> hash_byte_arr)
{
    libff::alt_bn128_G1 point;
    libff::alt_bn128_Fq counter = libff::alt_bn128_Fq::zero();
    libff::alt_bn128_Fq x1(HashToFq(hash_byte_arr));

    while (true)
    {
        libff::alt_bn128_Fq y1_sqr = x1 ^ 3;
        y1_sqr = y1_sqr + libff::alt_bn128_coeff_b;

        libff::alt_bn128_Fq euler = y1_sqr ^ libff::alt_bn128_Fq::euler;

        if (euler == libff::alt_bn128_Fq::one() ||
            euler == libff::alt_bn128_Fq::zero())
        { // if y1_sqr is a square
            point.X = x1;
            libff::alt_bn128_Fq temp_y = y1_sqr.sqrt();

            mpz_t pos_y;
            mpz_init(pos_y);

            temp_y.as_bigint().to_mpz(pos_y);

            mpz_t neg_y;
            mpz_init(neg_y);

            (-temp_y).as_bigint().to_mpz(neg_y);

            if (mpz_cmp(pos_y, neg_y) < 0)
            {
                temp_y = -temp_y;
            }

            mpz_clear(pos_y);
            mpz_clear(neg_y);

            point.Y = temp_y;
            break;
        }
        else
        {
            counter = counter + libff::alt_bn128_Fq::one();
            x1 = x1 + libff::alt_bn128_Fq::one();
        }
    }
    point.Z = libff::alt_bn128_Fq::one();

    return std::make_pair(point, stringFromFq(&counter));
}

libff::alt_bn128_Fr *keyFromString(const char *_keyStringHex)
{
    mpz_t skey;
    mpz_init(skey);
    mpz_set_str(skey, _keyStringHex, 16);

    char skey_dec[mpz_sizeinbase(skey, 10) + 2];
    char *skey_str = mpz_get_str(skey_dec, 10, skey);
    printf("keyFromString1 %s\n", skey_str);

    return new libff::alt_bn128_Fr(skey_dec);
}

bool bls_sign(const char *_keyString,
              const char *_hashXString, const char *_hashYString,
              char *sig)
{
    auto key = std::make_shared<libff::alt_bn128_Fr>(_keyString);

    libff::alt_bn128_Fq hashX(_hashXString);
    libff::alt_bn128_Fq hashY(_hashYString);
    libff::alt_bn128_Fq hashZ = 1;

    libff::alt_bn128_G1 hash(hashX, hashY, hashZ);

    libff::alt_bn128_G1 sign = key->as_bigint() * hash; // sign

    sign.to_affine_coordinates();

    auto r = stringFromG1(&sign);

    memset(sig, 0, BUF_LEN);

    strncpy(sig, r->c_str(), BUF_LEN);

    delete r;

    return true;
}

uint64_t *counters;
char BLSKEY[BUF_LEN];

void ecall_bls_init(int *err_status, char *err_string,
                    const char *key, uint32_t key_len,
                    uint32_t num_counters)
{
    strncpy(BLSKEY, key, key_len);
    counters = (uint64_t *)calloc(num_counters, sizeof(uint64_t));
    init();
}

void ecall_bls_sign(int *err_status, char *err_string,
                    char *_hashX, char *_hashY, char *signature)
{
    char *sig = (char *)calloc(BUF_LEN, 1);

    try
    {
        bls_sign(BLSKEY, _hashX, _hashY, sig);
    }
    catch (const std::exception &e)
    {
        printf("Very very bad happened %s\n", e.what());
        *err_status = -2;
        return;
    }

    strncpy(signature, sig, BUF_LEN);

    if (strnlen(signature, BUF_LEN) < 10)
    {
        *err_status = -1;
        return;
    }

    free(sig);
}

int ecall_counter_sign(uint32_t ctr_id, uint64_t new_value,
                                const char *_hash, char *signature)
{
    if (new_value <= counters[ctr_id])
    {
        snprintf(signature, BUF_LEN, "ctd_id %d: %d < %d", ctr_id, new_value, counters[ctr_id]);
        return INVALID_COUNTER_VALUE;
    }
    counters[ctr_id] = new_value;

    auto key = std::make_shared<libff::alt_bn128_Fr>(BLSKEY);

    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < 32; i++)
    {
        hash_byte_arr.at(i) = _hash[i];
    }
    for (size_t i = 0; i < 4; i++)
    {
        hash_byte_arr.at(31 - i) = (uint8_t)(hash_byte_arr.at(31 - i) ^ ((uint8_t)((ctr_id >> 8 * i) & 0xFF)));
    }
    for (size_t i = 0; i < 8; i++)
    {
        hash_byte_arr.at(27 - i) = (uint8_t)(hash_byte_arr.at(27 - i) ^ ((uint8_t)((new_value >> 8 * i) & 0xFF)));
    }

    std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
        std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

    std::pair<libff::alt_bn128_G1, std::string*> hash_with_hint =
        HashtoG1WithHint(hash_ptr);

    libff::alt_bn128_G1 sign = key->as_bigint() * hash_with_hint.first; // sign

    sign.to_affine_coordinates();

    auto r = stringFromG1(&sign);

    auto yStr = stringFromFq(&hash_with_hint.first.Y);
    std::string hint = *yStr + ":" + *hash_with_hint.second;

    r->append(":");
    r->append(hint);

    memset(signature, 0, BUF_LEN);
    strncpy(signature, r->c_str(), BUF_LEN);

    delete r;
    delete yStr;
    delete hash_with_hint.second;

    return SGX_SUCCESS;
}

void ecall_bls_batch_sign(int *err_status, char *err_string, int count,
                          char *_hashXs, char *_hashYs, char *signatures)
{
    // printf("Hello from SGX!\n");

    char *sig = (char *)calloc(BUF_LEN, 1);
    // printf("Calling bls_sign!\n");

    for (int i = 0; i < count; i++)
    {
        try
        {
            bls_sign(BLSKEY, &_hashXs[BUF_LEN * i], &_hashYs[BUF_LEN * i], sig);
        }
        catch (const std::exception &e)
        {
            printf("Very very bad happened %s\n", e.what());
        }

        strncpy(&signatures[BUF_LEN * i], sig, BUF_LEN);
        memset(sig, 0, BUF_LEN);

        if (strnlen(&signatures[BUF_LEN * i], BUF_LEN) < 10)
        {
            *err_status = -1;
            return;
        }
    }
    free(sig);
}