# amneziawg-panel — состояние проекта

Документ-передача для следующей сессии. Описывает архитектуру после миграции на AWG 2.0 + модель «Клиент → Устройства».

## TL;DR

Self-hosted Go-панель для управления AmneziaWG-сервером. Хост: `vpn.4ch.me` (155.212.226.85), деплой через Dokploy с автодеплоем из `main`.

```
Admin   → создаёт «Клиента» (Subscriber) → получает ссылку /cabinet/<token>
Subscriber → открывает ссылку → добавляет «Устройства» (Devices) → скачивает .conf/QR
```

Каждое устройство = свой `awgN` интерфейс на отдельном UDP-порту со своим snippet'ом обфускации (берётся из [AmneziaWG-Architect](https://vadim-khristenko.github.io/AmneziaWG-Architect/)).

## Стек

- **Backend:** Go 1.22, chi router, SQLite (modernc.org/sqlite), embedded SPA через `//go:embed`
- **Frontend:** Vue 3 (Composition API + `<script setup>`), TypeScript, Vite 5, Pinia, Tailwind
- **Userspace ядро:** `amneziawg-go` v0.2.18 + `amneziawg-tools` v1.0.20260223 (тулзы пока **не парсят** `Itime` / `J1-J3` — поля в snippet'е принимаем, но в `.conf` не эмитим)
- **Deploy:** Dokploy на `main` push, multi-stage Dockerfile собирает три бинаря (panel/amneziawg-go/amneziawg-tools)

## Модель данных

`SchemaVersion = 4` в `internal/awg/config.go`. При несовпадении схемы — fail-fast (`ErrSchemaTooOld`). Миграций нет, продакшн state можно стирать без жалости.

```
Subscriber (account)         Device (Client в Go)            Profile
├── ID (8-char slug)          ├── ID (uuid)                   ├── ID (dev-<sub>-<rand>)
├── Name ("Вася")             ├── SubscriberID (FK)           ├── Iface (awgN)
├── AccessToken (256-bit)     ├── ProfileID (1:1)             ├── Port (51820+)
├── Notes                     ├── Name ("iPhone")             ├── PrivateKey/PublicKey
└── CreatedAt                 ├── PrivateKey/PublicKey         ├── Address (10.8.0.1)
                              ├── Address (10.8.0.5)           └── Obf params (Jc/J/S/H/I)
                              ├── Enabled
                              ├── ItimeOverride *int (per-Windows)
                              └── stats (TotalRx/Tx, LastHandshakeAt)
```

**Инвариант:** 1 Device ↔ 1 Profile ↔ 1 awgN-интерфейс. Из-за того что параметры обфускации привязаны к интерфейсу, не к человеку.

## Структура кода

```
internal/
├── api/
│   ├── router.go              ← маршруты, см. секцию ниже
│   ├── handlers.go            ← клиент-handlers (бывшие per-device admin)
│   ├── handlers_admin.go      ← backup/restore/factory-reset/import
│   ├── handlers_profiles.go   ← /api/profiles (legacy, оставлен для совместимости)
│   ├── handlers_subscribers.go ← admin CRUD по subscribers
│   ├── handlers_cabinet.go    ← public, auth по token-в-URL
│   ├── handlers_stats.go      ← per-device метрики
│   ├── auth.go, ratelimit.go, stream.go (SSE)
├── awg/
│   ├── manager.go             ← core: profiles/clients/subscribers state + iface lifecycle
│   ├── subscriber.go          ← Subscriber type + CRUD + AddDevice/DeleteDevice
│   ├── profile.go             ← Profile type (с 2.0-полями)
│   ├── config.go              ← Config (state.json) + Client struct + templates рендера
│   ├── parse.go               ← ParseObfuscation(snippet) → ObfuscationSpec
│   ├── store.go               ← JSON state I/O + fail-fast на старой схеме
│   ├── keys.go, ipam.go       ← keygen + IP-аллокация
│   ├── portipam.go            ← порт→iface (awgN) аллокация
│   ├── sync.go                ← awg-quick wrapper, awg setconf
│   ├── status.go              ← awg show dump парсер
├── config/config.go           ← env vars (WG_HOST, порт-range, AWG-бинари и т.д.)
├── db/db.go                   ← SQLite (peer_samples, peer_daily, events)
├── events/events.go           ← журнал событий + SSE
├── stats/                     ← коллектор и запросы по метрикам
└── static/                    ← embed.FS для SPA

web/
├── src/
│   ├── views/
│   │   ├── ClientsView.vue        ← главная (admin): список subscribers + раскрывающиеся devices
│   │   ├── ClientDetailView.vue   ← деталка устройства (метрики, enable/disable, конфиг)
│   │   ├── SettingsView.vue       ← theme/backup/restore/factory-reset/события
│   │   ├── CabinetView.vue        ← публичный кабинет /cabinet/:token
│   │   ├── LoginView.vue
│   ├── components/
│   │   ├── organisms/
│   │   │   ├── SubscriberModal.vue   ← двухфазная (создать → показать URL)
│   │   │   ├── ConfigModal.vue       ← admin: скачать .conf клиента
│   │   │   ├── QrModal.vue           ← admin: показать QR
│   │   │   ├── ImportClientModal.vue ← ⚠ функционально сломан (см. TODO)
│   │   │   ├── ProfileModal.vue      ← ⚠ unused в UI, ждёт удаления
│   │   │   ├── NewClientModal.vue    ← ⚠ unused, ждёт удаления
│   │   ├── molecules/, atoms/
│   ├── stores/
│   │   ├── subscribers.ts ← основной admin store
│   │   ├── clients.ts     ← devices в плоском списке (для топа, метрик)
│   │   ├── stats.ts, theme.ts, toasts.ts, auth.ts
│   │   ├── profiles.ts    ← ⚠ orphaned, кандидат на удаление
│   ├── lib/api.ts         ← все REST endpoints клиента
│   ├── types.ts           ← TS-типы (Subscriber, Client, CabinetView, AddDeviceResult, ...)
│   ├── router/index.ts    ← /, /clients/:id, /settings, /cabinet/:token, /login
```

## API endpoints

**Public (auth = token в URL):**
- `GET    /api/cabinet/:token`                          — snapshot кабинета: `{name, devices[]}`
- `POST   /api/cabinet/:token/devices`                  — body `{snippet, deviceName}` → создаёт device + profile + iface, возвращает `{conf, qrPng64}`
- `DELETE /api/cabinet/:token/devices/:devId`           — удалить своё устройство (cascade с iface)
- `GET    /api/cabinet/:token/devices/:devId/configuration` — скачать .conf повторно
- `GET    /api/cabinet/:token/devices/:devId/qrcode.svg`    — QR PNG

**Admin (auth = session-cookie):**
- `/api/subscribers/*` — CRUD subscribers, `POST /:id/regenerate-token` (старая ссылка ломается)
- `/api/wireguard/client/*` — операции на устройствах: list/import/delete/enable/disable/rename/config/qr (+ patch/stats/events)
- `/api/profiles/*` — legacy, не используется UI (создание профиля админом убрано)
- `/api/wireguard/server/reset-clients` — снести все устройства
- `/api/wireguard/server/factory-reset` — снести всё (subscribers + devices + profiles)
- `/api/backup`, `POST /api/restore`, `GET /api/events`, `GET /api/stream` (SSE)

## Snippet формат

Парсер `ParseObfuscation` в `internal/awg/parse.go`. Принимает блок `[Interface]` или плоские пары `key = value`. Игнорирует комментарии, секции `[Peer]`, `PrivateKey`/`Address`/`ListenPort`/etc.

**Принимаются и сохраняются:**
- `Jc, Jmin, Jmax` (junk train)
- `S1..S4` (padding, с инвариантами `S1+56 ≠ S2`, `S1+56 ≠ S3`, `S2+92 ≠ S3`)
- `H1..H4` (диапазоны `min-max`, без пересечений)
- `I1..I5` (CPS-строки)
- `J1..J3`, `Itime` — **принимаются, но не эмитятся в .conf** (см. TODO)

## Известные TODO / unfinished

### Высокий приоритет
1. **Itime / J1-J3 в `.conf`** — закомментировано в шаблонах [config.go](amneziawg-panel/internal/awg/config.go:42-50). `amneziawg-tools v1.0.20260223` не парсит эти поля (`awg setconf` падает с `Line unrecognized`). Включить обратно когда upstream tools релизнут поддержку. Альтернатива — пушить эти поля напрямую через UAPI `amneziawg-go` после `awg-quick up`.

### Средний приоритет
2. **ImportClientModal сломан функционально.** Backend требует `subscriberId`, форма его не запрашивает. Restore-сценарий редкий, но если нужен — добавить в [ImportClientModal.vue](amneziawg-panel/web/src/components/organisms/ImportClientModal.vue) выбор существующего subscriber'а и устройство-под-новый-iface.
3. **Mertv-код:** [ProfileModal.vue](amneziawg-panel/web/src/components/organisms/ProfileModal.vue), [NewClientModal.vue](amneziawg-panel/web/src/components/organisms/NewClientModal.vue), [stores/profiles.ts](amneziawg-panel/web/src/stores/profiles.ts) — больше не используются UI. Удалить.
4. **`/api/profiles/*` legacy-endpoints** — UI к ним не обращается, можно убрать вместе с [handlers_profiles.go](amneziawg-panel/internal/api/handlers_profiles.go).
5. **ClientDetailView** — продолжает работать, но не показывает связь с Subscriber. Добавить хлебные крошки «Вася → iPhone».

### Низкий приоритет
6. **Конфигуратор мимикрии в `CabinetView`** — сейчас клиент должен идти в Architect и копировать snippet. Можно встроить мини-конфигуратор (выпадашка профилей мимикрии → автогенерация snippet'а на лету). Либо встроить весь Architect через [goja](https://github.com/dop251/goja) на сервере (обсуждалось, MIT-лицензия позволяет).
7. **Per-client speed/traffic limits** — отдельный эпик, в коде нет.
8. **2FA для кабинета** — token в URL = magic-link. Достаточно для v1, но можно добавить email-OTP / device-fingerprint binding позже.

## Deploy + smoke test

**Workflow:** `git push main` → Dokploy webhook → multi-stage build (≈3-5 мин) → перезапуск контейнера `amnezia-panel`.

**Если меняешь `SchemaVersion`** — обязательно стереть volume **до** деплоя:

```bash
ssh root@155.212.226.85 'docker stop amnezia-panel; rm -f /var/lib/docker/volumes/vpn-panel-bojlcp_amnezia-state/_data/*'
```

(volume mount: `/var/lib/docker/volumes/vpn-panel-bojlcp_amnezia-state/_data → /etc/amnezia/amneziawg`)

**Smoke test после деплоя:**

1. Открыть `https://vpn.4ch.me` → залогиниться
2. «+ Новый клиент» → имя → создать → скопировать ссылку
3. Открыть ссылку в инкогнито → попасть в кабинет
4. «+ Добавить устройство» → имя + snippet из [Architect](https://vadim-khristenko.github.io/AmneziaWG-Architect/) (AWG 2.0) → получить .conf + QR
5. Импортировать в клиент AmneziaWG → подключиться
6. Вернуться в админку → раскрыть строку клиента → видеть устройство со статистикой

**Сервер:**
- Хост: `root@155.212.226.85`
- Контейнер: `amnezia-panel` (Image `vpn-panel-bojlcp-panel`)
- Внутри: `awgpanel` (HTTP 51821) + `amneziawg-go` per-iface (51820+)
- Внешний nginx → Dokploy → этот контейнер

## История последних коммитов (главное)

- `0b9740d` — двухуровневая модель Subscriber/Device + кабинет
- `07a46e6` — не эмитим Itime/J1-J3 (tools ещё не умеет)
- `8c015d0` — invite UX в ClientsView, чистка SettingsView
- `cd6fb16` — фикс SPA редирект-цикла на `/onboard/<token>` (FileServer auto-redirect)
- `ff26784` — onboarding (legacy, заменён на subscriber)
- `47196d2` — BYOC: snippet от админа вместо генерации
- `55249ca` — interface pool: profiles + per-iface awgN
- `66ec682` — апгрейд AWG userspace до 2.0-capable

## Контекст по терминологии

| В UI пользователь видит | В Go-коде | Почему так |
|---|---|---|
| Клиент | `Subscriber` | UI на русском, Go-нейминг точнее семантически |
| Устройство | `Client` | Историческое имя — переименовать слишком инвазивно (89 ссылок) |
| Личный кабинет | cabinet | URL `/cabinet/:token` |
| Ссылка кабинета | AccessToken | 256-bit url-safe random, magic-link auth |

Если будешь редактировать код — в Go-файлах `Client` означает «устройство», `Subscriber` — «клиент-аккаунт». В TS — `Client` тоже = устройство.

## Полезные ссылки

- amneziawg-go: https://github.com/amnezia-vpn/amneziawg-go
- amneziawg-tools: https://github.com/amnezia-vpn/amneziawg-tools (релизы: смотри `v1.0.YYYYMMDD`)
- Architect (snippet generator): https://vadim-khristenko.github.io/AmneziaWG-Architect/
- Дока AWG 2.0: https://docs.amnezia.org/documentation/amnezia-wg/
