# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /sealed-secret-rotator ./cmd/sealed-secrets-rotator.go

# Final stage
FROM alpine:3.18

# Install dependencies
RUN apk add --no-cache curl tar && \
    # Install kubectl
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    # Install kubeseal
    curl -L https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.23.1/kubeseal-0.23.1-linux-amd64.tar.gz | tar xz && \
    install -o root -g root -m 0755 kubeseal /usr/local/bin/kubeseal && \
    # Cleanup
    rm -f kubectl kubeseal *.tar.gz && \
    apk del curl tar

# Copy binary from builder
COPY --from=builder /sealed-secret-rotator /usr/local/bin/sealed-secret-rotator
COPY assets /assets

# Ensure the binary is executable
RUN chmod +x /usr/local/bin/sealed-secret-rotator

ENTRYPOINT ["sealed-secret-rotator"]