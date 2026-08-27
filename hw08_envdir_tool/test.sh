#!/usr/bin/env bash
set -xeuo pipefail

tmp_dir="$(mktemp -d)"
binary="${tmp_dir}/go-envdir"
cleanup() {
  rm -f "${binary}"
  rmdir "${tmp_dir}"
}
trap cleanup EXIT

go build -o "${binary}" .

export HELLO="SHOULD_REPLACE"
export FOO="SHOULD_REPLACE"
export UNSET="SHOULD_REMOVE"
export ADDED="from original env"
export EMPTY="SHOULD_BE_EMPTY"

result=$("${binary}" "$(pwd)/testdata/env" "/bin/bash" "$(pwd)/testdata/echo.sh" arg1=1 arg2=2)
expected='HELLO is ("hello")
BAR is (bar)
FOO is (   foo
with new line)
UNSET is ()
ADDED is (from original env)
EMPTY is ()
arguments are arg1=1 arg2=2'

[ "${result}" = "${expected}" ] || (echo -e "invalid output: ${result}" && exit 1)

echo "PASS"
