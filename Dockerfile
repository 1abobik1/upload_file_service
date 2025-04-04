# Билд стадия
FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Сборка 
RUN CGO_ENABLED=0 GOOS=linux go build -o /upload-service ./cmd/main.go

# Финальная стадия
FROM alpine:latest

WORKDIR /root/

# Копируем бинарник и конфиги
COPY --from=builder /upload-service .
COPY --from=builder /app/.env .

RUN apk --no-cache add tzdata ca-certificates

EXPOSE 50051

CMD ["./upload-service"]