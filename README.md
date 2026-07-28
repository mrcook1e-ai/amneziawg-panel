# Amnezia Panel

Самостоятельная веб-панель управления **AmneziaWG**: Go-бэкенд + Vue 3 фронт, единый бинарь, SQLite для метрик и журнала событий, realtime через SSE.

<p align="center">
  <img src="web/public/logo.png" width="120" alt="Amnezia Panel" />
</p>

## Возможности

- **Клиенты**: создание / удаление / переименование / включение-выключение / смена IP, заметки, срок действия (авто-отключение), per-client overrides (DNS / AllowedIPs / MTU).
- **Импорт по pubkey** — вернуть peer в панель, если конфиг уже выдан.
- **Backup / Restore** — `tar.gz` с JSON-состоянием, серверным `.conf` и БД метрик.
- **Realtime через SSE** — живая скорость в шапке (1 с тик), мгновенные события без полла.
- **Журнал событий** — кто что и когда сделал, хранится 30 дней.
- **Метрики** — 24-часовой график трафика, top-talkers, per-client история, дневные агрегаты на 365 дней.
- **AmneziaWG-обфускация** — Jc / Jmin / Jmax / S1 / S2 / H1–H4, кнопка регенерации.
- **AmneziaVPN-ссылки** + QR-коды (стандарт WG и AmneziaVPN).
- **Расходы на хостинг** — поделите стоимость сервера между «плательщиками»: расчётные периоды с равным делением суммы, авто-отключение должников после льготного периода, ручная отметка оплаты или приём платежей через ЮKassa.
- **Тёмная / светлая тема** с автоматикой по системе.
- **Rate-limit** на логине (5 попыток/мин на IP), session-cookie auth.
- **Healthz** для uptime-мониторинга.

## Сборка

```sh
go mod tidy
CGO_ENABLED=0 go build ./cmd/server
```

Фронт встраивается в бинарь через `go:embed` (`internal/static`). Перед сборкой:

```sh
cd web && npm install && npm run build
```

## Запуск

### Dokploy (основной способ)

Быстрый запуск в Dokploy с автоматической генерацией домена и пароля через Base64 import:

1. **Создание сервиса**: В панели Dokploy выберите **New Service → Compose**.
2. **Импорт конфигурации**: Откройте вкладку **Advanced**, скопируйте содержимое файла `deploy/dokploy/import.base64` из репозитория, вставьте в поле импорта и нажмите **Import**.
3. **Запуск**: Нажмите **Deploy**.
4. **Доступ**: Dokploy автоматически сгенерирует домен в интерфейсе **Domains** и пароль в переменных **Environment** (переменная `PASSWORD`). Откройте сгенерированный URL и авторизуйтесь.

#### Требования к хосту и сети
* Dokploy должен работать в режиме **Docker Compose**.
* В файрволе провайдера/хоста должен быть открыт диапазон UDP-портов `51820-51859` для VPN-подключений клиентов.
* На хосте должно присутствовать устройство `/dev/net/tun`.
* Для сохранения состояния используется один именованный том `amnezia-state`.

#### Просмотр логов
Настройка формата и уровня логирования через переменные окружения:
* `LOG_FORMAT` (`json` или `text`, по умолчанию `json`)
* `LOG_LEVEL` (`debug`, `info`, `warn`, `error`, по умолчанию `info`)

Логи контейнера доступны во вкладке **Logs** сервиса в Dokploy или через CLI:
```sh
docker compose logs -f panel
```

#### Дополнительные настройки (Post-Install)
После первого запуска вы можете опционально настроить кастомный HTTPS-домен в UI Dokploy, а также при необходимости заполнить `PUBLIC_URL` и ключи ЮKassa (`YOOKASSA_SHOP_ID`, `YOOKASSA_SECRET_KEY`) для приёма онлайн-оплаты.

---

### Docker Compose CLI (вторичный способ)

Для локального запуска или ручного развёртывания через Docker CLI с помощью корневого `docker-compose.yml`:

```sh
cp .env.example .env  # выставить WG_HOST и PASSWORD
docker compose up -d
```

### Голый docker run

```sh
docker build -t amnezia-panel .
docker run -d \
  --cap-add=NET_ADMIN --device=/dev/net/tun \
  --sysctl net.ipv4.ip_forward=1 \
  -e WG_HOST=vpn.example.com -e PASSWORD=secret \
  -p 51820:51820/udp -p 51821:51821/tcp \
  -v amnezia-state:/etc/amnezia/amneziawg \
  amnezia-panel
```

## Env

| Переменная | По умолчанию | Описание |
|---|---|---|
| `WG_HOST` | — | **обязательно** — внешний адрес сервера |
| `WG_INTERFACE` | `awg0` | имя сетевого интерфейса |
| `WG_PATH` | `/etc/amnezia/amneziawg` | каталог состояния (JSON + `.conf` + `panel.db`) |
| `WG_DEFAULT_ADDRESS` | `10.8.0.x` | подсеть (символ `x` подставляется для каждого клиента) |
| `WG_DEFAULT_DNS` | `1.1.1.1` | DNS по умолчанию |
| `WG_ALLOWED_IPS` | `0.0.0.0/0, ::/0` | по умолчанию весь трафик в туннель |
| `WG_MTU` | `0` | `0` = не задавать |
| `WG_PERSISTENT_KEEPALIVE` | `0` | секунд (0 = выкл) |
| `PORT` | `51821` | HTTP-порт панели |
| `WEBUI_HOST` | `0.0.0.0` | bind-адрес HTTP |
| `LOG_FORMAT` | `json` | формат логов (`json` или `text`) |
| `LOG_LEVEL` | `info` | уровень логирования (`debug`, `info`, `warn`, `error`) |
| `PASSWORD` | пусто | без пароля = без auth |
| `AWG_BIN` | `awg` | путь к `awg` |
| `AWG_QUICK_BIN` | `awg-quick` | путь к `awg-quick` |
| `JC`, `JMIN`, `JMAX`, `S1`, `S2`, `H1..H4` | дефолты | обфускация (`1,2,3,4` для H = триггер на random) |
| `YOOKASSA_SHOP_ID` | пусто | ID магазина ЮKassa — онлайн-оплата включается, только если заданы и `YOOKASSA_SHOP_ID`, и `YOOKASSA_SECRET_KEY`, и `PUBLIC_URL` |
| `YOOKASSA_SECRET_KEY` | пусто | секретный ключ ЮKassa |
| `YOOKASSA_VAT_CODE` | `1` | код НДС для чека (1 = без НДС) |
| `PUBLIC_URL` | пусто | внешний HTTPS-адрес панели для возврата после оплаты (напр. `https://vpn.example.com`) |
| `PAYMENT_CONTACT` | пусто | короткий контакт для ручной оплаты в кабинете (напр. `Telegram @mrcook1e`); показывается, пока не настроена ЮKassa |
| `BILLING_MIN_SHARE_PCT` | `25` | при дележе «по трафику» каждый платит не меньше этого % от равной доли (`0` — чистая пропорция, `100` — поровну) |

## API

```
GET    /healthz                                 — без auth, для uptime
GET    /api/session
POST   /api/session                              { password }   — rate-limited 5/мин
DELETE /api/session

GET    /api/stream                              — SSE: event + tick (1с)

GET    /api/wireguard/server/
POST   /api/wireguard/server/regenerate-magic
POST   /api/wireguard/server/restart
POST   /api/wireguard/server/reset-clients

GET    /api/wireguard/client/
POST   /api/wireguard/client/                    { name }
POST   /api/wireguard/client/import              { name, publicKey, ... }
DELETE /api/wireguard/client/{id}
POST   /api/wireguard/client/{id}/enable
POST   /api/wireguard/client/{id}/disable
PUT    /api/wireguard/client/{id}/name           { name }
PUT    /api/wireguard/client/{id}/address        { address }
PATCH  /api/wireguard/client/{id}                { notes, expiresAt, ...overrides }
GET    /api/wireguard/client/{id}/configuration
GET    /api/wireguard/client/{id}/qrcode.svg     (PNG)
GET    /api/wireguard/client/{id}/amnezia.vpn
GET    /api/wireguard/client/{id}/amnezia-qrcode.svg
GET    /api/wireguard/client/{id}/stats
GET    /api/wireguard/client/{id}/events

GET    /api/stats/overview
GET    /api/stats/series?range=24h
GET    /api/events?limit=50

GET    /api/billing/cycles
POST   /api/billing/cycles                       { title, periodStart, periodEnd, paymentDueAt, graceEndsAt, totalAmount }
GET    /api/billing/cycles/{id}
POST   /api/billing/cycles/{id}/publish
POST   /api/billing/invoices/{id}/pay
GET    /api/billing/summary

GET    /api/cabinet/{token}/billing              — публичная сводка для кабинета плательщика
POST   /api/cabinet/{token}/billing/checkout     { invoiceId, email }
POST   /api/billing/yookassa/webhook             — webhook ЮKassa
GET    /payment/return/{publicToken}             — редирект после оплаты

GET    /api/backup                              → tar.gz
POST   /api/restore                             multipart file=
```

## Стек

- **Backend**: Go 1.21, chi-router, `modernc.org/sqlite` (pure-Go).
- **Frontend**: Vue 3 `<script setup>`, Vite, Pinia, TypeScript, Tailwind, Onest + JetBrains Mono, [Tabler Icons](https://tabler-icons.io/).

## Лицензия

MIT. См. [LICENSE](LICENSE).
