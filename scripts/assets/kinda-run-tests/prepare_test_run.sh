#!/bin/bash

set -eux

readonly TMP_DIR=/workspace
mkdir -p "$TMP_DIR"
readonly EIRINI_DIR="$TMP_DIR/eirini"
readonly EIRINI_COMMIT=${EIRINI_COMMIT:-"HEAD"}

if [ -d "/eirini" ]; then
  cp -a /eirini "$TMP_DIR"
else
  git clone https://github.com/cloudfoundry-incubator/eirini.git "$EIRINI_DIR"
fi

pushd "$EIRINI_DIR"
{
  git checkout "$EIRINI_COMMIT"
  $TEST_SCRIPT
}
popd
