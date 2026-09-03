# syntax=docker/dockerfile:1.7

# Pin AmneziaWG toolchain so we control which protocol features the host
# kernel understands. The upstream `amneziavpn/amneziawg-go:latest` on Docker
# Hub ships userspace from September 2021 — ancient, no AWG 1.5/2.0 support.
# We build both binaries from source against pinned refs and copy them into
# a slim Alpine runtime. Bump these when a new known-good release lands.
#
# AWG 3.1 line. v3 is wire-compatible downwards: with none of the 3.x device
# params set, the daemon behaves byte-for-byte like 2.0/1.0, so one binary
# serves awg1/awg2/awg31 profiles simultaneously. Tools v3.1 is required to
# `setconf` the new Interface keys — older tools abort on any unknown key.
ARG AWG_TOOLS_REF=v3.1.20260812
ARG AWG_GO_REF=v3.1.20260828

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

# --- amneziawg-go (userspace WG daemon, AWG 3.1-capable) ---
# v3.x declares `go 1.25.0` and moved to module path .../amneziawg-go/v3;
# the panel's own backend stays on 1.22 since our code is happy there.
# Two stages, two toolchains — by design.
FROM golang:1.25-alpine AS awggo
ARG AWG_GO_REF
RUN apk add --no-cache git
RUN git clone --depth=1 --branch ${AWG_GO_REF} \
    https://github.com/amnezia-vpn/amneziawg-go /src
WORKDIR /src
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/amneziawg-go .

# --- amneziawg-tools (awg, awg-quick) ---
# WITH_WGQUICK=yes gates the awg-quick script and its config dir; without it
# the Makefile installs only the `awg` binary and the runtime stage breaks.
FROM alpine:3.20 AS awgtools
ARG AWG_TOOLS_REF
RUN apk add --no-cache git make gcc musl-dev linux-headers bash
RUN git clone --depth=1 --branch ${AWG_TOOLS_REF} \
    https://github.com/amnezia-vpn/amneziawg-tools /src
WORKDIR /src/src
RUN make WITH_WGQUICK=yes && \
    make install WITH_WGQUICK=yes DESTDIR=/out PREFIX=/usr

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache iptables ca-certificates dumb-init bash openresolv iproute2
COPY --from=awggo    /out/amneziawg-go         /usr/bin/amneziawg-go
COPY --from=awgtools /out/usr/bin/awg          /usr/bin/awg
COPY --from=awgtools /out/usr/bin/awg-quick    /usr/bin/awg-quick
# awg-quick will spawn the Go daemon instead of expecting a kernel module —
# we don't ship/require the AWG kernel patch.
ENV WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go \
    WG_SUDO=1
COPY --from=backend /out/awgpanel /usr/local/bin/awgpanel
RUN mkdir -p /etc/amnezia/amneziawg
EXPOSE 51820/udp 51821/tcp
ENTRYPOINT ["/usr/bin/dumb-init", "/usr/local/bin/awgpanel"]
