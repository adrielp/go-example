include ./Makefile.Common

BUILD_DIR ?= $(SRC_ROOT)/build
OS := $(shell uname | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
BUILD_IN_TILT := false

CHECKS = generate lint test tidy fmt

# set ARCH var based on output
ifeq ($(ARCH),x86_64)
	ARCH = amd64
endif
ifeq ($(ARCH),aarch64)
	ARCH = arm64
endif

# .PHONY: all
# # all: install-tools
# all: build

.DEFAULT_GOAL := build

.PHONY: build
build: install-tools
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(BUILD_DIR)/go-example

# TODO: fix this release through goreleaser. Goreleaser installed through tools.go
# is the OSS version and doesn't support the `partial:` option in the
# .goreleaser.yaml. This option is needed for CI builds but isn't available locally.
.PHONY: grbuild
grbuild:
	$(GORELEASER) build --clean --snapshot

.PHONY: dockerbuild
dockerbuild:
	$(MAKE) build OS=linux ARCH=$(ARCH)
	@if [ $(BUILD_IN_TILT) = "true" ]; then \
		docker build . -t localhost:55166/go-example:localdev \
		--build-arg BIN_PATH="./build/go-example" \
		--platform linux/$(ARCH); \
		docker push localhost:55166/go-example:localdev; \
	else \
		docker build . -t adrielp/go-example:localdev \
		--build-arg BIN_PATH="./build/go-example" \
		--platform linux/$(ARCH); \
	fi

# Setting the paralellism to 1 to improve output readability. Reevaluate later as needed for performance
.PHONY: checks
checks: install-tools
	$(MAKE) -j 1 $(CHECKS)
	@if [ -n "$$(git diff --name-only)" ]; then \
		echo "Some files have changed. Please commit them."; \
		exit 1; \
	else \
		echo "completed successfully."; \
	fi

# Load O11y Tilt Cluster w local build
.PHONY: tilt
tilt:
	KUBECONFIG=.kube/k3dconfig
	kubectl config current-context
	echo "Looking for otel-basic cluster..."; \
	cluster=$$(k3d cluster ls --no-headers otel-basic 2> /dev/null | awk '{print $$1}'); \
	if [[ "$$cluster" && $$cluster = "otel-basic" ]]; then \
    echo "otel-basic cluster present"; \
	k3d kubeconfig merge -s otel-basic -o .kube/k3dconfig --overwrite; \
   	else \
  		echo "not present... creating otel-basic cluster"; \
		k3d cluster create otel-basic --registry-create otel-basic-registry:localhost:55166 1> /dev/null; \
		k3d kubeconfig merge -s otel-basic -o .kube/k3dconfig --overwrite; \
   	fi; \
	kubectl config use-context k3d-otel-basic
	tilt up

.PHONY: run
run:
	$(MAKE) build
	./run.sh
