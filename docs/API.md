# REST API — справочник для фронта

Всё под `/api`. Кодировка ответов: `application/json`, кроме явно отмеченных (`.conf`, `image/png`, `text/event-stream`, `application/gzip`).

## Содержание

- [Аутентификация](#аутентификация)
- [Сессия](#сессия)
- [Subscribers (admin)](#subscribers-admin)
- [Billing (admin)](#billing-admin)
- [Devices / WireGuard clients (admin)](#devices--wireguard-clients-admin)
- [Cabinet (public, token-auth)](#cabinet-public-token-auth)
- [Server-wide actions (admin)](#server-wide-actions-admin)
- [Stats и events (admin)](#stats-и-events-admin)
- [Backup / Restore (admin)](#backup--restore-admin)
- [SSE stream (admin)](#sse-stream-admin)
- [Healthcheck](#healthcheck)
- [Формат ошибок](#формат-ошибок)
- [Доменные типы](#доменные-типы)

---

## Аутентификация

Два независимых режима auth:

| Зона | Кто | Механизм |
|---|---|---|
| `/api/cabinet/{token}/*` | Subscriber (конечный юзер) | Magic-link: 256-bit `AccessToken` прямо в URL. Никаких куки. |
| Всё остальное под `/api/*` | Admin | Session cookie `awgp_sid`, выставляется после `POST /api/session`. |

Если `PASSWORD` env пуст — admin-auth отключён (cookie не требуется).

`POST /api/session` rate-limited: 5 попыток/мин на IP.

---

## Сессия

### `GET /api/session`
Состояние auth, без проверки cookie.
```json
{ "requiresPassword": true, "authenticated": false }
```

### `POST /api/session`
Body: `{ "password": "…" }`
- `200` → `{ "success": true }`, выставляет cookie `awgp_sid` (HttpOnly, SameSite=Lax, 7 дней)
- `401` → `{ "error": "Incorrect Password" }`
- `429` → `{ "error": "Too many attempts — try again in a minute" }`

### `DELETE /api/session`
Logout. Требует auth. Очищает cookie. `200 { "success": true }`.

---

## Subscribers (admin)

Subscriber = «Клиент» в UI = аккаунт-человек, владеющий несколькими устройствами.

### `GET /api/subscribers/`
Список без вложенных устройств.
```json
[
  {
    "id": "ab12cd34",
    "name": "Вася",
    "accessToken": "…256-bit base64url…",
    "url": "https://vpn.4ch.me/cabinet/…token…",
    "notes": "",
    "createdAt": "2026-06-01T12:00:00Z",
    "deviceCount": 2
  }
]
```
Сортировка: новые сверху (`createdAt` desc).

### `POST /api/subscribers/`
Body: `{ "name": "Вася", "notes": "опционально" }`
- `200` → `Subscriber` (без `deviceCount`/`devices`)
- `400` → `name is required`

### `GET /api/subscribers/{id}`
Деталка с массивом `devices` (полные `Client`-объекты, без live-handshake — для метрик есть отдельный endpoint).
```json
{
  "id": "ab12cd34",
  "name": "Вася",
  "accessToken": "…",
  "url": "…",
  "notes": "",
  "createdAt": "…",
  "deviceCount": 2,
  "devices": [ /* Client[] — см. ниже */ ]
}
```
- `404` если subscriber не найден.

### `PATCH /api/subscribers/{id}`
Любое подмножество:
```json
{ "name": "Новое имя", "notes": "…" }
```
- `200` → обновлённый `Subscriber`
- `400` при пустом `name`
- `404`

### `DELETE /api/subscribers/{id}`
**Cascade:** удаляет subscriber + все его devices + соответствующие profiles + поднимает `awgN` интерфейсы down.
- `200 { "success": true }` / `404`

### `POST /api/subscribers/{id}/regenerate-token`
Перевыпускает `AccessToken` — старая ссылка `/cabinet/<old>` сразу 404. Devices/profiles/интерфейсы не трогаются.
- `200` → `Subscriber` с новым токеном
- `404`

---

## Billing (admin)

Расходы на хостинг: админ создаёт расчётный период (draft) и публикует его —
сумма делится между подписчиками в роли `payer`. Статусы cycle:
`draft → published → closed`. Статусы invoice: `pending → paid | canceled`.
`splitMode`: `equal` (поровну) или `traffic` (пропорционально трафику за 30 дней,
каждый платит не меньше `BILLING_MIN_SHARE_PCT`% от равной доли).

### `GET /api/billing/summary`
```json
{ "totalReceived": 150000, "totalPending": 50000 }   // копейки
```

### `GET /api/billing/cycles`
Список периодов, новые сверху.

### `POST /api/billing/cycles`
Body (unix-секунды, `totalAmount` в копейках):
```json
{ "title": "Июль 2026", "periodStart": …, "periodEnd": …, "paymentDueAt": …, "graceEndsAt": …, "totalAmount": 300000, "splitMode": "traffic" }
```
- `201` → `BillingCycle` (статус `draft`)
- `400` при невалидных датах/сумме/режиме

### `GET /api/billing/cycles/{id}`
Деталка с массивом `invoices[]` (счета плательщиков).

### `GET /api/billing/cycles/{id}/preview`
Предпросмотр дележа по `splitMode` цикла без записи:
```json
[ { "subscriberId": "ab12cd34", "subscriberName": "Вася", "bytes": 17179869184, "amount": 75000 } ]
```
`bytes` — трафик за 30 дней (значимо только для `traffic`).

### `POST /api/billing/cycles/{id}/publish`
Фиксирует состав и суммы счетов (равный делёж, remainder — первому по порядку).
Иммутабельно. `400` если нет `payer`-подписчиков или статус не `draft`.

### `POST /api/billing/cycles/{id}/close`
`published → closed` (архив). `400` если статус не `published`.

### `DELETE /api/billing/cycles/{id}`
Удалить цикл. Разрешён **только** `draft` (без счетов). `400` для `published`/`closed`.

### `POST /api/billing/invoices/{id}/pay`
Ручная отметка оплаты (напр. после перевода через Telegram). Idempotent.
Реактивирует устройства подписчика, если нет других просроченных счетов.

### `POST /api/billing/invoices/{id}/cancel`
Списать pending-счёт («простить»). Idempotent для уже списанных; `paid` отменить
нельзя. Также реактивирует устройства при отсутствии других долгов.

> Cabinet-эндпоинты биллинга (публичные, по токену) — см. [Cabinet](#cabinet-public-token-auth).

---

## Devices / WireGuard clients (admin)

В Go-коде называется `Client`, в UI — «Устройство». 1 Device ↔ 1 Profile ↔ 1 `awgN` интерфейс.

> **Прямого создания админом нет.** Devices создаёт сам subscriber через `/api/cabinet/.../devices`. Админ может только импортировать существующий peer (см. `/import`).

### `GET /api/wireguard/client/`
Список всех устройств всех subscribers с live-метриками (merge handshake/transfer из `awg show dump`).
```json
[
  {
    "id": "uuid",
    "subscriberId": "ab12cd34",
    "profileId": "dev-ab12cd34-xxxxxx",
    "name": "iPhone",
    "address": "10.8.0.5",
    "privateKey": "…",
    "publicKey": "…",
    "preSharedKey": "…",
    "enabled": true,
    "createdAt": "…",
    "updatedAt": "…",
    "notes": "",
    "expiresAt": null,
    "dnsOverride": "",
    "allowedIPsOverride": "",
    "mtuOverride": 0,
    "totalRx": 0,
    "totalTx": 0,
    "lastHandshakeAt": null,
    "subscriberName": "Вася",
    "latestHandshakeAt": "2026-06-07T10:00:00Z",
    "transferRx": 12345,
    "transferTx": 6789,
    "persistentKeepalive": ""
  }
]
```
Сортировка: по `name` asc.

### `POST /api/wireguard/client/import`
Re-attach peer с уже существующей парой ключей.
```json
{
  "name": "iPhone restored",
  "subscriberId": "ab12cd34",  // обязателен
  "profileId": "dev-…",        // опционален — иначе lowest-port profile
  "publicKey": "…",            // обязателен
  "privateKey": "…",
  "preSharedKey": "…",
  "address": "10.8.0.5",       // опционален — иначе автоаллокация
  "notes": ""
}
```
- `200` → `Client`
- `400` без `name`/`publicKey`/`subscriberId`, или конфликт `address`/`publicKey`
- `404` если `subscriberId` или `profileId` не существует

> ⚠ Сейчас фронтовая `ImportClientModal` не запрашивает `subscriberId` — функционально сломана (см. STATE.md).

### `DELETE /api/wireguard/client/{id}`
Удаляет device + его profile + опускает iface. `200 { "success": true }` / `404`.

### `POST /api/wireguard/client/{id}/enable`
### `POST /api/wireguard/client/{id}/disable`
Включить/выключить peer. Триггерит `syncconf` интерфейса. `200 { "success": true }` / `404`.

### `PUT /api/wireguard/client/{id}/name`
Body: `{ "name": "новое имя" }`. `200 { "success": true }` / `400` пустое имя / `404`.

### `PUT /api/wireguard/client/{id}/address`
Body: `{ "address": "10.8.0.7" }`. Валидируется по /24 и uniqueness. `400` / `404` / `200`.

### `PATCH /api/wireguard/client/{id}`
Все поля опциональны, поле = null/отсутствует → без изменения. Особенность: для очистки nullable полей есть отдельный флаг.
```json
{
  "notes": "…",
  "expiresAt": "2026-12-31T23:59:59Z",
  "clearExpiresAt": false,
  "dnsOverride": "1.1.1.1",
  "allowedIPsOverride": "0.0.0.0/0",
  "mtuOverride": 1280
}
```
- `clearExpiresAt: true` → сбросить `expiresAt`. Иначе если передано — обновляет.
- `200` → полный `Client` / `404`.

### `GET /api/wireguard/client/{id}/configuration`
`Content-Type: text/plain`, `Content-Disposition: attachment; filename="<name>.conf"`. Готовый AWG `.conf` для импорта в клиент. `404` если device или его profile не найдены.

### `GET /api/wireguard/client/{id}/qrcode.svg`
**Возвращает PNG** (несмотря на расширение в URL). `Content-Type: image/png`, 512×512. То же содержимое что `.conf`.

### `GET /api/wireguard/client/{id}/amnezia.vpn`
`Content-Type: text/plain`. Возвращает `vpn://AAAN...` строку для импорта в **официальный AmneziaVPN клиент** (Android/iOS/Win/Mac/Linux). Совместимый формат: 4-байтная BE-длина + zlib(JSON ServerConfig) + base64url-без-padding.

### `GET /api/wireguard/client/{id}/amnezia-qrcode.svg`
**PNG 768×768**, error-correction уровень Low (vpn-URL длинный, 1–2 KB; Medium/High не влезает в QR-version 40). Кодирует ту же `vpn://...` строку — поднеси к камере, AmneziaVPN сам распознает.

### `GET /api/wireguard/client/{id}/stats`
Метрики из SQLite (per-client история).

### `GET /api/wireguard/client/{id}/events`
Журнал событий, фильтр по device id. Query `?limit=50` (опционально).

---

## Cabinet (public, token-auth)

Auth = `token` в URL. Никаких кук, CORS не требуется (тот же origin).

### `GET /api/cabinet/{token}`
Snapshot кабинета для subscriber'а.
```json
{
  "name": "Вася",
  "devices": [
    {
      "id": "uuid",
      "name": "iPhone",
      "address": "10.8.0.5",
      "enabled": true,
      "createdAt": "…",
      "latestHandshakeAt": "2026-06-07T10:00:00Z"
    }
  ]
}
```
Сортировка `devices`: по `createdAt` asc.
- `404 { "error": "cabinet not found" }` если token не валиден.

### `POST /api/cabinet/{token}/devices`
Subscriber добавляет себе устройство.
```json
{
  "preset": "awg2",
  "deviceName": "iPhone"
}
```

`preset` — поколение протокола AmneziaWG, обфускация генерируется на сервере:

| preset | Что получает клиент | Кому |
|---|---|---|
| `awg1` | Jc/Jmin/Jmax, S1–S2, фиксированные H1–H4. Без S3/S4, без I*, без 3.x-ключей | Роутеры и старые клиенты: Keenetic, OpenWrt, GL.iNet, kernel-модуль 1.0 |
| `awg2` | + S3/S4, H как диапазоны, официальный DNS-мимикрия I1 | Совместимость — работает со всеми версиями приложений Amnezia |
| `awg31` | + HeaderProtectionKey, ContentPaddingAddition, рандомизированные таймеры, RandomTrailers, DisableCookies; S1–S4 ≥ 12, H1–H4 = 1/2/3/4 | Максимальная защита, нужна AmneziaVPN 5.x |

Пустой или неизвестный `preset` → `awg2` (значение `awg.DefaultPreset`). Легаси-значения
`auto` / `stealth` / `fast` принимаются и тоже дают `awg2` — ровно то, что они генерировали раньше.

Альтернатива: `{"snippet": "[Interface]\nJc = 4\n…"}` без `preset` — обфускация берётся из
snippet (см. [snippet формат](#snippet-формат)).

Backend создаёт `Profile + Client + awgN` интерфейс, поднимает его, рендерит `.conf`.

**Ответ:**
```json
{
  "deviceId": "uuid",
  "name": "iPhone",
  "address": "10.8.0.5",
  "conf": "[Interface]\nPrivateKey = …\n…",
  "qrPng64": "iVBORw0KGgoAAAA…"          // base64 PNG, рисуется <img src=data:image/png;base64,...>
}
```
- `400` — невалидный JSON / пустой snippet / ошибка валидации snippet (текст ошибки в `error`)
- `404` — token не найден
- `500` — внутренний сбой (генерация ключей, поднятие интерфейса)

### `DELETE /api/cabinet/{token}/devices/{devId}`
Удалить своё устройство. Backend проверяет что `devId` принадлежит owner-у токена — иначе `404` (не `403`, чтобы не палить существование чужих ID).

### `GET /api/cabinet/{token}/devices/{devId}/configuration`
Скачать `.conf` повторно. Те же `Content-Type/Disposition` что у admin-эндпоинта.

### `GET /api/cabinet/{token}/devices/{devId}/qrcode.svg`
Повторный QR. PNG, 512×512.

### `GET /api/cabinet/{token}/devices/{devId}/amnezia.vpn`
То же что admin-эндпоинт (`vpn://...` строка), но с проверкой что device принадлежит owner-у токена.

### `GET /api/cabinet/{token}/devices/{devId}/amnezia-qrcode.svg`
PNG 768×768 с `vpn://...` для AmneziaVPN-клиента.

### `GET /api/cabinet/{token}/billing`
Сводка по подписке для плательщика. Для `owner`/`trusted` → `derivedStatus: "exempt"`.
```json
{
  "billingRole": "payer",
  "derivedStatus": "pending",            // exempt|pending|grace|overdue|paid
  "checkoutEnabled": false,              // true когда настроена ЮKassa
  "paymentContact": "Telegram @mrcook1e", // способ ручной оплаты
  "latestInvoice": { /* BillingInvoice */ },
  "latestCycle": { /* BillingCycle */ },
  "history": [ { "cycleTitle": "Июнь 2026", "amount": 150000, "status": "paid", "periodEnd": …, "paidAt": … } ]
}
```

### `POST /api/cabinet/{token}/billing/checkout`
Только при настроенной ЮKassa. Body: `{ "invoiceId": …, "email": "…" }` →
`{ "confirmationUrl": "https://yoomoney.ru/…" }`.

---

## Server-wide actions (admin)

### `POST /api/wireguard/server/reset-clients`
Удаляет ВСЕ devices (clients) на всех profiles, profiles и subscribers сохраняются. `200 { "success": true }`.

### `POST /api/wireguard/server/factory-reset`
Полный сброс: subscribers, devices, profiles, метрики, события. Сервер возвращается в состояние свежей установки.

---

## Stats и events (admin)

### `GET /api/stats/overview`
Сводка для дашборда (top-talkers, totals).

### `GET /api/stats/series?range=24h`
Временной ряд трафика. `range` форматы: `5m`, `1h`, `24h`, `7d`, `30d`. Clamp `[5m, 90d]`. Bucket выбирается автоматически (минута/5мин/15мин/час/6ч).
```json
{ "bucketSeconds": 900, "points": [ /* … */ ] }
```

### `GET /api/events?limit=50`
Журнал событий, новые сверху. Без фильтра.

---

## Backup / Restore (admin)

### `GET /api/backup`
`Content-Type: application/gzip`, `Content-Disposition: attachment; filename="amneziawg-panel-YYYYMMDD-HHMMSS.tar.gz"`. Внутри: `state.json` + `panel.db` + `awgN.conf` файлы.

### `POST /api/restore`
Принимает либо `multipart/form-data` (поле `file`), либо сырое тело `application/gzip`. После распаковки вызывает manager reload (перезапускает все интерфейсы).
- `200 { "success": true, "restored": ["state.json", "panel.db", "awg0.conf"] }`
- `400` если архив не gzip / пустой / без распознанных файлов
- `500` при ошибке reload

> Только whitelisted файлы извлекаются: `state.json`, `panel.db`, `awgN.conf` для уже существующих ifaces. На fresh-install `.conf` файлы будут отброшены, но это OK — Reload их перерендерит из `state.json`.

---

## SSE stream (admin)

### `GET /api/stream`
Server-Sent Events. Auth — обычный admin cookie.

**Сообщения:**

| event | data | частота |
|---|---|---|
| `hello` | `{"id": <streamN>}` | один раз при подключении |
| `tick` | `{"ts": …, "rxRate": …, "txRate": …, "online": N, "clients": [{"id","rxRate","txRate","online"}]}` | 1/сек |
| `audit` | `events.Event` | моментально при каждом событии |
| (comment `: ping`) | — | каждые 15 секунд (keep-alive через прокси) |

`rxRate`/`txRate` — байт/сек, считается diff'ом kernel-счётчиков. Reset (после `awg-quick down/up`) сглаживается отрицательной дельтой → 0.

Если никто не подписан, `tick`-цикл не дёргает `awg show dump`.

**Пример клиента:**
```ts
const es = new EventSource('/api/stream', { withCredentials: true })
es.addEventListener('hello',  e => …)
es.addEventListener('tick',   e => { const t = JSON.parse(e.data); … })
es.addEventListener('audit',  e => { const ev = JSON.parse(e.data); … })
```

---

## Healthcheck

### `GET /healthz`
Без auth. `200 { "status": "ok" }`. Для uptime-мониторов и Dokploy healthcheck.

---

## Формат ошибок

Все JSON-ответы об ошибке — единая форма:
```json
{ "error": "human-readable message" }
```
Статусы:
- `400` — bad request (невалидный JSON, пустые обязательные поля, ошибки парсинга snippet)
- `401` — нет/невалидный auth (для admin-зоны)
- `404` — ресурс не найден (subscriber/client/profile/cabinet)
- `429` — rate limit (только `POST /api/session`)
- `500` — внутренняя ошибка (I/O, awg-quick, генерация ключей)

Cabinet-endpoints возвращают `404 "cabinet not found"` для невалидного токена и для попытки доступа к чужому device — намеренно одинаково.

---

## Доменные типы

### Subscriber
```ts
interface Subscriber {
  id: string            // 8-char slug
  name: string
  accessToken: string   // 256-bit base64url
  url: string           // https://<host>/cabinet/<token>, собирается per-request из X-Forwarded-Host
  notes?: string
  createdAt: string     // RFC3339
  deviceCount?: number  // только в list/get
  devices?: Client[]    // только в /subscribers/{id}
}
```

### Client (device)
```ts
interface Client {
  id: string                       // uuid
  subscriberId: string
  profileId: string                // "dev-<sub>-<rand>"
  name: string
  address: string                  // "10.8.0.5"
  privateKey: string
  publicKey: string
  preSharedKey: string
  enabled: boolean
  createdAt: string
  updatedAt: string

  notes?: string
  expiresAt?: string | null        // авто-disable когда наступит

  dnsOverride?: string
  allowedIPsOverride?: string
  mtuOverride?: number             // 0 = use default

  totalRx?: number                 // накопленный счётчик с момента создания
  totalTx?: number
  lastHandshakeAt?: string | null  // из persisted state

  // только в /api/wireguard/client/ (live merge):
  subscriberName?: string
  latestHandshakeAt?: string | null  // live приоритетнее, fallback на lastHandshakeAt
  transferRx?: number              // мгновенные счётчики, сбрасываются при ifaceDown
  transferTx?: number
  persistentKeepalive?: string
}
```

### CabinetView
```ts
interface CabinetView {
  name: string
  devices: Array<{
    id: string
    name: string
    address: string
    enabled: boolean
    createdAt: string
    latestHandshakeAt?: string | null
  }>
}
```

### Snippet формат

Snippet принимается как блок `[Interface]` или плоские `key = value`. Парсер игнорирует комментарии (`#`/`;`), любые `[Section]` заголовки, и стандартные WG-ключи (`PrivateKey`, `Address`, `ListenPort`, `DNS`, `MTU`, `AllowedIPs`, `Endpoint`, `PersistentKeepalive`, `PostUp/Down`, `PreUp/Down`, `FwMark`, `Table`, `SaveConfig`).

**Обязательные:**
- `Jc` (0..128), `Jmin` (0..1280), `Jmax` (0..1280) — `Jmax > Jmin` если ненулевой
- `S1..S3` (0..1280), `S4` (0..32) — не все нули; инварианты `S1+56 != S2`, `S1+56 != S3`, `S2+92 != S3`
- `H1..H4` — `"n"` (фиксированное значение, AWG 1.0 и 3.1) или `"min-max"` (диапазон, AWG 2.0);
  диапазоны без пересечений. `"5-5"` нормализуется в `"5"`

**Опциональные (AWG 2.0):**
- `I1..I5` — opaque CPS-строки; теги `b/t/r/rc/rd/d/ds/dz`

**Опциональные (AWG 3.x):**
- `HeaderProtectionKey` — base64, ровно 32 байта (`awg genkey`).
  **При заданном ключе `S1..S4` обязаны быть ≥ 12** — amneziawg-go берёт nonce ChaCha20
  из первых 12 байт S-префикса и иначе отклоняет UAPI-set
- `ContentPaddingAddition`, `RekeyAfterTime`, `RekeyTimeout`, `RejectAfterTime`,
  `KeepaliveTimeout`, `MaxHandshakeAttempts` — `"n"` или `"lo-hi"`, значения ≤ 65535
  (amneziawg-tools парсит их как u16-диапазоны)
- `RandomTrailers`, `DisableCookies` — `on` / `off`

> ⚠ `Itime` / `J1..J3` из беты AWG 1.5 **принимаются и молча отбрасываются**: их нет ни в
> amneziawg-go v3, ни в amneziawg-tools v3.1. Эмитить их нельзя — tools обрывают весь
> интерфейс на любом неизвестном ключе `[Interface]`.

Дубликат ключа → ошибка. Неизвестный ключ → молча игнорируется.

Новые ключи попадают в `.conf` **только когда заданы**, поэтому профили AWG 1.0/2.0
рендерятся байт-в-байт как раньше и остаются читаемыми для старых `amneziawg-tools`.

Генерировать snippet можно в [AmneziaWG-Architect](https://vadim-khristenko.github.io/AmneziaWG-Architect/).
