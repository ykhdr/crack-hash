# worker/Dockerfile
FROM golang:1.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN env CGO_ENABLED=0 GOOS=linux go build -o worker

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/worker .
EXPOSE 8081
ENTRYPOINT ["./worker"]
