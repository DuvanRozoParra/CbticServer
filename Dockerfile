# ┌───────────────────────────┐
# │   SINGLE STAGE           │
# └───────────────────────────┘
FROM golang:1.23-alpine

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY . .

EXPOSE 8080

CMD ["go", "run", "main.go"]
