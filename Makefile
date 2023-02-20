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
	@go test $(ARGS) \
		./mods/server \
		./test

test-all:
ifeq ($(uname_s),Linux)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm64))
	make -f Makefile ARGS="-cover -v -count 1 -tags=fog_edition" test
endif
ifeq ($(uname_m),$(filter $(uname_m), arm armv6l armv7l))
	make -f Makefile ARGS="-cover -v -count 1 -tags=edge_edition" test
endif
ifeq ($(uname_m),x86_64)
	make -f Makefile ARGS="-cover -v -count 1 -tags=fog_edition" test
endif
endif
ifeq ($(uname_s),Darwin)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm arm64))
	make -f Makefile ARGS="-cover -v -count 1 -tags=fog_edition" test
endif
ifeq ($(uname_m),x86_64)
	make -f Makefile ARGS="-cover -v -count 1 -tags=fog_edition" test
endif
endif

package:
	@make -f Makefile package-machbase-neo

package-all:
	@for tg in $(targets) ; do \
		make package-$$tg; \
	done

package-%:
	@echo "package" $(uname_s) $(uname_m)
ifeq ($(uname_s),Linux)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm64))
	./scripts/package.sh $*  linux  arm64 $(nextver) edge && \
	./scripts/package.sh $*  linux  arm64 $(nextver) fog
endif
ifeq ($(uname_m),$(filter $(uname_m), arm armv6l armv7l))
	./scripts/package.sh $*  linux  arm $(nextver) edge
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
ifeq ($(uname_m),x86_64)
	./scripts/package.sh $*  darwin  amd64 $(nextver) edge && \
	./scripts/package.sh $*  darwin  amd64 $(nextver) fog
endif
endif

%:
ifeq ($(uname_m),$(filter $(uname_m), arm armv6l armv7l))
	@./scripts/build.sh $@ $(nextver) edge
else
	@./scripts/build.sh $@ $(nextver) fog
endif
