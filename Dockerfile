# syntax=docker/dockerfile:1

# Build frontend
FROM node:18-alpine AS frontend_builder
WORKDIR /app
COPY ./web/ /app
RUN yarn install --frozen-lockfile && yarn build

# Build backend
FROM golang:1.18 AS backend_builder
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends libpcap-dev && rm -rf /var/lib/apt/lists/*
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ ./
COPY --from=frontend_builder /app/dist ./web/dist
RUN go build -o ./bin/pkappa2 ./cmd/pkappa2/main.go

# Run
FROM debian:latest
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends libpcap0.8 && rm -rf /var/lib/apt/lists/*

COPY --from=backend_builder /app/bin/pkappa2 ./pkappa2
COPY --from=backend_builder /app/web/dist ./web/dist

RUN adduser pkappa2
USER pkappa2

EXPOSE 8080
VOLUME /data
VOLUME /app/converters

ENV PKAPPA2_USER_PASSWORD ""
ENV PKAPPA2_PCAP_PASSWORD ""

CMD /app/pkappa2 -base_dir /data -address :8080 -user_password "${PKAPPA2_USER_PASSWORD}" -pcap_password "${PKAPPA2_PCAP_PASSWORD}"
