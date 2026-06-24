.PHONY: help build run test race bench fmt vet clean docker

BINARY := server
DOCKER_IMAGE := cbticserver

help: ## Mostrar ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Compilar binario
	go build -o $(BINARY) ./cmd/main.go

run: ## Ejecutar servidor
	go run ./cmd/main.go

test: ## Tests sin race
	go test -timeout 30s ./...

race: ## Tests con race detector
	go test -race -timeout 60s ./...

bench: ## Benchmark sintético (N=100, 60Hz, 10s)
	go run ./cmd/benchtest -clients 100 -rate 60 -duration 10s

fmt: ## Formatear código
	gofmt -w .

vet: ## go vet
	go vet ./...

clean: ## Limpiar binarios
	rm -f $(BINARY) /tmp/cbtic-race

docker: ## Build imagen Docker
	docker build -t $(DOCKER_IMAGE) .

verify: fmt vet race ## fmt + vet + race (verificación completa)
