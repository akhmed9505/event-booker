# Этап сборки
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

# Копируем только модули для кэширования
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Копируем код
COPY . . 


# Статическая сборка минимального бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main cmd/main.go

# Финальный минимальный образ
FROM alpine:latest

WORKDIR /app

# Копируем бинарник из builder-этапа
COPY --from=builder /app/main .

# Копируем папку с шаблонами
COPY --from=builder /app/templates ./templates

# Запуск приложения
CMD ["./main"]
