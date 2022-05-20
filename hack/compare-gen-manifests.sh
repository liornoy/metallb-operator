#!/bin/bash

. $(dirname "$0")/common.sh

NATIVE_MANIFESTS_DIR="bindata/deployment/native"
NATIVE_MANIFESTS_FILE="metallb-native.yaml"

FRR_MANIFESTS_FILE="metallb-frr.yaml"
FRR_MANIFESTS_DIR="bindata/deployment/frr"

FRR_WITH_WEBHOOKS_MANIFESTS_FILE="metallb-frr-with-webhooks.yaml"
FRR_WITH_WEBHOOKS_MANIFESTS_DIR="bindata/deployment/frr-with-webhooks"


mv ${NATIVE_MANIFESTS_DIR}/${NATIVE_MANIFESTS_FILE} _cache/${NATIVE_MANIFESTS_FILE}.manifests
mv config/metallb_rbac/${NATIVE_MANIFESTS_FILE} _cache/${NATIVE_MANIFESTS_FILE}.rbac

mv ${FRR_MANIFESTS_DIR}/${FRR_MANIFESTS_FILE} _cache/${FRR_MANIFESTS_FILE}.manifests
mv config/metallb_rbac/${FRR_MANIFESTS_FILE} _cache/${FRR_MANIFESTS_FILE}.rbac

mv ${FRR_WITH_WEBHOOKS_MANIFESTS_DIR}/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE} _cache/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE}.manifests
mv config/metallb_rbac/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE} _cache/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE}.rbac

hack/generate-metallb-manifests.sh

diff ${NATIVE_MANIFESTS_DIR}/${NATIVE_MANIFESTS_FILE} _cache/${NATIVE_MANIFESTS_FILE}.manifests -q || { echo "Current native MetalLB manifests differ from the manifests in the MetalLB repo"; exit 1; }
diff config/metallb_rbac/${NATIVE_MANIFESTS_FILE} _cache/${NATIVE_MANIFESTS_FILE}.rbac -q || { echo "Current native MetalLB RBAC manifests differ from the RBAC manifests in the MetalLB repo"; exit 1; }

diff ${FRR_MANIFESTS_DIR}/${FRR_MANIFESTS_FILE} _cache/${FRR_MANIFESTS_FILE}.manifests -q || { echo "Current FRR MetalLB manifests differ from the manifests in the MetalLB repo"; exit 1; }
diff config/metallb_rbac/${FRR_MANIFESTS_FILE} _cache/${FRR_MANIFESTS_FILE}.rbac -q || { echo "Current FRR MetalLB RBAC manifests differ from the RBAC manifests in the MetalLB repo"; exit 1; }

diff ${FRR_WITH_WEBHOOKS_MANIFESTS_DIR}/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE} _cache/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE}.manifests -q || { echo "Current FRR with webhooks MetalLB manifests differ from the manifests in the MetalLB repo"; exit 1; }
diff config/metallb_rbac/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE} _cache/${FRR_WITH_WEBHOOKS_MANIFESTS_FILE}.rbac -q || { echo "Current FRR with webhooks MetalLB RBAC manifests differ from the RBAC manifests in the MetalLB repo"; exit 1; }
