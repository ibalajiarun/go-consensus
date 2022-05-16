#ifndef _THRESHSIGN_INTERNAL_H_
#define _THRESHSIGN_INTERNAL_H_

#include <memory>

#include <polycrypto/PolyCrypto.h>

#include "sgx_urts.h"

using namespace libpolycrypto;

struct threshsign
{
    sgx_enclave_id_t eid;

    std::shared_ptr<libff::alt_bn128_Fr> secret_key;
    std::shared_ptr<libff::alt_bn128_G2> public_key;

    int node_index;
    size_t threshold, total;
};

#endif /* !_THRESHSIGN_INTERNAL_H_ */
