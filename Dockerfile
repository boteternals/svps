# STAGE 1: Kompilasi Engine
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
# Menghapus cache go.sum lama dan membangun ulang sesuai lingkungan Zeabur
RUN rm -f go.sum && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /svps-server core/main.go

# STAGE 2: Runtime Environment (Ubuntu)
FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y \
    bash curl git vim htop wget sudo net-tools iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# Ambil biner dari stage builder
COPY --from=builder /svps-server /usr/local/bin/svps-server
RUN chmod +x /usr/local/bin/svps-server

# Konfigurasi Terminal
ENV TERM=xterm-256color
ENV SHELL=/bin/bash
WORKDIR /root

# Jalankan SVPS
EXPOSE 8080
CMD ["svps-server"]

