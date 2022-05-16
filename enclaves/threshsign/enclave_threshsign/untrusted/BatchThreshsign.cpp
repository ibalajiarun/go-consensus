#include "Threshsign.h"
#include "BatchThreshsign.h"
#include "BLSPrivateKeyShareSGX.h"
#include "ThreshsignInternal.h"

int threshsign_batch_sign(threshsign_t *ts,
                          unsigned num_requests,
                          const char *hash,
                          uint32_t *ctr_id,
                          uint64_t *counter,
                          char *sig,
                          unsigned *sig_len)
{
    for (int i = 0; i < num_requests; i++)
    {
        int ret = threshsign_sign(ts,
                                  &hash[32 * i], 32,
                                  ctr_id[i],
                                  counter[i],
                                  &sig[256 * i], &sig_len[i]);
        if (ret != 0)
        {
            return ret;
        }
    }
    return 0;
}

int threshsign_batch_sign_using_enclave(threshsign_t *ts,
                                        unsigned num_requests,
                                        const char *hash,
                                        uint32_t *ctr_id,
                                        uint64_t *counter,
                                        char *sig,
                                        unsigned *sig_len)
{
    for (int i = 0; i < num_requests; i++)
    {
        int ret = threshsign_sign_using_enclave(ts,
                                                &hash[32 * i], 32,
                                                ctr_id[i],
                                                counter[i],
                                                &sig[256 * i], &sig_len[i]);
        if (ret != 0)
        {
            return ret;
        }
    }
    return 0;
}

int threshsign_batch_sign_using_enclave_batch(threshsign_t *ts,
                                              unsigned num_requests,
                                              const char *hashes,
                                              uint32_t *ctr_id,
                                              uint64_t *counters,
                                              char *sigs,
                                              unsigned *sig_lens)
{
    auto hash_ptrs = std::make_shared<std::vector<std::shared_ptr<std::array<uint8_t, 32>>>>();
    hash_ptrs->reserve(num_requests);

    for (int i = 0; i < num_requests; i++)
    {
        const char *hash = &hashes[32 * i];
        unsigned hash_len = 32;
        uint64_t counter = counters[i];

        std::array<uint8_t, 32> hash_byte_arr;
        for (size_t i = 0; i < hash_len; i++)
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
        // std::shared_ptr<std::array<uint8_t, 32>> hash_ptr =
        // std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr);

        hash_ptrs->emplace_back(std::make_shared<std::array<uint8_t, 32>>(hash_byte_arr));
    }

    // for (const auto &hash : *hash_ptrs)
    // {
    //     std::cout << "hash: ";
    //     for (const auto &s : *hash)
    //     {
    //         std::cout << int(s) << ' ';
    //     }
    //     std::cout << std::endl;
    // }

    auto sigShares = ts->sgx_sshare->batchSignWithHelperSGX(hash_ptrs, ts->node_index);

    for (int i = 0; i < num_requests; i++)
    {
        char *sig = &sigs[256 * i];
        unsigned *sig_len = &sig_lens[i];
        auto sigSharestr = sigShares->at(i)->toString();
        *sig_len = sigSharestr->size();
        // printf("sig %s\n", sigSharestr->c_str());
        // printf("sig_len %d\n", sigSharestr->size());
        // fflush(stdout);
        snprintf(sig, *sig_len + 1, "%s", sigSharestr->c_str());
    }
    return 0;
}

int threshsign_batch_aggregate_and_verify(threshsign_t *ts,
                                          unsigned num_requests,
                                          const char *hashes,
                                          uint32_t *ctr_id,
                                          uint64_t *counters,
                                          const char *sigshare_batch,
                                          unsigned *ss_batch_lens,
                                          unsigned *ss_batch_counts,
                                          unsigned *ss_lens,
                                          const int *sigshare_ids,
                                          char *sig,
                                          unsigned *sig_len)
{
    int ss_idx = 0, ssl_idx = 0;
    for (int i = 0; i < num_requests; i++)
    {
        int ret = threshsign_aggregate_and_verify(ts,
                                                  &hashes[32 * i], 32,
                                                  counters[i],
                                                  ctr_id[i],
                                                  ss_batch_counts[i],
                                                  &sigshare_batch[ss_idx],
                                                  &ss_lens[ssl_idx],
                                                  &sigshare_ids[ssl_idx],
                                                  &sig[256 * i], &sig_len[i]);
        if (ret != 0)
        {
            return ret;
        }
        ss_idx += ss_batch_lens[i];
        ssl_idx += ss_batch_counts[i];
    }
    return 0;
}

int threshsign_batch_verify(threshsign_t *ts,
                            unsigned num_requests,
                            const char *hashes,
                            uint32_t *ctr_id,
                            uint64_t *counters,
                            const char *combined_sigs,
                            unsigned *combines_siglens)
{
    int ss_idx = 0;
    for (int i = 0; i < num_requests; i++)
    {
        int ret = threshsign_verify(ts,
                                    &hashes[32 * i], 32,
                                    ctr_id[i],
                                    counters[i],
                                    &combined_sigs[ss_idx], combines_siglens[i]);
        if (ret != 0)
        {
            return ret;
        }
        ss_idx += combines_siglens[i];
    }
    return 0;
}