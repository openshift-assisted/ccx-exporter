NAMESPACE                := ccx-exporter
DEPLOYMENT_NAME          := ccx-exporter
VALKEY_URL               := valkey-ccx-exporter-0.valkey-ccx-exporter-headless:6379
S3_BUCKET_SECRETNAME     := ccx-processing-result
S3_DLQ_BUCKET_SECRETNAME := ccx-processing-dlq
KAFKA_TOPIC              := assisted-service-events

CLUSTER_NAME      := ccx-exporter
LOCAL_KUBE_CONFIG := $(BUILD_DIR)/kubeconfig
DEPLOYMENT_DIR    := $(BUILD_DIR)/$(DEPLOYMENT_NAME)
KUBECTL           := kubectl --kubeconfig=$(LOCAL_KUBE_CONFIG)
OC                := oc --kubeconfig=$(LOCAL_KUBE_CONFIG)
HELM              := helm --kubeconfig=$(LOCAL_KUBE_CONFIG)
KUBE_WAIT         := $(KUBECTL) wait -n $(NAMESPACE) --timeout=120s --for=jsonpath='{.status.readyReplicas}'=1

LOGS_LEVEL := 10

.PHONY: local.kind
## local.kind: Start kind cluster
local.kind: test.prepare
	@kind create cluster -n $(CLUSTER_NAME) --image kindest/node:v1.28.0 --config $(CURDIR)/local/kind-config.yaml --wait 1m

.PHONY: local.kubeconfig
## local.kubeconfig: Export kubeconfig
local.kubeconfig: build.prepare
	@kind get kubeconfig -n $(CLUSTER_NAME) > $(LOCAL_KUBE_CONFIG)

.PHONY: local.namespace
local.namespace: local.kubeconfig
	@$(KUBECTL) create namespace $(NAMESPACE)

.PHONY: local.kraft
## local.kraft: Start kraft (kafka w/o zookeeper) exposed on 32323
local.kraft: local.kubeconfig
	@$(KUBECTL) apply -n $(NAMESPACE) -f $(CURDIR)/local/kraft.yaml

.PHONY: local.valkey
## local.valkey: Start valkey exposed on 30379
local.valkey: local.kubeconfig
	@$(KUBECTL) apply -n $(NAMESPACE) -f $(CURDIR)/local/valkey-custo.yaml
	@$(OC) process -f $(CURDIR)/openshift/valkey.yaml --local | $(OC) apply -n $(NAMESPACE) -f -

.PHONY: local.valkey.e2e
local.valkey.e2e: local.kubeconfig
	@$(OC) process -f $(CURDIR)/openshift/valkey.yaml --local -p VALKEY_NAME=$(VALKEY_NAME) | $(OC) apply -n $(NAMESPACE) -f -
	@$(KUBE_WAIT) sts/$(VALKEY_NAME)

.PHONY: local.delete.valkey.e2e
local.delete.valkey.e2e: local.kubeconfig
	@$(OC) process -f $(CURDIR)/openshift/valkey.yaml --local -p VALKEY_NAME=$(VALKEY_NAME) | $(OC) delete -n $(NAMESPACE) -f -

## local.localstack: Start s3 localstack exposed on 31566
.PHONY: local.localstack
local.localstack: local.kubeconfig
	@$(HELM) repo add localstack https://localstack.github.io/helm-charts
	@$(HELM) install -n $(NAMESPACE) localstack localstack/localstack --set-string persistence.enabled=true

## local.localstack.buckets: Create defaults buckets
.PHONY: local.localstack.buckets
local.localstack.buckets: local.kubeconfig
	@$(KUBECTL) exec -t deploy/localstack -- awslocal s3api create-bucket --bucket ccx-processing-result
	@$(KUBECTL) exec -t deploy/localstack -- awslocal s3api create-bucket --bucket ccx-processing-dlq

.PHONY: local.delete
## local.delete: Stop kind server
local.delete:
	@kind delete cluster -n $(CLUSTER_NAME)

.PHONY: local.import
## local.import: Import docker images to kind
local.import: local.kubeconfig
	@kind load docker-image -n $(CLUSTER_NAME) $(IMAGE_FULL)

.PHONY: local.wait
## local.wait: Wait for post-install to be ready
local.wait: local.kubeconfig
	@$(KUBE_WAIT) sts/kraft
	@$(KUBE_WAIT) sts/valkey-ccx-exporter
	@$(KUBE_WAIT) deployment/localstack

.PHONY: local.processing.secret
## local.processing.secret: Deploy processing secrets
local.processing.secret: local.kubeconfig
	@$(KUBECTL) apply -f $(CURDIR)/local/processing-custo.yaml

.PHONY: local.processing
## local.processing: Deploy processing
local.processing: local.kubeconfig
	@mkdir -p $(DEPLOYMENT_DIR)
	@$(OC) process -o yaml \
		-f $(CURDIR)/openshift/processing.yaml --local \
		-p IMAGE_PULL_POLICY=Never \
		-p IMAGE_TAG=$(GIT_COMMIT) \
		-p LOGS_LEVEL="$(LOGS_LEVEL)" \
		-p DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) \
		-p VALKEY_URL=$(VALKEY_URL) \
		-p S3_USE_PATH_STYLE=true \
		-p S3_BUCKET_SECRETNAME=$(S3_BUCKET_SECRETNAME) \
		-p S3_DLQ_BUCKET_SECRETNAME=$(S3_DLQ_BUCKET_SECRETNAME) \
		-p KAFKA_TOPIC=$(KAFKA_TOPIC) \
		-p KAFKA_USE_SCRAM_AUTH=false \
		-p KAFKA_USE_TLS=false \
		-p SKIP_ACL=true \
		> $(DEPLOYMENT_DIR)/template.yaml
	@cp $(CURDIR)/local/kustomization.yaml $(DEPLOYMENT_DIR)/
	@cp $(CURDIR)/local/add-coverage.yaml $(DEPLOYMENT_DIR)/
	@$(OC) kustomize $(DEPLOYMENT_DIR) | $(OC) apply -n $(NAMESPACE) -f -
	@$(KUBE_WAIT) deployment/$(DEPLOYMENT_NAME)
	@rm -fr $(DEPLOYMENT_DIR)

.PHONY: local.processing.update
## local.processing.update: Rebuild & redeploy the image
local.processing.update: build.docker local.import local.processing

.PHONY: local.delete.processing
local.delete.processing: local.kubeconfig
	@$(OC) process \
		-f $(CURDIR)/openshift/processing.yaml --local \
		-p DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) \
	| $(OC) delete -n $(NAMESPACE) -f -
