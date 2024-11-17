all: help

# Default package
PKG := ./...

ROOT_DIR = $(shell pwd)

.PHONY: build
build: ## Run build
	@echo "==> Running build"
	@go build -v -o $(ROOT_DIR)/bin/gosible github.com/scylladb/gosible/cmd/gosible
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/remote/gosible_client github.com/scylladb/gosible/remote/main
	@./tools/pack-py-runtime.sh

.PHONY: unit-test
unit-test: ## Run unit tests
	@echo "==> Running tests"
	@go test -cover -race -count=1 $(PKG)

#TODO: add golangci-lint support
.PHONY: check-lint
check-lint: ## Run lint checks
	@echo "==> Running lint checks"
	@$(ROOT_DIR)/gofmtcheck.sh

.PHONY: e2e-build-ansible
e2e-build-ansible:
	@echo "==> Running e2e build ansible"
	docker build -f $(ROOT_DIR)/e2e/assets/dockerfiles/ansible.dockerfile -t "gosible/ansible:latest" $(ROOT_DIR)

.PHONY: e2e-build-gosible
e2e-build-gosible:
	@echo "==> Running e2e build gosible"
	docker build -f $(ROOT_DIR)/e2e/assets/dockerfiles/gosible.dockerfile -t "gosible/gosible:latest" $(ROOT_DIR)

.PHONY: e2e-build-ubuntu
e2e-build-ubuntu:
	@echo "==> Running e2e build ubuntu"
	docker build -f $(ROOT_DIR)/e2e/assets/dockerfiles/os/ubuntu.dockerfile -t "gosible/ubuntu:latest" $(ROOT_DIR)

.PHONY: e2e-build-diff
e2e-build-diff:
	@echo "==> Running e2e build diff"
	docker build -f $(ROOT_DIR)/e2e/assets/dockerfiles/diff.dockerfile -t "gosible/diff:latest" $(ROOT_DIR)

.PHONY: e2e-build
e2e-build: ## Build all e2e images
e2e-build: e2e-build-ansible e2e-build-gosible e2e-build-ubuntu e2e-build-diff

.PHONY: e2e-test
e2e-test: ## Run e2e tests. Use only=REGEXP to select test to run by test name with regexp
e2e-test: e2e-build
	@echo "==> Running e2e tests"
	@go test -v ./e2e/ -e2e -only "$(only)"

.PHONY: help
help:
	@awk -F ':|##' '/^[^\t].+?:.*?##/ {printf "\033[36m%-25s\033[0m %s\n", $$1, $$NF}' $(MAKEFILE_LIST)
