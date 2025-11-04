# ---- Build stage
FROM golang:1.24-alpine AS build
WORKDIR /app
RUN apk add --no-cache ca-certificates git wget
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ai-wrapper-service ./cmd/server

# Fetch grpc_health_probe (pick a recent version)
ARG GRPC_HEALTH_PROBE_VERSION=v0.4.28
RUN wget -O /grpc_health_probe \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
 && chmod +x /grpc_health_probe

# ---- Runtime (distroless)
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /app/ai-wrapper-service /ai-service
COPY --from=build /grpc_health_probe /bin/grpc_health_probe
USER nonroot:nonroot
EXPOSE 50051
ENTRYPOINT ["/ai-service"]
