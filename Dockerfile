FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o telegram-mcp .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/telegram-mcp .

EXPOSE 8080

ENTRYPOINT ["/app/telegram-mcp"]
