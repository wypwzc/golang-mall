# 有 make 的环境用这个；Windows 默认没有 make，请用 scripts/tasks.sh

SERVICES := gateway user product cart order payment
GO       ?= go

.PHONY: help tidy fmt vet build test test-race infra infra-down monitoring ps clean

help:
	@echo "make tidy | fmt | vet | build | test | test-race | infra | infra-down | monitoring | ps | clean"

tidy:
	$(GO) mod tidy

fmt:
	gofmt -l -w .

vet:
	$(GO) vet ./...

build:
	@mkdir -p bin
	@for s in $(SERVICES); do echo "==> building $$s"; $(GO) build -o bin/$$s ./services/$$s/cmd/server; done

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

infra:
	docker compose up -d mysql redis
	docker compose ps

infra-down:
	docker compose down

monitoring:
	docker compose --profile monitoring up -d prometheus

ps:
	docker compose ps

clean:
	rm -rf bin
