# Используем официальный образ Go 1.23 (не rc)
FROM golang:1.23

# Устанавливаем зависимости через apt (для Debian)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    git \
    bash \
    ca-certificates \
    curl && \
    rm -rf /var/lib/apt/lists/*

# Настраиваем окружение
ENV GOTOOLCHAIN=local
WORKDIR /app

# Копируем зависимости и скачиваем модули
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Устанавливаем тестовые зависимости
RUN go get github.com/stretchr/testify@latest

# Команда для запуска тестов
CMD ["bash", "-c", "cd /app && go test -v ./tests/integration/... -tags=integration"]
