FROM golang:1.23.2-alpine3.20 AS builder

# Set Go env
ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /go/src/serveralpha

# Install dependencies
RUN apk --update --no-cache add ca-certificates make

# Download grpc_health_probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.26 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

# Build Go binary
COPY Makefile go.mod go.sum ./
RUN go mod download
COPY . .
RUN make tidy

# Skaffold passes in debug-oriented compiler flags
ARG SKAFFOLD_GO_GCFLAGS
RUN go build -gcflags="${SKAFFOLD_GO_GCFLAGS}" -o /go/src/serveralpha/image-service cmd/main.go

# Deployment container
FROM scratch

# Definition of this variable is used by 'skaffold debug' to identify a golang binary.
# Default behavior - a failure prints a stack trace for the current goroutine.
# See https://golang.org/pkg/runtime/
ENV GOTRACEBACK=single

COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /bin/grpc_health_probe /bin/
COPY --from=builder /go/src/serveralpha/image-service /serveralpha/image-service

ENTRYPOINT ["/serveralpha/image-service"]