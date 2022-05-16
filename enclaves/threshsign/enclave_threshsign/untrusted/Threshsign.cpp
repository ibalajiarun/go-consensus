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

#include <stdio.h>
#include <string.h>
#include <assert.h>

#include <unistd.h>
#include <pwd.h>
#define MAX_PATH FILENAME_MAX

#include <memory>
#include <sstream>
#include <ios>

#include "sgx_urts.h"
#include "Threshsign.h"
#include "Enclave_u.h"

#include <bls/BLSPrivateKey.h>
#include <bls/BLSSigShareSet.h>
#include <bls/BLSPublicKeyShare.h>
#include <bls/BLSutils.h>
#include "BLSPrivateKeyShareSGX.h"

#include "ThreshsignInternal.h"

#include <libff/common/profiling.hpp>

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

/* OCall functions */
void ocall_print_string(const char *str)
{
    /* Proxy/Bridge will check the length and null-terminate 
     * the input string to prevent buffer overflow. 
     */
    printf("%s", str);
    fflush(stdout);
}

int char2int(char _input)
{
    if (_input >= '0' && _input <= '9')
        return _input - '0';
    if (_input >= 'A' && _input <= 'F')
        return _input - 'A' + 10;
    if (_input >= 'a' && _input <= 'f')
        return _input - 'a' + 10;
    return -1;
}

bool hex2carray(const char *_hex, uint64_t *_bin_len,
                uint8_t *_bin)
{

    int len = strnlen(_hex, 2 * BUF_LEN);

    if (len == 0 && len % 2 == 1)
        return false;

    *_bin_len = len / 2;

    for (int i = 0; i < len / 2; i++)
    {
        int high = char2int((char)_hex[i * 2]);
        int low = char2int((char)_hex[i * 2 + 1]);

        if (high < 0 || low < 0)
        {
            return false;
        }

        _bin[i] = (unsigned char)(high * 16 + low);
    }

    return true;
}

int inited = 0;
void init()
{
    if (inited == 1)
        return;
    inited = 1;
    libff::init_alt_bn128_params();
    libff::inhibit_profiling_counters = true;
    libff::inhibit_profiling_info = true;
}

std::shared_ptr<std::vector<std::vector<uint64_t>>> blsPublicKeyToBytes(std::shared_ptr<BLSPublicKey> pubKey)
{
    std::vector<std::vector<uint64_t>> pkey_str_vect;

    auto libffPublicKey = pubKey->getPublicKey();
    libffPublicKey->to_affine_coordinates();

    pkey_str_vect.push_back(libffPublicKey->X.c0.to_words());
    pkey_str_vect.push_back(libffPublicKey->X.c1.to_words());
    pkey_str_vect.push_back(libffPublicKey->Y.c0.to_words());
    pkey_str_vect.push_back(libffPublicKey->Y.c1.to_words());
    pkey_str_vect.push_back(libffPublicKey->Z.c0.to_words());
    pkey_str_vect.push_back(libffPublicKey->Z.c1.to_words());

    return std::make_shared<std::vector<std::vector<uint64_t>>>(pkey_str_vect);
}

template <class T>
std::string convertToHexString(const T &field_elem)
{
    mpz_t t;
    mpz_init(t);

    field_elem.as_bigint().to_mpz(t);

    char arr[mpz_sizeinbase(t, 16) + 2];

    char *tmp = mpz_get_str(arr, 16, t);
    mpz_clear(t);

    std::string output = tmp;

    return output;
}

std::shared_ptr<std::vector<std::string>> blsPublicKeyToHexString(std::shared_ptr<BLSPublicKey> pubKey)
{
    std::vector<std::string> pkey_str_vect;

    auto libffPublicKey = pubKey->getPublicKey();
    libffPublicKey->to_affine_coordinates();

    pkey_str_vect.push_back(convertToHexString(libffPublicKey->X.c0));
    pkey_str_vect.push_back(convertToHexString(libffPublicKey->X.c1));
    pkey_str_vect.push_back(convertToHexString(libffPublicKey->Y.c0));
    pkey_str_vect.push_back(convertToHexString(libffPublicKey->Y.c1));
    pkey_str_vect.push_back(convertToHexString(libffPublicKey->Z.c0));
    pkey_str_vect.push_back(convertToHexString(libffPublicKey->Z.c1));

    return std::make_shared<std::vector<std::string>>(pkey_str_vect);
}

template <typename InputIt>
std::string join(InputIt begin,
                 InputIt end,
                 const std::string &separator = ", ", // see 1.
                 const std::string &concluder = "")   // see 1.
{
    std::ostringstream ss;

    if (begin != end)
    {
        ss << *begin++; // see 3.
    }

    while (begin != end) // see 3.
    {
        ss << separator;
        ss << *begin++;
    }

    ss << concluder;
    return ss.str();
}

int threshsign_setup(const char *enclave_file,
                     unsigned enclave_file_len,
                     uint32_t _threshold,
                     uint32_t _total,
                     uint32_t _node_index,
                     const char *skey,
                     unsigned skey_len,
                     const char **pkey,
                     unsigned pkey_count,
                     char *cpkey,
                     unsigned *cpkey_len,
                     uint32_t num_counters,
                     threshsign_t **ts)
{
    threshsign_t *_ts = new threshsign_t{};

    sgx_status_t ret = SGX_ERROR_UNEXPECTED;
    printf("opening enclave file %s\n", enclave_file);
    fflush(stdout);
    ret = sgx_create_enclave(enclave_file, SGX_DEBUG_FLAG, NULL, NULL, &_ts->eid, NULL);
    if (ret != SGX_SUCCESS)
    {
        print_error_message(ret);
        fflush(stdout);
        return 1;
    }

    char errMsg[BUF_LEN];
    int errStatus = 0;
    ret = ecall_bls_init(_ts->eid, &errStatus, errMsg, skey, skey_len, num_counters);
    if (ret != SGX_SUCCESS)
    {
        print_error_message(ret);
        fflush(stdout);
        return 2;
    }
    if (errStatus != 0)
    {
        printf("Error: Code: %d; Message: %s\n", errStatus, errMsg);
        fflush(stdout);
        return 2;
    }

    _ts->threshold = _threshold;
    _ts->total = _total;
    _ts->node_index = _node_index;
    std::string skey_str(skey, skey_len);

    init();
    _ts->sshare = std::make_shared<BLSPrivateKeyShare>(skey_str, _threshold, _total);
    _ts->sgx_sshare = std::make_shared<BLSPrivateKeyShareSGX>(_ts->eid, _threshold, _total);

    std::vector<std::string> vec_pkeys;
    for (int i = 0; i < pkey_count; i++)
    {
        vec_pkeys.emplace_back(pkey[i], strlen(pkey[i]));
        // std::cout << vec_pkeys.at(i) << "\n";
    }
    _ts->common_pkey = std::make_shared<BLSPublicKey>(
        std::make_shared<std::vector<std::string>>(vec_pkeys),
        _threshold, _total);

    std::shared_ptr<std::vector<std::string>> pkeyStrVec = _ts->common_pkey->toString();
    std::string str = join(pkeyStrVec->begin(), pkeyStrVec->end(), ":");
    // printf("%s\n", pkeyStrVec->at(i).c_str(), pkeyStrVec->at(i).size());
    *cpkey_len = str.size();
    snprintf(cpkey, *cpkey_len + 1, "%s", str.c_str());
    // fflush(stdout);

    // std::shared_ptr<std::vector<std::vector<uint64_t>>> pkeyStrVec2 = blsPublicKeyToBytes(_ts->common_pkey);
    // for (int i=0; i < pkeyStrVec2->size(); i++) {
    //     for(auto& val : pkeyStrVec2->at(i)) {
    //         printf("%llx ", val);
    //     }
    //     printf("(%d)\n", pkeyStrVec2->at(i).size());
    // }
    // fflush(stdout);

    // std::shared_ptr<std::vector<std::string>> pkeyStrVec1 = blsPublicKeyToHexString(_ts->common_pkey);
    // for (int i=0; i < pkeyStrVec1->size(); i++) {
    //     printf("%s(%d)\n", pkeyStrVec1->at(i).c_str(), pkeyStrVec1->at(i).size());
    // }
    // fflush(stdout);

    *ts = _ts;

    return 0;
}

int threshsign_sign(threshsign_t *ts,
                    const char *hash,
                    unsigned hash_len,
                    uint32_t ctr_id,
                    uint64_t new_value,
                    char *sig,
                    unsigned *sig_len)
{
    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < hash_len; i++)
    {
        hash_byte_arr.at(i) = hash[i];
    }
    for (size_t i = 0; i < 4; i++)
    {
        hash_byte_arr.at(31 - i) = (uint8_t)(hash_byte_arr.at(31 - i) ^ ((uint8_t)((new_value >> 8 * i) & 0xFF)));
    }
    for (size_t i = 0; i < 8; i++)
    {
        hash_byte_arr.at(27 - i) = (uint8_t)(hash_byte_arr.at(27 - i) ^ ((uint8_t)((new_value >> 8 * i) & 0xFF)));
    }

    std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
        std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

    std::shared_ptr<BLSSigShare> sigShare = ts->sshare->sign(hash_ptr, ts->node_index);

    auto sigSharestr = sigShare->toString();
    // printf("sig %s\n", sigShare->toString()->c_str());
    // printf("sig_len %d\n", sigShare->toString()->size());
    // fflush(stdout);
    *sig_len = sigSharestr->size();
    snprintf(sig, *sig_len + 1, "%s", sigSharestr->c_str());
    return 0;
}

int threshsign_sign_using_enclave(threshsign_t *ts,
                                       const char *hash,
                                       unsigned hash_len,
                                       uint32_t ctr_id,
                                       uint64_t new_value,
                                       char *sig,
                                       unsigned *sig_len)
{
    // std::array<uint8_t, 32> hash_byte_arr;
    // for (size_t i = 0; i < hash_len; i++)
    // {
    //     hash_byte_arr.at(i) = hash[i];
    // }
    // std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
    //     std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

    sgx_status_t sgx_ret;
    int call_ret;
    char errMsg[BUF_LEN];
    char signature[BUF_LEN];
    memset(errMsg, 0, BUF_LEN);
    memset(signature, 0, BUF_LEN);

    sgx_ret = ecall_counter_sign(ts->eid, &call_ret, ctr_id, new_value, hash, signature);

    if (sgx_ret != SGX_SUCCESS)
    {
        return sgx_ret;
    }

    *sig_len = snprintf(sig, *sig_len + 1, "%s", signature);

    return call_ret;
}

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
                                    unsigned *sig_len)
{
    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < hash_len; i++)
    {
        hash_byte_arr.at(i) = hash[i];
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

    BLSSigShareSet sigSet(ts->threshold, ts->total);

    size_t idx = 0;
    for (size_t i = 0; i < sigshare_count; i++)
    {
        sigSet.addSigShare(std::make_shared<BLSSigShare>(
            std::make_shared<std::string>(&sigshares[idx], sigshare_lens[i]),
            sigshare_idx[i],
            ts->threshold, ts->total));
        idx += sigshare_lens[i];
    }

    std::shared_ptr<BLSSignature> common_sig_ptr = sigSet.merge(); //create common signature

    if (!ts->common_pkey->VerifySigWithHelper(hash_ptr, common_sig_ptr, ts->threshold, ts->total))
    {
        return 1;
    }

    auto common_sig_str = common_sig_ptr->toString();
    *sig_len = common_sig_str->size();
    snprintf(sig, *sig_len + 1, "%s", common_sig_str->c_str());
    return 0;
}

int threshsign_verify(threshsign_t *ts,
                      const char *hash,
                      unsigned hash_len,
                      uint32_t ctr_id,
                      uint64_t new_value,
                      const char *sig,
                      unsigned sig_len)
{
    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < hash_len; i++)
    {
        hash_byte_arr.at(i) = hash[i];
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

    auto common_sig_ptr = std::make_shared<BLSSignature>(
        std::make_shared<std::string>(sig, sig_len),
        ts->threshold, ts->total);

    if (!ts->common_pkey->VerifySigWithHelper(hash_ptr, common_sig_ptr, ts->threshold, ts->total))
    {
        return 1;
    }
    return 0;
}