# Build Stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.sum && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /svps-server core/main.go

# Runtime Stage
FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies agresif
RUN apt-get update && apt-get install -y \
    bash curl git vim htop wget sudo net-tools iputils-ping \
    python3 python3-pip nano \
    && rm -rf /var/lib/apt/lists/*

# Copy Binary dari Builder
COPY --from=builder /svps-server /usr/local/bin/svps-server
RUN chmod +x /usr/local/bin/svps-server

# Environment Setup
ENV TERM=xterm-256color
ENV SHELL=/bin/bash
WORKDIR /root

# Expose & Run
EXPOSE 8080
CMD ["svps-server"]
/root

EXPOSE 8080
CMD ["svps-server"]

CMD ["svps-server"]

