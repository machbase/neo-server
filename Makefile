.PHONY: all test

targets := $(shell ls main)
uname_s := $(shell uname -s)
uname_p := $(shell uname -p)

all:
	@for tg in $(targets) ; do \
		make $$tg; \
	done

cleanpackage:
	@rm -rf packages/*

test:
	@go test $(ARGS) ./test/

test-all:
	@make -f Makefile ARGS="-cover -v -count 1" test

package:
	@./docker-package.sh machgo

package-all:
	@for tg in $(targets) ; do \
		make package-$$tg; \
	done

releases:
	@./docker-package.sh machgo linux amd64
	@./docker-package.sh machgo linux arm64/v7

package-%:
	@echo "package" $(uname_s) $(uname_p)
ifeq ($(uname_s),Linux)
ifeq ($(uname_p),aarch64)
	@./scripts/package.sh $*  linux    arm64
endif
ifeq ($(uname_p),x86_64)
	@./scripts/package.sh $*  linux    amd64
endif
endif

protos := $(basename $(shell cd proto && ls *.proto))

regen-all:
	@for tg in $(protos) ; do \
		make regen-$$tg; \
	done

regen-%:
	@./regen.sh $*

%:
	@./scripts/build.sh $@
