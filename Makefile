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
	@[ -d ./tmp ] || mkdir -p ./tmp
ifeq ($(uname_s), Linux)
	go test `go list ./... | grep -v main/neow` -cover -race -coverprofile ./tmp/cover.out
else
	go test ./... -cover -race -coverprofile ./tmp/cover.out
endif
	@go tool cover -func ./tmp/cover.out |grep total:

test-all:
	go test ./... -v -count 1 -cover -race

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
	go run mage.go buildx machbase-neo linux arm
#	./scripts/package.sh machbase-neo linux arm $(nextver)

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
