ARG GO_VERSION=1.23

# ---- build stage ----
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# Copy dependency files first so this layer is cached until they change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/member-service ./cmd/server

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/member-service /member-service
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/member-service"]
