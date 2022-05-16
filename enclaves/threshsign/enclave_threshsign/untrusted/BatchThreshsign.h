#ifndef _BATCH_THRESHSIGN_H_
#define _BATCH_THRESHSIGN_H_

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <string.h>

#include "sgx_error.h" /* sgx_status_t */
#include "sgx_eid.h"   /* sgx_enclave_id_t */

#include "Threshsign.h"

#if defined(__cplusplus)
extern "C"
{
#endif

    int threshsign_batch_sign(threshsign_t *ts,
                              unsigned num_requests,
                              const char *hashes,
                              uint32_t *ctr_id,
                              uint64_t *counters,
                              char *sig,
                              unsigned *sig_len);

    int threshsign_batch_sign_using_enclave(threshsign_t *ts,
                                            unsigned num_requests,
                                            const char *hashes,
                                            uint32_t *ctr_id,
                                            uint64_t *counters,
                                            char *sig,
                                            unsigned *sig_len);

    int threshsign_batch_sign_using_enclave_batch(threshsign_t *ts,
                                                  unsigned num_requests,
                                                  const char *hashes,
                                                  uint32_t *ctr_id,
                                                  uint64_t *counters,
                                                  char *sigs,
                                                  unsigned *sig_lens);

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
                                              unsigned *sig_len);

    int threshsign_batch_verify(threshsign_t *ts,
                                unsigned num_requests,
                                const char *hashes,
                                uint32_t *ctr_id,
                                uint64_t *counters,
                                const char *combined_sigs,
                                unsigned *combines_siglens);

#if defined(__cplusplus)
}
#endif

#endif /* !_BATCH_THRESHSIGN_H_ */
