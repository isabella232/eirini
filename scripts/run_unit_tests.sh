#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export GO111MODULE=on

main() {
  run_tests
}

run_tests() {
  pushd "$BASEDIR" >/dev/null || exit 1
  mkdir -p coverage
  # TODO: Find a way to not generate multiple files other than the one in the `coverage/` dir
  # If there is no way, cleanup with something like: `find -name *coverage.out -exec rm {} \;`
  # After moving the file in the `coverage` dir to some other name that doesn't match the above.
  ginkgo -cover -outputdir="coverage/" -coverprofile=coverage.out -mod=vendor -p -r -keepGoing --skipPackage=tests -randomizeAllSpecs -randomizeSuites "$@"
  popd >/dev/null || exit 1
}

main "$@"
