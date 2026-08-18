#!/usr/bin/env bash
set -euo pipefail

mode="${1:-}"
namespace="${BALEMOH_DEV_NAMESPACE:-balemoh-dev}"
kind_cluster="${BALEMOH_KIND_CLUSTER_NAME:-balemoh}"
vind_cluster="${BALEMOH_VIND_CLUSTER_NAME:-balemoh}"

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "missing required command: $1" >&2
		exit 1
	fi
}

ensure_namespace() {
	local context="$1"
	kubectl --context "$context" create namespace "$namespace" --dry-run=client -o yaml | kubectl --context "$context" apply -f - >/dev/null
}

ensure_kind() {
	require_command kind
	require_command kubectl
	if ! kind get clusters | awk -v expected="$kind_cluster" '$0 == expected { found = 1 } END { exit found ? 0 : 1 }'; then
		kind create cluster --name "$kind_cluster" --config devspace/kind-config.yaml --wait 120s
	fi
	local context="kind-$kind_cluster"
	kubectl config use-context "$context" >/dev/null
	ensure_namespace "$context"
	echo "kind context ready: $context"
}

ensure_vind() {
	require_command vcluster
	require_command kubectl
	vcluster use driver docker >/dev/null
	if ! vcluster describe "$vind_cluster" --driver docker --output json >/dev/null 2>&1; then
		vcluster create "$vind_cluster" --values devspace/vind-values.yaml --connect=false
	fi
	vcluster connect "$vind_cluster"
	local context
	context="$(kubectl config current-context)"
	ensure_namespace "$context"
	echo "vind context ready: $context"
}

delete_kind() {
	require_command kind
	kind delete cluster --name "$kind_cluster"
}

delete_vind() {
	require_command vcluster
	vcluster delete "$vind_cluster"
}

case "$mode" in
	kind)
		ensure_kind
		;;
	kind-down)
		delete_kind
		;;
	vind)
		ensure_vind
		;;
	vind-down)
		delete_vind
		;;
	*)
		echo "usage: $0 {kind|kind-down|vind|vind-down}" >&2
		exit 2
		;;
esac
