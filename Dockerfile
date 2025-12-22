
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.sum && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /svps-server core/main.go

FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y \
    bash curl git vim htop wget sudo net-tools iputils-ping \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /svps-server /usr/local/bin/svps-server
RUN chmod +x /usr/local/bin/svps-server

ENV TERM=xterm-256color
ENV SHELL=/bin/bash
WORKDIR /root

EXPOSE 8080
CMD ["svps-server"]

r /svps-server /usr/local/bin/svps-server
RUN chmod +x /usr/local/bin/svps-server

# Konfigurasi Terminal
ENV TERM=xterm-256color
ENV SHELL=/bin/bash
WORKDIR /root

# Jalankan SVPS
EXPOSE 8080
CMD ["svps-server"]

