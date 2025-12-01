# ┌───────────────────────────┐
# │   MULTI STAGE BUILD       │
# └───────────────────────────┘

# Etapa 1: compilar
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copiar go.mod y go.sum primero (mejora cacheo de dependencias)
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código
COPY . .

# Compilar el binario (cmd/main.go)
RUN go build -o server ./cmd/main.go

# Etapa 2: imagen final mínima
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copiar binario desde la etapa builder
COPY --from=builder /app/server .

EXPOSE 8080

# Ejecutar binario
CMD ["./server"]
