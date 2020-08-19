#!/bin/bash

# TODOs:
# - Generate random suffix for job name
# - Implement a cleanup method
# - Cleanup by default but skip if a flag is set (for debugging)

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export EIRINIUSER_PASSWORD="${EIRINIUSER_PASSWORD:-$(pass eirini/docker-hub)}"

is_init_container_running() {
  local pod_name="$1"
  local container_name="$2"
  if [[ "$(kubectl get pods "${pod_name}" \
    --output jsonpath="{.status.initContainerStatuses[?(@.name == \"${container_name}\")].state.running}")" != "" ]]; then
    return 0
  fi
  return 1
}

# Cleanup possible leftovers
kubectl delete job eirini-integration-tests --wait --ignore-not-found
kubectl delete pod --wait $(kubectl get pods | grep eirini-integration-tests | awk '{print $1}')
kubectl apply -f "$BASEDIR"/scripts/assets/test-job-rbac.yml
goml set -d -f "$BASEDIR"/scripts/assets/test-job.yml -p spec.template.spec.containers.0.env.name:EIRINIUSER_PASSWORD.value -v "$EIRINIUSER_PASSWORD" | kubectl apply -f -

pod_name=$(kubectl get pods --selector=job-name=eirini-integration-tests --template '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}')
timeout=30
while [[ $pod_name == "" ]] && [[ ! "$timeout" == "0" ]]; do
  sleep 1
  pod_name=$(kubectl get pods --selector=job-name=eirini-integration-tests)
  timeout=$((timeout - 1))
done
if [[ "${timeout}" == 0 ]]; then
  exit 1
fi

timeout=30
until is_init_container_running "${pod_name}" "wait-for-code" || [[ "$timeout" == "0" ]]; do
  sleep 1
  timeout=$((timeout - 1))
done
if [[ "${timeout}" == 0 ]]; then
  exit 1
fi

kubectl cp "$BASEDIR" "$pod_name":/eirini-code -c wait-for-code
kubectl cp "$(mktemp)" "$pod_name":/eirini-code/tests-can-start -c wait-for-code

kubectl wait pod "$pod_name" --for=condition=Ready
# Tail the test logs.
container_name="tests"
kubectl logs "${pod_name}" \
  --follow \
  --container "${container_name}"

# Wait for the container to terminate and then exit the script with the container's exit code.
jsonpath="{.status.containerStatuses[?(@.name == \"${container_name}\")].state.terminated.exitCode}"
while true; do
  exit_code="$(kubectl get pod "${pod_name}" --output "jsonpath=${jsonpath}")"
  if [[ -n "${exit_code}" ]]; then
    exit "${exit_code}"
  fi
  sleep 1
done
