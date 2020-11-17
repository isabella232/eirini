#!/bin/bash

set -euo pipefail

readonly CF4K8S_DIR="$HOME/workspace/cf-for-k8s"
readonly CLUSTER_NAME="cc-work-env"
readonly TMP_DIR="$(mktemp -d)"
readonly KIND_CONF="${TMP_DIR}/kind-config-cc-work-env"
readonly CC_NG_DIR="${HOME}/workspace/capi-release/src/cloud_controller_ng/"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
source "$SCRIPT_DIR/local-registry-helpers.sh"

main() {
  echo "Creating a kind cluster with cc code mounted"
  cleanup-existing-cluster
  create-kind-config
  run-local-registry
  expose-docker-network
  create-kind-cluster
  document-local-registry

  generate-cf-for-k8s-values
  deploy-cf
}

create-kind-config() {
  local temp_conf
  temp_conf="$(mktemp)"
  trap "rm $temp_conf" EXIT

  cat <<EOF >"${temp_conf}"
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REG_PORT}"]
    endpoint = ["http://${REG_NAME}:${REG_PORT}"]
nodes:
- role: control-plane
  extraMounts:
  - containerPath: /cc-workspace
    hostPath: $CC_NG_DIR
    readOnly: true
EOF

  # https://github.com/cloudfoundry/cf-for-k8s/blob/develop/docs/deploy-local.md
  pushd "$CF4K8S_DIR" || exit 1
  {
    k8s_minor_version="$(yq r supported_k8s_versions.yml newest_version)" # or k8s_minor_version="1.17"
    patch_version=$(wget -q https://registry.hub.docker.com/v1/repositories/kindest/node/tags -O - |
      jq -r '.[].name' | grep -E "^v${k8s_minor_version}.[0-9]+$" |
      cut -d. -f3 | sort -rn | head -1)
    k8s_version="v${k8s_minor_version}.${patch_version}"
    echo "Creating KinD cluster with Kubernetes version ${k8s_version}"
    yq merge deploy/kind/cluster.yml "$temp_conf" >"${KIND_CONF}"
  }
  popd || exit 1
}

cleanup-existing-cluster() {
  if kind get clusters | grep -q "$CLUSTER_NAME"; then
    while true; do
      read -p "Found a cluster named ${CLUSTER_NAME}. Should I delete it and continue? [y/n]" yn
      case $yn in
        [Yy]*)
          kind delete cluster --name "$CLUSTER_NAME"
          break
          ;;
        [Nn]*) exit ;;
        *) echo "Please answer yes or no." ;;
      esac
    done
  fi
}

create-kind-cluster() {
  kind create cluster --config="$KIND_CONF" --image kindest/node:${k8s_version} --name ${CLUSTER_NAME}
}

generate-cf-for-k8s-values() {
  pushd "$CF4K8S_DIR" || exit 1
  {
    ./hack/generate-values.sh -d vcap.me >${TMP_DIR}/cf-values.yml
    cat <<EOF >>${TMP_DIR}/cf-values.yml
add_metrics_server_components: true
enable_automount_service_account_token: true
metrics_server_prefer_internal_kubelet_address: true
remove_resource_requirements: true
use_first_party_jwt_tokens: true

load_balancer:
  enable: false

app_registry:
  hostname: "localhost:${REG_PORT}"
  username: "a"
  password: "a"
  repository_prefix: "/"
EOF
  }
  popd || exit 1
}

deploy-cf() {
  kapp deploy -a cf -f <(
    ytt -f "$CF4K8S_DIR/config" \
      -f ${SCRIPT_DIR}/assets/local-cloud-controller.yml \
      -f ${TMP_DIR}/cf-values.yml >${TMP_DIR}/cf-for-k8s-rendered.yml
  ) -y
}

main "$@"
