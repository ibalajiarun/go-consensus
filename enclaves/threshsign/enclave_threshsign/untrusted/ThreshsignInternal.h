#ifndef _THRESHSIGN_INTERNAL_H_
#define _THRESHSIGN_INTERNAL_H_

#include "sgx_urts.h"

#include <bls/BLSPrivateKey.h>
#include <bls/BLSSigShareSet.h>
#include <bls/BLSPublicKeyShare.h>
#include "BLSPrivateKeyShareSGX.h"

struct threshsign
{
    sgx_enclave_id_t eid;

    std::shared_ptr<BLSPrivateKeyShare> sshare;
    std::shared_ptr<BLSPrivateKeyShareSGX> sgx_sshare;
    std::shared_ptr<BLSPublicKey> common_pkey;

    int node_index;
    size_t threshold, total;
};

#endif /* !_APP_H_ */
