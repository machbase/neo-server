.PHONY: all test test-all package package-all

targets := $(shell ls main)
uname_s := $(shell uname -s)
uname_m := $(shell uname -m)
nextver := $(shell ./scripts/buildversion.sh)

all:
	@for tg in $(targets) ; do \
		make $$tg; \
	done

cleanpackage:
	@rm -rf packages/*

tmpdir:
	@mkdir -p tmp

test: tmpdir
ifeq ($(uname_s),Linux)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm arm64))
	@go test $(ARGS) -tags=edge_edition ./test/
endif
ifeq ($(uname_m),x86_64)
	@go test $(ARGS) -tags=edge_edition ./test/
endif
endif
ifeq ($(uname_s),Darwin)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm arm64))
	go test $(ARGS) -tags=edge_edition ./test/
endif
ifeq ($(uname_m),i386)
	@go test $(ARGS) -tags=edge_edition ./test/
endif
endif

test-all:
	@make -f Makefile ARGS="-cover -v -count 1" test

package:
	@make -f Makefile package-machbase-neo

package-all:
	@for tg in $(targets) ; do \
		make package-$$tg; \
	done

package-%:
	@echo "package" $(uname_s) $(uname_m)
ifeq ($(uname_s),Linux)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm arm64))
	./scripts/package.sh $*  linux  arm64 $(nextver) edge && \
	./scripts/package.sh $*  linux  arm64 $(nextver) fog
endif
ifeq ($(uname_m),x86_64)
	./scripts/package.sh $*  linux  amd64 $(nextver) edge && \
	./scripts/package.sh $*  linux  amd64 $(nextver) fog
endif
endif
ifeq ($(uname_s),Darwin)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm arm64))
	./scripts/package.sh $*  darwin  arm64 $(nextver) edge && \
	./scripts/package.sh $*  darwin  arm64 $(nextver) fog
endif
ifeq ($(uname_m),i386)
	./scripts/package.sh $*  darwin  amd64 $(nextver) edge && \
	./scripts/package.sh $*  darwin  amd64 $(nextver) fog
endif
endif

%:
	@./scripts/build.sh $@ $(nextver) $(EDITION)
