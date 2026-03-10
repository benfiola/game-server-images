GORELEASER_VERSION ?= 2.12.7
SVU_VERSION ?= 3.3.0

BIN ?= ./.bin
DIST ?= ./.dist

arch := $(shell uname -m)
ifeq ($(arch),aarch64)
    arch := arm64
else
    arch := amd64
endif
bin := $(abspath $(BIN))
dist := $(abspath $(DIST))
project := $(abspath $(dir $(MAKEFILE_LIST)))

include Makefile.include.mk

.PHONY: default
default: list-targets

list-targets:
	@echo "available targets:"
	@LC_ALL=C $(MAKE) -pRrq -f $(firstword $(MAKEFILE_LIST)) : 2>/dev/null \
		| awk -v RS= -F: '/(^|\n)# Files(\n|$$$$)/,/(^|\n)# Finished Make data base/ {if ($$$$1 !~ "^[#.]") {print $$$$1}}' \
		| sort \
		| grep -E -v -e '^[^[:alnum:]]' -e '^$$@$$$$' \
		| sed 's/^/\t/'

.PHONY: build-docs
build-docs: install-docs
	# build docs
	cd docs && npm run build -- --out-dir=$(dist)

.PHONY: dev-docs
dev-docs: install-docs
	# run docs dev server
	cd docs && npm run start

.PHONY: install-docs
install-docs:
	# install doc dependencies
	cd docs && npm install

.PHONY: install-project
install-project:
	# install project dependencies
	go mod download

.PHONY: install-tools
install-tools:

$(eval $(call tool-from-apt,bsdtar,libarchive-tools))
$(eval $(call tool-from-apt,curl,curl))

goreleaser_arch := $(arch)
ifeq ($(goreleaser_arch),amd64)
	goreleaser_arch := x86_64
endif
goreleaser_url := https://github.com/goreleaser/goreleaser/releases/download/v$(GORELEASER_VERSION)/goreleaser_Linux_$(goreleaser_arch).tar.gz
$(eval $(call tool-from-tar-gz,goreleaser,$(goreleaser_url),0))

svu_url := https://github.com/caarlos0/svu/releases/download/v$(SVU_VERSION)/svu_$(SVU_VERSION)_linux_$(arch).tar.gz
$(eval $(call tool-from-tar-gz,svu,$(svu_url),0))

.PHONY: generate
generate: generate__goreleaser

.PHONY: generate__goreleaser
generate__goreleaser:
	go run $(project)/hack/generate-goreleaser