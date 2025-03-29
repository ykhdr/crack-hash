# manager/Dockerfile
FROM golang:1.24.1-alpine AS builder
WORKDIR /app

COPY ./common ./common
COPY ./manager ./manager
COPY ./worker ./worker
COPY go.work go.work.sum ./

WORKDIR /app/manager

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager ./cmd/manager


FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/manager/bin/manager .
COPY --from=builder /app/manager/config/config.kdl .

EXPOSE 8080
ENTRYPOINT ["/app/manager"]
