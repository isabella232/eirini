#!/bin/bash
set -o errexit

# This script has helpers to run a local registry in a docker container
# and expose it in your local kind cluster. For more information:
# https://kind.sigs.k8s.io/docs/user/local-registry/

readonly REG_NAME='kind-registry'
readonly REG_PORT='5000'

run-local-registry() {
  # patched_cc.ymlcreate registry container unless it already exists
  local running
  running="$(docker inspect -f '{{.State.Running}}' "${REG_NAME}" 2>/dev/null || true)"
  if [ "${running}" != 'true' ]; then
    docker run \
      -d --restart=always -p "${REG_PORT}:5000" --name "${REG_NAME}" \
      registry:2
  fi
}

expose-docker-network() {
  # connect the registry to the cluster network
  # (the network may already be connected)
  docker network connect "kind" "${REG_NAME}" || true
}

document-local-registry() {
  # Document the local registry
  # https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
}
