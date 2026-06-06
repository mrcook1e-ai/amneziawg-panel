FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/awgpanel ./cmd/server

FROM amneziavpn/amneziawg-go:latest
RUN apk add --no-cache iptables ca-certificates
COPY --from=build /out/awgpanel /usr/local/bin/awgpanel
RUN mkdir -p /etc/amnezia/amneziawg
EXPOSE 51820/udp 51821/tcp
ENTRYPOINT ["/usr/local/bin/awgpanel"]
