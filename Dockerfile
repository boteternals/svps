# STAGE 1: Kompilasi Engine menggunakan Golang Alpine
FROM golang:1.21-alpine AS builder

# Set direktori kerja
WORKDIR /app

# 1. Salin seluruh file proyek ke dalam container (termasuk folder core)
COPY . .

# 2. Jalankan mod tidy untuk mendeteksi import di core/main.go secara otomatis
# Dan paksa download dependensi utama untuk memastikan keberadaannya
RUN go mod tidy && \
    go mod download github.com/creack/pty && \
    go mod download github.com/gorilla/websocket

# 3. Kompilasi program menjadi file biner statis
RUN CGO_ENABLED=0 GOOS=linux go build -o /svps-server core/main.go

# STAGE 2: Runtime Environment menggunakan Ubuntu
FROM ubuntu:22.04

# Hindari interaksi saat instalasi package
ENV DEBIAN_FRONTEND=noninteractive

# Update dan install tool esensial yang membuat SVPS terasa seperti VPS asli
RUN apt-get update && apt-get install -y \
    bash \
    curl \
    git \
    vim \
    htop \
    wget \
    sudo \
    net-tools \
    iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# Salin file biner dari builder ke image akhir
COPY --from=builder /svps-server /usr/local/bin/svps-server
RUN chmod +x /usr/local/bin/svps-server

# Pengaturan Terminal agar mendukung warna dan shell default
ENV TERM=xterm-256color
ENV SHELL=/bin/bash

# Set folder kerja default saat user masuk
WORKDIR /root

# Zeabur menggunakan port dinamis, kita expose 8080 sebagai standar
EXPOSE 8080

# Jalankan SVPS Engine
CMD ["svps-server"]

