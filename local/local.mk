CLUSTER_NAME=ccx-exporter
CONTEXT_NAME=kind-$(CLUSTER_NAME)
LOCAL_KUBE_CONFIG=$(BUILD_DIR)/kubeconfig
NAMESPACE=ccx-exporter

.PHONY: local.kind local.kubeconfig local.context local.namespace local.kraft local.valkey local.localstack local.import local.delete

## local.kind: Start kind cluster
local.kind:
	@kind create cluster -n $(CLUSTER_NAME) --image kindest/node:v1.28.0 --config $(CURDIR)/local/kind-config.yaml

## local.kubeconfig: Export kubeconfig
local.kubeconfig: build.prepare
	@kind get kubeconfig -n $(CLUSTER_NAME) > $(LOCAL_KUBE_CONFIG)

local.context: local.kubeconfig
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl config use-context $(CONTEXT_NAME)

local.namespace: local.context
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl create namespace $(NAMESPACE)

## local.kraft: Start kraft (kafka w/o zookeeper) exposed on 32323
local.kraft: local.context
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl apply -n $(NAMESPACE) -f $(CURDIR)/local/kraft.yaml

## local.valkey: Start valkey exposed on 30379
local.valkey: local.context
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl apply -n $(NAMESPACE) -f $(CURDIR)/local/valkey-custo.yaml
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) oc process -f $(CURDIR)/openshift/valkey.yaml --local | oc apply -n $(NAMESPACE) -f -

## local.localstack: Start s3 localstack exposed on 31566
local.localstack: local.context
	@helm repo add localstack https://localstack.github.io/helm-charts
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) helm install -n $(NAMESPACE) localstack localstack/localstack --set-string persistence.enabled=true

## local.delete: Stop kind server
local.delete: local.context
	@kind delete cluster -n $(CLUSTER_NAME)

## local.import: Import docker images to kind
local.import: local.context
	@kind load docker-image -n $(CLUSTER_NAME) $(IMAGE_FULL)

## local.wait: Wait for post-install to be ready
local.wait: local.context
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl wait -n $(NAMESPACE) --timeout=120s --for=jsonpath='{.status.readyReplicas}'=1 sts/kraft
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl wait -n $(NAMESPACE) --timeout=120s --for=jsonpath='{.status.readyReplicas}'=1 sts/valkey-ccx-exporter
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl wait -n $(NAMESPACE) --timeout=120s --for=jsonpath='{.status.readyReplicas}'=1 deployment/localstack
