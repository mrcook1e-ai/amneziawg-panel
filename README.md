# amneziawg-panel

Go-бэкенд + Vue-фронтенд для управления AmneziaWG. Скелет, не для прода.

## Структура

```
cmd/server/main.go        — entrypoint
internal/config           — env-конфиг
internal/awg              — ядро: модель, парсер/рендерер .conf, exec-обёртки, IPAM
internal/api              — HTTP-роутер, session-cookie auth, handlers
web/                      — фронт (Vue 3 + Vite), пока пусто
```

## Сборка

```sh
go mod tidy
CGO_ENABLED=0 go build ./cmd/server
```

## Env

| Переменная | По умолчанию |
|---|---|
| `WG_HOST` | (обязательно) внешний адрес сервера |
| `WG_PORT` | 51820 |
| `WG_INTERFACE` | awg0 |
| `WG_PATH` | /etc/amnezia/amneziawg |
| `WG_DEFAULT_ADDRESS` | 10.8.0.x |
| `WG_DEFAULT_DNS` | 1.1.1.1 |
| `WG_ALLOWED_IPS` | 0.0.0.0/0, ::/0 |
| `WG_MTU` | 0 (не задавать) |
| `WG_PERSISTENT_KEEPALIVE` | 0 |
| `PORT` | 51821 |
| `WEBUI_HOST` | 0.0.0.0 |
| `PASSWORD` | (пусто = без auth) |
| `AWG_BIN` | awg |
| `AWG_QUICK_BIN` | awg-quick |
| `JC, JMIN, JMAX, S1, S2, H1..H4` | дефолты обфускации |

## API

Совместим с фронтом `amnezia-wg-easy`:

```
GET    /api/session
POST   /api/session                          { password }
DELETE /api/session

GET    /api/wireguard/client/
POST   /api/wireguard/client/                { name }
DELETE /api/wireguard/client/{id}
POST   /api/wireguard/client/{id}/enable
POST   /api/wireguard/client/{id}/disable
PUT    /api/wireguard/client/{id}/name       { name }
PUT    /api/wireguard/client/{id}/address    { address }
GET    /api/wireguard/client/{id}/configuration
GET    /api/wireguard/client/{id}/qrcode.svg   (PNG, не SVG)
```

## TODO

- [ ] Vue 3 + Vite фронт, встроить через `go:embed`
- [ ] SQLite-store вместо JSON
- [ ] WebSocket для live-статуса (сейчас опрос на каждый list)
- [ ] bcrypt-хеш пароля, rate limiting на /api/session
- [ ] IPv6 / поддержка нескольких интерфейсов
- [ ] iptables MASQUERADE/FORWARD (сейчас полагаемся на `PostUp`)
- [ ] метрики, история трафика
