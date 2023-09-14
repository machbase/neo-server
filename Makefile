.PHONY: all test test-base test-all package package-all arm32package-machbase-neo

all:
	@go run mage.go machbase-neo neow neoshell

cleanpackage:
	@rm -rf packages/*

tmpdir:
	@mkdir -p tmp

test:
	@go run mage.go test

test-all:
	@go run mage.go test

package:
	@go run mage.go package

package-all:
	@go run mage.go package

docker-image:
	docker build -t machbase-neo --file ./Dockerfile .

docker-run:
	docker run -d --name neo -p 5652-5656:5652-5656 machbase-neo

arm32package-machbase-neo:
	@go run mage.go buildx machbase-neo linux arm packagex machbase-neo linux arm

package-%:
	@go run mage.go machbase-neo package

%:
	@go run mage.go $@

generate:
	@go run mage.go generate
