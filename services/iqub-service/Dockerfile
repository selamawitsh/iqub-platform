# The Go version here must be at least the "go" line in go.mod (1.25.0).
# The official golang images do not download newer toolchains on their own.
FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/iqub-service ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/iqub-service /app/iqub-service

USER app
EXPOSE 8080
ENTRYPOINT ["/app/iqub-service"]
