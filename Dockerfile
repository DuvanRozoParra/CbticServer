# Etapa de construcción
FROM golang:1.22 AS builder

WORKDIR /app

# Usa el repositorio remoto o local (preferiblemente remoto si quieres actualización automática)
RUN git clone https://github.com/usuario/mi-servidor.git .
RUN go build -o servidor

# Etapa final
FROM debian:bullseye-slim

WORKDIR /app

# Copiar binario compilado y script de inicio
COPY --from=builder /app/servidor /app/servidor
COPY --from=builder /app /src
COPY start.sh /start.sh

RUN apt-get update && apt-get install -y git curl && chmod +x /start.sh

CMD ["/start.sh"]
