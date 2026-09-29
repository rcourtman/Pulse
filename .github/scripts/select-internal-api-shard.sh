#!/usr/bin/env bash
# Print the internal/api top-level tests that belong to one CI shard.
#
# Usage: select-internal-api-shard.sh <weights-file> <shard-count> <shard-index>
# The full test list, in go test's own order, is read from stdin.
#
# Shards are contiguous runs of go test's order, because some internal/api
# tests still depend on package state left by the tests just before them.
# Cut points are chosen by estimated run time instead of test count. Every
# test weighs its seconds from the weights file, or DEFAULT_WEIGHT when the
# file does not name it (new tests, renamed tests). The smallest per-shard
# capacity that lets the list be cut into at most <shard-count> contiguous
# runs is found by binary search, then the list is filled greedily up to that
# capacity. If fewer runs than shards would result, the tail is cut early so
# every shard still gets at least one test.
#
# The weights only move the cut points. Each test is assigned exactly one
# shard index in 0..count-1 by a single pass over the list, so a stale,
# missing or wrong weight can make shards uneven but can never drop or repeat
# a test.
set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: $0 <weights-file> <shard-count> <shard-index> < tests" >&2
  exit 2
fi
weights=$1
count=$2
index=$3

case "$count" in '' | *[!0-9]*) echo "shard count must be a positive integer" >&2; exit 2 ;; esac
case "$index" in '' | *[!0-9]*) echo "shard index must be a non-negative integer" >&2; exit 2 ;; esac
if [ "$count" -lt 1 ] || [ "$index" -ge "$count" ]; then
  echo "shard index ${index} is outside 0..$((count - 1))" >&2
  exit 2
fi
if [ ! -f "$weights" ]; then
  echo "weights file ${weights} not found" >&2
  exit 2
fi

awk -v n="$count" -v want="$index" -v default_weight="${DEFAULT_WEIGHT:-0.05}" '
  function centis(seconds,    c) {
    c = int(seconds * 100 + 0.5)
    return c < 1 ? 1 : c
  }
  # Number of contiguous runs a greedy fill up to capacity cap produces.
  function runs(cap,    k, used, groups) {
    groups = 1
    used = 0
    for (k = 1; k <= total; k++) {
      if (used > 0 && used + cost[k] > cap) {
        groups++
        used = 0
      }
      used += cost[k]
    }
    return groups
  }
  FILENAME == ARGV[1] {
    if ($0 ~ /^[[:space:]]*(#|$)/) next
    if (NF != 2 || $2 !~ /^[0-9]+(\.[0-9]+)?$/ || $2 + 0 <= 0) {
      printf "invalid weights line %d: %s\n", FNR, $0 > "/dev/stderr"
      bad = 1
      exit 1
    }
    weight[$1] = $2 + 0
    next
  }
  NF {
    total++
    name[total] = $1
    cost[total] = centis(($1 in weight) ? weight[$1] : default_weight)
    sum += cost[total]
    if (cost[total] > heaviest) heaviest = cost[total]
  }
  END {
    if (bad) exit 1
    if (total < n) {
      printf "%d tests cannot fill %d shards\n", total, n > "/dev/stderr"
      exit 1
    }
    lo = heaviest
    hi = sum
    while (lo < hi) {
      mid = int((lo + hi) / 2)
      if (runs(mid) <= n) hi = mid
      else lo = mid + 1
    }
    shard = 0
    used = 0
    for (k = 1; k <= total; k++) {
      if (used > 0 && used + cost[k] > lo) {
        shard++
        used = 0
      }
      used += cost[k]
      if (shard == want) print name[k]
      # Once the tests left equal the shards left, give each its own shard
      # so no shard is empty.
      if (total - k > 0 && total - k == n - 1 - shard) {
        shard++
        used = 0
      }
    }
  }
' "$weights" -
