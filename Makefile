.PHONY: all test

targets := $(shell ls main)
uname_s := $(shell uname -s)
uname_p := $(shell uname -p)
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
ifeq ($(uname_p),$(filter $(uname_p), aarch64 arm))
	@go test $(ARGS) -tags linux,arm64,edge_edition ./test/
endif
ifeq ($(uname_p),x86_64)
	@go test $(ARGS) -tags linux,amd64,edge_edition ./test/
endif
endif
ifeq ($(uname_s),Darwin)
ifeq ($(uname_p),$(filter $(uname_p), aarch64 arm))
	@go test $(ARGS) -tags darwin,arm64,edge_edition ./test/
endif
ifeq ($(uname_p),i386)
	@go test $(ARGS) -tags darwin,amd64,edge_edition ./test/
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
	@echo "package" $(uname_s) $(uname_p)
ifeq ($(uname_s),Linux)
ifeq ($(uname_p),$(filter $(uname_p), aarch64 arm))
	@./scripts/package.sh $*  linux  arm64 $(nextver) edge
	@./scripts/package.sh $*  linux  arm64 $(nextver) fog
endif
ifeq ($(uname_p),x86_64)
	@./scripts/package.sh $*  linux  amd64 $(nextver) edge
	@./scripts/package.sh $*  linux  amd64 $(nextver) fog
endif
endif
ifeq ($(uname_s),Darwin)
ifeq ($(uname_p),$(filter $(uname_p), aarch64 arm))
	@./scripts/package.sh $*  darwin  arm64 $(nextver) edge
	@./scripts/package.sh $*  darwin  arm64 $(nextver) fog
endif
ifeq ($(uname_p),i386)
	@./scripts/package.sh $*  darwin  amd64 $(nextver) edge
	@./scripts/package.sh $*  darwin  amd64 $(nextver) fog
endif
endif

%:
	@./scripts/build.sh $@ $(nextver) $(EDITION)
