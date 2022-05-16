#include <stdio.h>
#include <string.h>
#include <assert.h>

#include <unistd.h>
#include <pwd.h>

#include <memory>
#include <thread>

#include "sgx_urts.h"
#include "threshsign.h"
#include "helper.h"
#include "constants.h"
#include "threshsign_internal.h"
#include "threshsign_enclave_u.h"

#include <polycrypto/NtlLib.h>
#include <polycrypto/PolyCrypto.h>
#include <polycrypto/Lagrange.h>
#include <polycrypto/FFThresh.h>
#include <xutils/Utils.h>

using namespace libpolycrypto;

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
#define ECALL_TSS(enclave_id, fn, ...)                  \
    ({                                                  \
        sgx_status_t sgx_ret, call_ret;                 \
        sgx_ret = ecall_tss_##fn(enclave_id, &call_ret, \
                                 ##__VA_ARGS__);        \
        sgx_ret != SGX_SUCCESS ? sgx_ret : call_ret;    \
    })

/* OCall functions */
void ocall_print_string(const char *str)
{
    /* Proxy/Bridge will check the length and null-terminate 
     * the input string to prevent buffer overflow. 
     */
    printf("%s", str);
}

static int inited = 0;
static std::vector<Fr> omegas;
static bool _fast_lagrange;
void init(int n, bool fast_lagrange)
{
    if (inited == 1)
        return;
    inited = 1;

    _fast_lagrange = fast_lagrange;
    libff::inhibit_profiling_counters = true;
    libpolycrypto::initialize(nullptr, 0);
    size_t N = Utils::smallestPowerOfTwoAbove(n);
    omegas = get_all_roots_of_unity(N);
}

thread_local int ntl_inited = 0;
void ntl_init()
{
    if (ntl_inited == 1)
        return;
    ntl_inited = 1;

    // Initializes the NTL finite field
    NTL::ZZ p = NTL::conv<ZZ> ("21888242871839275222246405745257275088548364400416034343698204186575808495617");
    NTL::ZZ_p::init(p);
}

sgx_status_t threshsign2_init(
    const char *_enclave_file_path,
    uint32_t _threshold,
    uint32_t _total,
    uint32_t _node_index,
    const char *_secret_key,
    const char *_public_key,
    bool _fast_lagrange,
    threshsign_t **ts)
{
    threshsign_t *_ts = new threshsign_t{};
    sgx_enclave_id_t eid;
    sgx_status_t ret;

    printf("opening enclave file %s\n", _enclave_file_path);
    fflush(stdout);
    ret = sgx_create_enclave(_enclave_file_path, SGX_DEBUG_FLAG, NULL, NULL, &eid, NULL);
    if (ret != SGX_SUCCESS)
    {
        goto err_out;
    }

    ret = ECALL_TSS(eid, init, _secret_key);
    if (ret != SGX_SUCCESS)
    {
        goto err_enclave_created;
    }

    init(_total, _fast_lagrange);

    _ts->eid = eid;
    _ts->threshold = _threshold;
    _ts->total = _total;
    _ts->node_index = _node_index;
    _ts->secret_key = std::make_shared<libff::alt_bn128_Fr>(_secret_key);
    _ts->public_key = std::make_shared<libff::alt_bn128_G2>();

    {
        auto pubKeyParts = SplitString(std::make_shared<std::string>(_public_key), ":");
        _ts->public_key->X.c0 = libff::alt_bn128_Fq(pubKeyParts->at(0).c_str());
        _ts->public_key->X.c1 = libff::alt_bn128_Fq(pubKeyParts->at(1).c_str());
        _ts->public_key->Y.c0 = libff::alt_bn128_Fq(pubKeyParts->at(2).c_str());
        _ts->public_key->Y.c1 = libff::alt_bn128_Fq(pubKeyParts->at(3).c_str());
        _ts->public_key->Z.c0 = libff::alt_bn128_Fq::one();
        _ts->public_key->Z.c1 = libff::alt_bn128_Fq::zero();
    }

    *ts = _ts;

    return SGX_SUCCESS;

err_enclave_created:
    sgx_destroy_enclave(eid);
err_out:
    delete _ts;
    return ret;
}

sgx_status_t threshsign2_sign(
    threshsign_t *ts,
    const char *hash,
    uint64_t counter,
    char *sig,
    unsigned *sig_len)
{
    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < 32; i++)
    {
        hash_byte_arr.at(i) = hash[i];
    }
    if (counter > 0)
    {
        for (size_t i = 0; i < 4; i++)
        {
            hash_byte_arr.at(31 - i) = (uint8_t)(hash_byte_arr.at(31 - i) ^ ((uint8_t)((counter >> 8 * i) & 0xFF)));
        }
    }

    std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
        std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

    G1 hashG1 = HashtoG1(hash_ptr);

    G1 signature = shareSign(*ts->secret_key, hashG1);
    signature.to_affine_coordinates();

    size_t len = gmp_sprintf(
        sig,
        "%Nd:%Nd",
        signature.X.as_bigint().data, libff::alt_bn128_Fq::num_limbs,
        signature.Y.as_bigint().data, libff::alt_bn128_Fq::num_limbs);
    *sig_len = len;

    return SGX_SUCCESS;
}

sgx_status_t threshsign2_sign_using_enclave(
    threshsign_t *ts,
    const char *hash,
    uint64_t counter,
    char *sig,
    unsigned *sig_len)
{
    sgx_status_t ret;
    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < 32; ++i)
    {
        hash_byte_arr.at(i) = hash[i];
    }
    if (counter > 0)
    {
        for (size_t i = 0; i < 4; ++i)
        {
            hash_byte_arr.at(31 - i) = (uint8_t)(hash_byte_arr.at(31 - i) ^ ((uint8_t)((counter >> 8 * i) & 0xFF)));
        }
    }

    std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
        std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

    G1 hashG1 = HashtoG1(hash_ptr);

    string *xStr = nullptr, *yStr = nullptr;
    xStr = stringFromFq(&(hashG1.X));
    if (xStr == nullptr)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto clean;
    }

    yStr = stringFromFq(&(hashG1.Y));
    if (yStr == nullptr)
    {
        ret = SGX_ERROR_UNEXPECTED;
        goto clean;
    }

    char xStrArg[BUF_LEN];
    char yStrArg[BUF_LEN];

    memset(xStrArg, 0, BUF_LEN);
    memset(yStrArg, 0, BUF_LEN);

    strncpy(xStrArg, xStr->c_str(), BUF_LEN);
    strncpy(yStrArg, yStr->c_str(), BUF_LEN);

    ret = ECALL_TSS(ts->eid, sign, xStrArg, yStrArg, sig, sig_len);

clean:
    SAFE_FREE(xStr);
    SAFE_FREE(yStr);

    return ret;
}

sgx_status_t threshsign2_aggregate_and_verify(
    threshsign_t *ts,
    const char *hash,
    uint64_t counter,
    unsigned sigshare_count,
    const char *sigshares,
    const unsigned *sigshare_lens,
    const int *sigshare_idx,
    char *sig,
    unsigned *sig_len)
{
    ntl_init();
    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < 32; i++)
    {
        hash_byte_arr.at(i) = hash[i];
    }
    if (counter > 0)
    {
        for (size_t i = 0; i < 4; i++)
        {
            hash_byte_arr.at(31 - i) = (uint8_t)(hash_byte_arr.at(31 - i) ^ ((uint8_t)((counter >> 8 * i) & 0xFF)));
        }
    }
    std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
        std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

    G1 hashG1 = HashtoG1(hash_ptr);

    size_t idx = 0;
    vector<size_t> signersVec(sigshare_count);
    vector<G1> sigSharesVec(sigshare_count);
    for (size_t i = 0; i < sigshare_count; i++)
    {
        auto sig_ptr = make_shared<std::string>(&sigshares[idx], sigshare_lens[i]);
        auto result = SplitString(sig_ptr, ":");

        libff::alt_bn128_Fq X(result->at(0).c_str());
        libff::alt_bn128_Fq Y(result->at(1).c_str());

        sigSharesVec[i] = libff::alt_bn128_G1(X, Y, libff::alt_bn128_Fq::one());
        signersVec[i] = sigshare_idx[i],

        idx += sigshare_lens[i];
    }

    vector<Fr> lagr;
    vector<Fr> someOmegas;
    for (auto id : signersVec)
    {
        assertStrictlyLessThan(id, omegas.size());
        someOmegas.push_back(omegas[id]);
    }
    if (_fast_lagrange) {
        lagrange_coefficients(lagr, omegas, someOmegas, signersVec);
    } else {
        lagrange_coefficients_naive(lagr, omegas, someOmegas, signersVec);
    }
    libff::alt_bn128_G1 signature = aggregate(sigSharesVec, lagr);
    // auto r = stringFromG1(&signature);
    // *sig_len = r->size();
    // snprintf(sig, *sig_len, "%s", r->c_str());
    // SAFE_FREE(r);
    signature.to_affine_coordinates();
    size_t len = gmp_sprintf(
        sig,
        "%Nd:%Nd",
        signature.X.as_bigint().data, libff::alt_bn128_Fq::num_limbs,
        signature.Y.as_bigint().data, libff::alt_bn128_Fq::num_limbs);
    *sig_len = len;

    // std::cout << "agg_signature:::" << std::endl;
    // signature.print();
    // fflush(stdout);
    auto p1 = ReducedPairing(signature, libff::alt_bn128_G2::one());
    auto p2 = ReducedPairing(hashG1, *ts->public_key);
    if (p1 != p2)
    {
        return SGX_ERROR_UNEXPECTED;
    }

    return SGX_SUCCESS;
}

sgx_status_t threshsign2_verify(
    threshsign_t *ts,
    const char *hash,
    uint64_t counter,
    const char *sig,
    unsigned sig_len)
{
    std::array<uint8_t, 32> hash_byte_arr;
    for (size_t i = 0; i < 32; i++)
    {
        hash_byte_arr.at(i) = hash[i];
    }
    if (counter > 0)
    {
        for (size_t i = 0; i < 4; i++)
        {
            hash_byte_arr.at(31 - i) = (uint8_t)(hash_byte_arr.at(31 - i) ^ ((uint8_t)((counter >> 8 * i) & 0xFF)));
        }
    }

    std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
        std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

    G1 hashG1 = HashtoG1(hash_ptr);

    auto sig_ptr = make_shared<std::string>(sig, sig_len);
    auto result = SplitString(sig_ptr, ":");

    libff::alt_bn128_Fq X(result->at(0).c_str());
    libff::alt_bn128_Fq Y(result->at(1).c_str());
    // libff::alt_bn128_Fq Z(result->at(2).c_str());

    G1 signature = libff::alt_bn128_G1(X, Y, libff::alt_bn128_Fq::one());

    // std::cout << "verify_signature:::" << std::endl;
    // signature.print();
    // fflush(stdout);
    auto p1 = ReducedPairing(signature, libff::alt_bn128_G2::one());
    auto p2 = ReducedPairing(hashG1, *ts->public_key);
    if (p1 != p2)
    {
        return SGX_ERROR_UNEXPECTED;
    }

    return SGX_SUCCESS;
}

sgx_status_t threshsign2_destroy(threshsign_t *ts)
{
    return sgx_destroy_enclave(ts->eid);
}