#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export EIRINIUSER_PASSWORD="${EIRINIUSER_PASSWORD:-$(pass eirini/docker-hub)}"

goml set -d -f "$BASEDIR"/scripts/assets/test-job.yml -p spec.template.spec.containers.0.env.name:EIRINIUSER_PASSWORD.value -v "$EIRINIUSER_PASSWORD" | kubectl apply -f -

sleep 5

pod_name=$(kubectl get pods | grep eirini-integration | awk '{ print $1 }')
kubectl cp "$BASEDIR" "$pod_name":/eirini-code -c wait-for-code
kubectl cp "$(mktemp)" "$pod_name":/eirini-code/tests-can-start -c wait-for-code
