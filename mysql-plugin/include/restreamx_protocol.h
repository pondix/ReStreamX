#pragma once

#include <stdint.h>

struct restreamx_segment_header {
  const char *range_id;
  const char *txn_id;
  uint64_t epoch;
  uint64_t commit_index;
  uint32_t checksum;
};
