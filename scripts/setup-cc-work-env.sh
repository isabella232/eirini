#!/bin/bash

set -euo pipefail

readonly CF4K8S_DIR="$HOME/workspace/cf-for-k8s"
readonly CAPIK8S_DIR="$HOME/workspace/capi-k8s-release"
readonly CLUSTER_NAME="cc-work-env"
readonly TMP_DIR="$(mktemp -d)"
readonly KIND_CONF="${TMP_DIR}/kind-config-cc-work-env"
readonly CC_NG_DIR="${HOME}/workspace/capi-release/src/cloud_controller_ng/"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
readonly GCP_SERVICE_ACCOUNT_JSON=$(pass eirini/gcs-eirini-ci-terraform-json-key)

main() {
  echo "Creating a kind cluster with cc code mounted"
  # cleanup-existing-cluster
  # create-kind-config
  # create-kind-cluster
  build_ccng_image
  generate-cf-for-k8s-values
  deploy-cf
}

build_ccng_image() {
  export IMAGE_DESTINATION_CCNG="docker.io/eirini/dev-ccng"
  export IMAGE_DESTINATION_CF_API_CONTROLLERS="docker.io/eirini/dev-controllers"
  export IMAGE_DESTINATION_REGISTRY_BUDDY="docker.io/eirini/dev-registry-buddy"
  git -C "$CAPIK8S_DIR" checkout values/images.yml
  "$CAPIK8S_DIR"/scripts/build-into-values.sh "$CAPIK8S_DIR/values/images.yml"
  "$CAPIK8S_DIR"/scripts/bump-cf-for-k8s.sh

  # update respective vendir directories in cf-for-k8s
  cp -r "$CAPIK8S_DIR/values/" "$CF4K8S_DIR/config/capi/_ytt_lib/capi-k8s-release/"
  cp -r "$CAPIK8S_DIR/templates/" "$CF4K8S_DIR/config/capi/_ytt_lib/capi-k8s-release/"
}

create-kind-config() {
  local temp_conf
  temp_conf="$(mktemp)"

  # https://github.com/cloudfoundry/cf-for-k8s/blob/develop/docs/deploy-local.md
  pushd "$CF4K8S_DIR" || exit 1
  {
    k8s_minor_version="$(yq r supported_k8s_versions.yml newest_version)" # or k8s_minor_version="1.17"
    patch_version=$(wget -q https://registry.hub.docker.com/v1/repositories/kindest/node/tags -O - |
      jq -r '.[].name' | grep -E "^v${k8s_minor_version}.[0-9]+$" |
      cut -d. -f3 | sort -rn | head -1)
    k8s_version="v${k8s_minor_version}.${patch_version}"
    echo "Creating KinD cluster with Kubernetes version ${k8s_version}"
    ls -la "$temp_conf"
    cp deploy/kind/cluster.yml "${KIND_CONF}"
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
    echo ${GCP_SERVICE_ACCOUNT_JSON} >${TMP_DIR}/gcp-service-account-json
    ./hack/generate-values.sh -d vcap.me -g ${TMP_DIR}/gcp-service-account-json >${TMP_DIR}/cf-values.yml
    cat <<EOF >>${TMP_DIR}/cf-values.yml
add_metrics_server_components: true
enable_automount_service_account_token: true
metrics_server_prefer_internal_kubelet_address: true
remove_resource_requirements: true
use_first_party_jwt_tokens: true

load_balancer:
  enable: false
EOF
  }
  popd || exit 1
}

deploy-cf() {
  kapp deploy -a cf -f <(
    ytt -f "$CF4K8S_DIR/config" \
      -f ${TMP_DIR}/cf-values.yml
  ) -y
}

main "$@"
