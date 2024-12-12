CLUSTER_NAME      := ccx-exporter
CONTEXT_NAME      := kind-$(CLUSTER_NAME)
LOCAL_KUBE_CONFIG := $(BUILD_DIR)/kubeconfig
KUBE_ENV          := KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG)
NAMESPACE         := ccx-exporter
KUBE_WAIT         := $(KUBE_ENV) kubectl wait -n $(NAMESPACE) --timeout=120s --for=jsonpath='{.status.readyReplicas}'=1

LOGS_LEVEL := 10

DEPLOYMENT_NAME := ccx-exporter
VALKEY_URL      := valkey-ccx-exporter-0.valkey-ccx-exporter-headless:6379
DQL_S3_BUCKET   := ccx-processing-dlq
KAFKA_TOPIC     := assisted-service-events
S3_BUCKET       := ccx-processing-result

.PHONY: local.kind
## local.kind: Start kind cluster
local.kind:
	@kind create cluster -n $(CLUSTER_NAME) --image kindest/node:v1.28.0 --config $(CURDIR)/local/kind-config.yaml

.PHONY: local.kubeconfig
## local.kubeconfig: Export kubeconfig
local.kubeconfig: build.prepare
	@kind get kubeconfig -n $(CLUSTER_NAME) > $(LOCAL_KUBE_CONFIG)

.PHONY: local.context
local.context: local.kubeconfig
	@$(KUBE_ENV) kubectl config use-context $(CONTEXT_NAME)
	@$(KUBE_ENV) kubectl config set-context --current --namespace=$(NAMESPACE)

.PHONY: local.namespace
local.namespace: local.context
	@$(KUBE_ENV) kubectl create namespace $(NAMESPACE)

.PHONY: local.kraft
## local.kraft: Start kraft (kafka w/o zookeeper) exposed on 32323
local.kraft: local.context
	@$(KUBE_ENV) kubectl apply -n $(NAMESPACE) -f $(CURDIR)/local/kraft.yaml

.PHONY: local.valkey
## local.valkey: Start valkey exposed on 30379
local.valkey: local.context
	@$(KUBE_ENV) kubectl apply -n $(NAMESPACE) -f $(CURDIR)/local/valkey-custo.yaml
	@$(KUBE_ENV) oc process -f $(CURDIR)/openshift/valkey.yaml --local | oc apply -n $(NAMESPACE) -f -

.PHONY: local.valkey.e2e
local.valkey.e2e: local.context
	@$(KUBE_ENV) oc process -f $(CURDIR)/openshift/valkey.yaml --local -p VALKEY_NAME=$(VALKEY_NAME) | oc apply -n $(NAMESPACE) -f -
	@$(KUBE_WAIT) sts/$(VALKEY_NAME)

.PHONY: local.delete.valkey.e2e
local.delete.valkey.e2e: local.context
	@$(KUBE_ENV) oc process -f $(CURDIR)/openshift/valkey.yaml --local -p VALKEY_NAME=$(VALKEY_NAME) | oc delete -n $(NAMESPACE) -f -

## local.localstack: Start s3 localstack exposed on 31566
.PHONY: local.localstack
local.localstack: local.context
	@helm repo add localstack https://localstack.github.io/helm-charts
	@$(KUBE_ENV) helm install -n $(NAMESPACE) localstack localstack/localstack --set-string persistence.enabled=true

.PHONY: local.delete
## local.delete: Stop kind server
local.delete: local.context
	@kind delete cluster -n $(CLUSTER_NAME)

.PHONY: local.import
## local.import: Import docker images to kind
local.import: local.context
	@kind load docker-image -n $(CLUSTER_NAME) $(IMAGE_FULL)

.PHONY: local.wait
## local.wait: Wait for post-install to be ready
local.wait: local.context
	@$(KUBE_WAIT) sts/kraft
	@$(KUBE_WAIT) sts/valkey-ccx-exporter
	@$(KUBE_WAIT) deployment/localstack

.PHONY: local.processing.secret
## local.processing.secret: Deploy processing secrets
local.processing.secret: local.context
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) kubectl apply -f $(CURDIR)/local/processing-custo.yaml

.PHONY: local.processing
## local.processing: Deploy processing
local.processing: local.context
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) oc process \
		-f $(CURDIR)/openshift/processing.yaml --local \
		-p IMAGE_PULL_POLICY=Never \
		-p IMAGE_TAG=$(GIT_COMMIT) \
		-p LOGS_LEVEL="$(LOGS_LEVEL)" \
		-p DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) \
		-p VALKEY_URL=$(VALKEY_URL) \
		-p KAFKA_TOPIC=$(KAFKA_TOPIC) \
		-p S3_USE_PATH_STYLE=true \
		-p S3_BASE_ENDPOINT=http://localstack:4566 \
		-p S3_BUCKET=$(S3_BUCKET) \
		-p DQL_S3_BUCKET=$(DQL_S3_BUCKET) \
	| oc apply -n $(NAMESPACE) -f -
	@$(KUBE_WAIT) deployment/$(DEPLOYMENT_NAME)

.PHONY: local.processing.update
## local.processing.update: Rebuild & redeploy the image
local.processing.update: build.docker local.import local.processing

.PHONY: local.delete.processing
local.delete.processing: local.context
	@KUBECONFIG=$(KUBECONFIG):$(LOCAL_KUBE_CONFIG) oc process \
		-f $(CURDIR)/openshift/processing.yaml --local \
		-p DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) \
	| oc delete -n $(NAMESPACE) -f -
