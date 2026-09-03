# Repository Guide

## Architecture

- `cmd/server/main.go` is the composition root: it starts the AmneziaWG manager, SQLite-backed events/stats, billing loop, SSE broker, and HTTP router.
- `internal/awg` owns persisted VPN state and interface lifecycle; `internal/api` exposes it; `internal/db`, `internal/events`, `internal/stats`, and `internal/billing` share the SQLite database under `WG_PATH`.
- In code, `Subscriber` means the user/account shown as “Клиент”; historical `Client` means one device. The invariant is one `Client` -> one `Profile` -> one `awgN` interface and UDP port.
- The Vue app lives entirely under `web/`. API calls are centralized in `web/src/lib/api.ts`; admin state is mainly in Pinia stores; `/cabinet/:token` is public magic-link access.

## Commands

Run backend commands from the repository root:

```sh
go test ./internal/awg -run TestName -count=1  # focused test
go vet ./...                                   # CI step 1
go build -v ./...                              # CI step 2
go test ./... -count=1                         # CI step 3
```

Run frontend commands from `web/`:

```sh
npm ci
npm run dev      # Vite :5173; /api proxies to VITE_API_TARGET or localhost:51821
npm run build    # vue-tsc --noEmit, then Vite build to web/dist
```

- There is no frontend test, lint, or format script. `npm run build` is the frontend typecheck and build gate.
- Before handoff, mirror `.github/workflows/ci.yml`: `go vet ./...`, `go build -v ./...`, `go test ./... -count=1`, and `npm run build` in `web/`.

## Build and Runtime Traps

- A default Go build uses `internal/static/static.go`: `static.FS` is nil and the SPA is not served. Development therefore needs Vite separately.
- Production uses `-tags embed`; `internal/static/static_embed.go` embeds `internal/static/dist`, not `web/dist`. The Dockerfile is the canonical pipeline: build `web/dist`, copy it to `internal/static/dist`, then compile with `CGO_ENABLED=0 go build -tags embed`.
- Do not replace `awg`/`awg-quick` with standard WireGuard tools. The image pins and builds `amneziawg-go` and `amneziawg-tools`; the userspace daemon intentionally uses a newer Go toolchain than the panel backend (v3 declares `go 1.25.0` and moved to module path `.../amneziawg-go/v3`).
- A real server run is not a plain web-app smoke test: manager startup invokes AmneziaWG tooling and manages interfaces. Compose supplies `NET_ADMIN`, `/dev/net/tun`, IP-forwarding sysctls, the UDP port range, and persistent state.
- Leave `WG_EGRESS_IFACE` empty unless deployment requires an override. Runtime detection follows the container's default route; hardcoding `eth0` can produce successful handshakes with no client internet.

## State and Protocol Constraints

- `WG_PATH` contains JSON state, generated `.conf` files, and `panel.db`; the Compose volume and backup/restore cover them together. Avoid treating any one file as disposable in isolation.
- `internal/awg/config.go` uses `SchemaVersion = 5` with `MinSchemaVersion = 4`: state inside that window loads as-is and is rewritten at the current version on the next Save. Anything older fails fast (no in-place migration), and anything newer is refused too, so a binary downgrade cannot silently drop unknown fields and persist the loss. Any schema change must account for persisted deployments and restore behavior.
- Each profile consumes one interface and one UDP port. Defaults are `awg0` and ports `51820-51859`; keep interface allocation, port allocation, generated config, and Compose exposure aligned.
- Profiles carry one of three protocol generations, detected from their own markers by `Profile.Generation()` (the same order the official client uses): AWG 1.0, 2.0, or 3.1. The pinned `amneziawg-go` v3 serves all three from one binary — with the 3.x device params unset it is byte-for-byte a 2.0/1.0 device on the wire.
- AWG 3.x keys are emitted to `.conf` only when actually set, so 1.0/2.0 profiles render exactly as before. `TestRenderAWG2_GoldenUnchanged` is the regression gate for that; do not update its golden to make a change pass.
- `ParseObfuscation` accepts `Itime` and `J1-J3` and silently drops them — they belonged to the abandoned AWG 1.5 beta and exist in neither `amneziawg-go` v3 nor `amneziawg-tools` v3.1. Never emit them: the tools abort the whole interface on any unrecognised `[Interface]` key.
- With `HeaderProtectionKey` set, all of `S1`-`S4` must be >= 12. `amneziawg-go` reads the ChaCha20 nonce from the first 12 bytes of the S prefix and rejects the UAPI set otherwise, leaving the interface down.
- `H1`-`H4` accept both a fixed value and a `min-max` range; `amneziawg-tools` `u32_range_from_string` treats a bare integer as the degenerate range. AWG 1.0 and the official AWG 3.1 defaults both use fixed values.
- YooKassa checkout is enabled only when `YOOKASSA_SHOP_ID`, `YOOKASSA_SECRET_KEY`, and `PUBLIC_URL` are all set.

## Source of Truth

- Prefer executable config over prose: `.github/workflows/ci.yml` for verification, `Dockerfile` for production builds, `docker-compose.yml`, `deploy/dokploy/` template artifacts (`template.toml`, `import.base64`, `generate.mjs`), plus `internal/config/config.go` for runtime defaults.
- UDP port allocation is configured via `WG_PORT_RANGE_START` and `WG_PORT_RANGE_END` (defaulting to ports 51820–51859 for profiles `awg0`..`awg39`). Single-port allocation is unsupported and must not be reintroduced.
- `docs/STATE.md` contains valuable terminology and state-model context, but also deployment history and TODOs that may age; verify claims in current source before relying on them.
