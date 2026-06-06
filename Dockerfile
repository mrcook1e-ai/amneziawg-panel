# syntax=docker/dockerfile:1.7

# --- frontend ---
FROM node:20-alpine AS frontend
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ .
RUN npm run build

# --- backend ---
FROM golang:1.22-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Place the freshly built frontend where the embed directive expects it.
COPY --from=frontend /web/dist ./internal/static/dist
RUN CGO_ENABLED=0 go build -tags embed -ldflags="-s -w" -o /out/awgpanel ./cmd/server

# --- runtime: AmneziaWG userspace base image gives us awg + awg-quick ---
FROM amneziavpn/amneziawg-go:latest
RUN apk add --no-cache iptables ca-certificates dumb-init
COPY --from=backend /out/awgpanel /usr/local/bin/awgpanel
RUN mkdir -p /etc/amnezia/amneziawg
EXPOSE 51820/udp 51821/tcp
ENTRYPOINT ["/usr/bin/dumb-init", "/usr/local/bin/awgpanel"]
