# manager/Dockerfile
FROM golang:1.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN env CGO_ENABLED=0 GOOS=linux go build -o manager

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/manager .
EXPOSE 8080
ENTRYPOINT ["./manager"]
