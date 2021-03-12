VERSION ?= 0.1.0
WEBHOOK_BIN ?= centrify-webhook-server
SECRET_INJECTOR_BIN ?= centrify-secret-injector
APP_LAUNCHER_BIN ?= centrify-app-launcher
BUILD_DIR ?= build
TARGET_WEBHOOK = ${BUILD_DIR}/${WEBHOOK_BIN}
TARGET_SECRET_INJECTOR = ${BUILD_DIR}/${SECRET_INJECTOR_BIN}
TARGET_APP_LAUNCHER = ${BUILD_DIR}/${APP_LAUNCHER_BIN}
LDFLAGS=-ldflags "-X=main.VERSION=$(VERSION)"

DOCKER_REGISTRY ?= centrify
WEBHOOK_DOCKER_IMAGE = ${DOCKER_REGISTRY}/webhook-server
SECRET_INJECTOR_OAUTH_DOCKER_IMAGE = ${DOCKER_REGISTRY}/secret-injector-oauth
SECRET_INJECTOR_DMC_DOCKER_IMAGE = ${DOCKER_REGISTRY}/secret-injector-dmc


build: build-webhook build-secret-injector build-app-launcher

image: docker-webhook docker-secret-injector-oauth docker-secret-injector-dmc

build-webhook:
	echo "Building webhook server ...";
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -mod=mod -a -o $(TARGET_WEBHOOK) ./webhook;

build-secret-injector:
	echo "Building secret-injector ...";
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -mod=mod -a -o $(TARGET_SECRET_INJECTOR) ./secret-injector;

build-app-launcher:
	echo "Building app-launcher ...";
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -mod=mod -a -o $(TARGET_APP_LAUNCHER) ./app-launcher;

docker-webhook: ## Build webhook image
	echo "Building webhook docker image ..."
	docker build -t ${WEBHOOK_DOCKER_IMAGE} -f build/Dockerfile.webhook .

docker-secret-injector-oauth: ## Build secret-injector-oauth image
	echo "Building secret-injector docker image ..."
	docker build -t ${SECRET_INJECTOR_OAUTH_DOCKER_IMAGE} -f build/Dockerfile.injector-oauth .

docker-secret-injector-dmc: ## Build secret-injector-dmc image
	echo "Building secret-injector-dmc docker image ..."
	docker build -t ${SECRET_INJECTOR_DMC_DOCKER_IMAGE} --network host -f build/Dockerfile.injector-dmc .

deploy: ## Deploy webhook into K8s
	kubectl apply -f deployment/deployment.yaml
	kubectl apply -f deployment/service.yaml
	kubectl apply -f deployment/mutatingwebhook.yaml

deploy-eks: ## Deploy webhook into EKS
	kubectl apply -f deployment/deployment-eks.yaml
	kubectl apply -f deployment/service.yaml
	kubectl apply -f deployment/mutatingwebhook-eks.yaml

deploy-aks: ## Deploy webhook into AKS
	kubectl apply -f deployment/deployment-aks.yaml
	kubectl apply -f deployment/service.yaml
	kubectl apply -f deployment/mutatingwebhook-aks.yaml

deploy-gks: ## Deploy webhook into GKS
	kubectl apply -f deployment/deployment-gks.yaml
	kubectl apply -f deployment/service.yaml
	kubectl apply -f deployment/mutatingwebhook-gks.yaml

undeploy: ## Undeploy webhook from K8s
	kubectl delete -f deployment/deployment.yaml
	kubectl delete -f deployment/service.yaml
	kubectl delete -f deployment/mutatingwebhook.yaml

undeploy-eks: ## Undeploy webhook from EKS
	kubectl delete -f deployment/deployment-eks.yaml
	kubectl delete -f deployment/service.yaml
	kubectl delete -f deployment/mutatingwebhook-eks.yaml

undeploy-aks: ## Undeploy webhook from AKS
	kubectl delete -f deployment/deployment-aks.yaml
	kubectl delete -f deployment/service.yaml
	kubectl delete -f deployment/mutatingwebhook-aks.yaml

undeploy-gks: ## Undeploy webhook from GKS
	kubectl delete -f deployment/deployment-gks.yaml
	kubectl delete -f deployment/service.yaml
	kubectl delete -f deployment/mutatingwebhook-gks.yaml

.PHONY: build-webhook build-secret-injector build-app-launcher docker-webhook docker-secret-injector-oauth docker-secret-injector-dmc deploy deploy-eks deploy-aks deploy-gks