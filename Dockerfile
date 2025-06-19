# ┌───────────────────────────┐
# │   1) BUILD STAGE         │
# └───────────────────────────┘
FROM golang:1.23-alpine AS builder

# Necesitamos git + certificados TLS para poder hacer git pull en runtime
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copiamos TODO tu proyecto (incluido .git)
COPY . .

# Compilamos el binario desde cmd
RUN go build -o server ./cmd


# ┌───────────────────────────┐
# │   2) RUNTIME STAGE       │
# └───────────────────────────┘
FROM golang:1.23-alpine

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Traemos el código + el binario ya compilado
COPY --from=builder /app /app

# Script de arranque
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

EXPOSE 8080

ENTRYPOINT ["./start.sh"]
