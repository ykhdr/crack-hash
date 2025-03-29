# manager/Dockerfile
FROM golang:1.24.1-alpine AS builder
WORKDIR /app

COPY ./common ./common
COPY ./worker ./worker

WORKDIR /app/worker

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o worker


FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/worker/worker .
COPY --from=builder /app/worker/config.kdl .

ENTRYPOINT ["/app/worker"]
