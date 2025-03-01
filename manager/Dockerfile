# manager/Dockerfile
FROM golang:1.23 AS builder
WORKDIR /app

# Установим ssh-клиент
RUN apt-get update && apt-get install -y --no-install-recommends openssh-client

# Создадим папку ~/.ssh
RUN mkdir -p /root/.ssh

# Добавим публичный ключ GitHub в known_hosts,
# чтобы SSH не ругался "Host key verification failed"
RUN ssh-keyscan github.com >> /root/.ssh/known_hosts

RUN --mount=type=ssh \
    git config --global url."git@github.com:ykhdr/crack-hash".insteadOf "https://github.com/ykhdr/crack-hash" && \
    go env -w GOPRIVATE=github.com/ykhdr/crack-hash && \
    go env -w GONOSUMDB=github.com && \
    go env -w GONOPROXY=github.com

COPY go.mod go.sum ./
RUN --mount=type=ssh go mod download

COPY . .
RUN --mount=type=ssh env CGO_ENABLED=0 GOOS=linux go build -o manager

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/manager .
EXPOSE 8080
ENTRYPOINT ["./manager"]
