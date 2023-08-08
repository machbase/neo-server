.PHONY: all test test-base test-all package package-all arm32package-machbase-neo

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

test:
	@make -f Makefile test-base

test-base: tmpdir
	@go test $(ARGS) -cover -race \
		./booter \
		./mods/util \
		./mods/util/glob \
		./mods/util/ini \
		./mods/util/ssfs \
		./mods/expression \
		./mods/tql \
		./mods/nums/opensimplex \
		./mods/script \
		./mods/transcoder \
		./mods/codec/internal/json \
		./main/machbase-neo \
		./mods/do \
		./mods/service/security \
		./mods/service/mqttd/mqtt \
		./mods/service/httpd \
		./mods/server \
		./mods/shell \
		./test

test-all:
	make -f Makefile ARGS="-v -count 1" test-base

package:
	@make -f Makefile package-machbase-neo

package-all:
	@for tg in $(targets) ; do \
		make package-$$tg; \
	done

docker-image:
	docker build -t machbase-neo --file ./scripts/build-dockerfile .

docker-run:
	docker run -d --name neo -p 5652-5656:5652-5656 machbase-neo

arm32package-machbase-neo:
	@echo "package arm 32bit linux"
	./scripts/package.sh machbase-neo linux arm $(nextver)

package-%:
	@echo "package" $(uname_s) $(uname_m)
ifeq ($(uname_s),Linux)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm64))
	./scripts/package.sh $*  linux  arm64 $(nextver)
endif
ifeq ($(uname_m),$(filter $(uname_m), arm armv6l armv7l))
	./scripts/package.sh $*  linux  arm $(nextver)
endif
ifeq ($(uname_m),x86_64)
	./scripts/package.sh $*  linux  amd64 $(nextver)
endif
endif
ifeq ($(uname_s),Darwin)
ifeq ($(uname_m),$(filter $(uname_m), aarch64 arm arm64))
	./scripts/package.sh $*  darwin  arm64 $(nextver)
endif
ifeq ($(uname_m),x86_64)
	./scripts/package.sh $*  darwin  amd64 $(nextver)
endif
endif

%:
	@./scripts/build.sh $@ $(nextver)

release-%:
	@echo "release" $*
	./scripts/package.sh $* linux amd64 $(nextver)
	./scripts/package.sh $* linux arm64 $(nextver)
	./scripts/package.sh $* darwin arm64 $(nextver)
	./scripts/package.sh $* darwin amd64 $(nextver)
	./scripts/package.sh $* windows amd64 $(nextver)

generate:
	@go generate ./...

## Require https://github.com/matryer/moq
regen-mock:
	moq -out ./mods/util/mock/database.go -pkg mock ../neo-spi Database
	moq -out ./mods/util/mock/server.go -pkg mock   ../neo-spi DatabaseServer
	moq -out ./mods/util/mock/client.go -pkg mock   ../neo-spi DatabaseClient
	moq -out ./mods/util/mock/auth.go -pkg mock     ../neo-spi DatabaseAuth
	moq -out ./mods/util/mock/result.go -pkg mock   ../neo-spi Result
	moq -out ./mods/util/mock/rows.go -pkg mock     ../neo-spi Rows
	moq -out ./mods/util/mock/row.go -pkg mock      ../neo-spi Row
	moq -out ./mods/util/mock/appender.go -pkg mock ../neo-spi Appender
