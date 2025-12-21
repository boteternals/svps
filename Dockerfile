# STAGE 1: Kompilasi Engine
FROM golang:1.21-alpine AS builder
WORKDIR /app

# Copy modul dulu
COPY go.mod ./
# Jika ada go.sum, copy juga. Jika tidak, tidak apa-apa.
COPY go.sum* ./

# Paksa update modul dan buat go.sum yang valid di dalam container
RUN go mod tidy

# Copy sisanya
COPY . .

# Build dengan flag -installsuffix cgo agar benar-benar statis
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /svps-server core/main.go

# STAGE 2: OS Ubuntu
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    bash \
    curl \
    git \
    vim \
    htop \
    wget \
    sudo \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /svps-server /usr/local/bin/svps-server
RUN chmod +x /usr/local/bin/svps-server

ENV TERM=xterm-256color
ENV SHELL=/bin/bash

WORKDIR /root

EXPOSE 8080
CMD ["svps-server"]

