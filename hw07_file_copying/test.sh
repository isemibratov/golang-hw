#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

test_dir="$(mktemp -d "${TMPDIR:-/tmp}/hw07_file_copying.XXXXXX")"
binary="$test_dir/go-cp"
output="$test_dir/out.txt"
input="testdata/input.txt"

cleanup() {
  rm -f "$binary" "$output"
  rmdir "$test_dir"
}
trap cleanup EXIT

go build -o "$binary" .

check() {
  local expected="$1"
  shift
  "$binary" -from "$input" -to "$output" "$@"
  cmp "$output" "testdata/$expected"
}

check out_offset0_limit0.txt
check out_offset0_limit10.txt -limit 10
check out_offset0_limit1000.txt -limit 1000
check out_offset0_limit10000.txt -limit 10000
check out_offset100_limit1000.txt -offset 100 -limit 1000
check out_offset6000_limit1000.txt -offset 6000 -limit 1000

echo "PASS"
