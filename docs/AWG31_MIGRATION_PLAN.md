# План перевода панели на AWG 3.1 (пресеты AWG 1.0 / 2.0 / 3.1)

Дата: 2026-08-29. Протокольная справка: `docs/AWG3_COMPAT.md` (обязательно к прочтению перед реализацией).

## Цель

1. Data plane панели — amneziawg-go **v3.1** (обратно совместим: один бинарь обслуживает конфиги всех поколений, существующие 2.0-профили продолжают работать без изменений).
2. Вместо кастомных пресетов кабинета (auto/stealth/fast) — **три пресета-поколения: AWG 1.0, AWG 2.0, AWG 3.1** с параметрами, максимально близкими к официальным дефолтам Amnezia (= максимальная совместимость с официальными клиентами и сторонними реализациями).
3. Удалить встроенный генератор обфускации:
   - `web/src/utils/generator.ts` (2208 строк, порт «AmneziaWG Architect») — **мёртвый код, не импортируется нигде в web/src** — удалить целиком;
   - `internal/awg/obfuscation_gen.go` — упростить: вместо полос auto/stealth/fast — три генератора-поколения по официальным дефолтам.
   - **`internal/awg/keys.go` НЕ трогаем** — это генерация WG-ключей через `awg genkey/genpsk/pubkey`, она нужна и дополнительно понадобится для HeaderProtectionKey.

## Этап 0 — стек (Dockerfile)

```dockerfile
ARG AWG_TOOLS_REF=v3.1.20260812   # было v1.0.20260223
ARG AWG_GO_REF=v3.1.20260828      # было v0.2.18
```

- amneziawg-go v3 сменил модуль на `github.com/amnezia-vpn/amneziawg-go/v3` и требует Go ≥ 1.25 — проверить версию тулчейна в стадии awggo (стадии backend это не касается, «two toolchains by design»).
- `WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go` уже стоит — data plane детерминированно userspace, kernel-модуль не участвует. Ничего менять не надо.
- Обновить комментарии в Dockerfile («AWG 2.0-capable» → 3.1) и AGENTS.md (абзац про ParseObfuscation/Itime — см. этап 1).

Проверка: пересобрать образ, поднять существующий 2.0-профиль, handshake от текущего клиента проходит (wire-формат v3-ядра без 3.x-параметров идентичен 2.0).

## Этап 1 — модель данных

### Profile (internal/awg/profile.go)

Добавить (все `omitempty`, пустое = не задано = поведение 2.0):

```go
HeaderProtectionKey  string `json:"headerProtectionKey,omitempty"` // base64, awg genkey
ContentPaddingAddition string `json:"contentPaddingAddition,omitempty"` // "lo-hi"
RekeyAfterTime       string `json:"rekeyAfterTime,omitempty"`
RekeyTimeout         string `json:"rekeyTimeout,omitempty"`
RejectAfterTime      string `json:"rejectAfterTime,omitempty"`
KeepaliveTimeout     string `json:"keepaliveTimeout,omitempty"`
MaxHandshakeAttempts string `json:"maxHandshakeAttempts,omitempty"`
RandomTrailers       bool   `json:"randomTrailers,omitempty"`
DisableCookies       bool   `json:"disableCookies,omitempty"`
PersistentKeepalive  string `json:"persistentKeepalive,omitempty"` // "25-35"; пусто = текущее поведение
```

Удалить `J1, J2, J3, Itime` — мертвы окончательно (нет ни в go v3, ни в tools 3.1; выпилены из спеки). Старый state с этими json-ключами загрузится нормально (unknown-поля игнорируются).

### ObfuscationSpec + ParseObfuscation + Validate (internal/awg/parse.go)

- Добавить 3.x-поля в `ObfuscationSpec`; в `ParseObfuscation` принимать новые ключи; `Itime`/`J1–J3` продолжать **принимать и игнорировать** (толерантность к старым сниппетам), убрать их из spec.
- Валидация:
  - `HeaderProtectionKey` — валидный base64 32 байта;
  - при заданном HPK: **S1–S4 ≥ 12** (nonce ChaCha20 живёт в первых 12 байтах S-префикса; ядро отклонит set);
  - диапазоны `lo-hi`: lo ≤ hi, значения ≤ 65535 (tools парсят u16-диапазоны);
  - булевы — `on`/`off`;
  - существующие проверки (S-коллизии, H-непересечение) сохраняются.

### Схема состояния

`SchemaVersion` 4 → 5 (профили могут нести 3.x-поля). Заодно добавить верхнюю границу в store.go:52 — сейчас state новее бинаря загружается молча и новые поля будут потеряны при следующем Save (downgrade-коррупция): `if c.SchemaVersion > SchemaVersion { fail fast }`.

## Этап 2 — пресеты-поколения (internal/awg/obfuscation_gen.go)

`GenerateObfuscation(preset)` → три пресета. Параметры — официальные дефолты клиента Amnezia 5.x (см. AWG3_COMPAT.md, «Дефолтный профиль 3.1»):

| Параметр | `awg1` | `awg2` | `awg31` |
|---|---|---|---|
| Jc / Jmin / Jmax | rand(3–6) / 10 / 50 | rand(4–7) / 10 / 50 | rand(4–6) / 10 / 50 |
| S1, S2 | rand(15–150) | rand(15–150) | rand(12–150) |
| S3 / S4 | — (не эмитим) | rand(15–64) / 12 | rand(12–64) / 12 |
| H1–H4 | 4 уникальных случайных uint32 ≥ 5, одиночные | случайные диапазоны (как сейчас randHRange) | **1, 2, 3, 4** (стандартные WG — HPK всё шифрует) |
| I1 | — | официальный DNS-CPS (defaultI1CPS) | официальный DNS-CPS |
| HPK, CPA, таймеры, трейлеры | — | — | HPK=genkey; CPA=10-100; RekeyAfterTime=100-120; RekeyTimeout=3-7; RejectAfterTime=150-180; KeepaliveTimeout=5-15; MaxHandshakeAttempts=15-20; PersistentKeepalive=25-35; RandomTrailers=on; DisableCookies=on |

- Анти-коллизии размеров как в официальном инсталлере (уникальность S, `S1+56 ≠ S2` и т.д.) — код уже есть, переиспользовать.
- Кастомные полосы stealth/fast и randHRange-эвристики удалить. Легаси-значения API `auto|stealth|fast` принимать как алиас → `awg2` (идентично сегодняшнему поведению для закешированных бандлов кабинета).
- HPK генерится **на профиль** (у нас 1 устройство = 1 профиль = 1 iface — инвариант сохраняется, ключ не шарится между подписчиками).

## Этап 3 — рендер конфигов (internal/awg/config.go)

Раскладка ключей по сторонам (сервер = наш awgN.conf, клиент = выдаваемый .conf/QR; референс — template.conf официального клиента):

| Ключ | Сервер | Клиент | Примечание |
|---|---|---|---|
| Jc/Jmin/Jmax | ✓ (как сейчас) | ✓ | initiator-only, но официально ставится с обеих сторон |
| S1–S4, H1–H4 | ✓ | ✓ | must-match |
| I1–I5 | — (как сейчас) | ✓ | initiator-only |
| HeaderProtectionKey | ✓ | ✓ | **must-match** |
| RandomTrailers | ✓ | ✓ | **must-match** (приёмник с off дропает) |
| DisableCookies | ✓ | ✓ | серверный механизм, официально ставится с обеих |
| ContentPaddingAddition | — | ✓ | односторонний |
| Rekey*/Reject*/Keepalive*/MaxHandshakeAttempts | — | ✓ | клиентские таймеры |
| PersistentKeepalive | — | ✓ (диапазон) | peer-секция |

Все новые ключи эмитить **только когда заданы** — awg1/awg2-профили дают байт-в-байт прежний conf (регрессия нулевая), а конфы остаются совместимы со старыми tools при ручном переносе.

## Этап 4 — vpn:// экспорт (internal/awg/amneziaurl.go)

1. Добавить заданные 3.x-ключи в `last_config` и в `awg{}` (имена = conf-ключи: `"HeaderProtectionKey"`, `"RandomTrailers"`... — клиент 5.x читает именно их).
2. `protocol_version`: `"3.1"` для awg31-профилей, `"2"` — как сейчас, для awg1 — `"1.0"`/опустить. Поле информационное: клиент 5.x детектит версию по маркерам (наличие 3.x-ключей / S3-S4 / H-дефисов / I*), но выставляем честно.
3. В `config` внутри last_config (текстовый conf) — полный клиентский conf из этапа 3, включая `PersistentKeepalive = 25-35`.

## Этап 5 — API и UI

1. `handlers_cabinet.go`: `Preset` принимает `awg1|awg2|awg31` (+ легаси-алиасы). Дефолт при пустом — решение ниже (см. Риски).
2. `CabinetView.vue`: PRESETS → три карточки:
   - **AWG 3.1** — «Максимальная защита. Требуется свежее приложение Amnezia» (recommended);
   - **AWG 2.0** — «Совместимость. Работает со всеми версиями приложений»;
   - **AWG 1.0** — «Роутеры и старые клиенты (Keenetic/OpenWrt/GL.iNet, kernel-модуль 1.0)».
3. Удалить `web/src/utils/generator.ts` и связанные типы; `npm run build` — гейт.
4. Админка: показать поколение профиля (бейдж из детекции по маркерам) в списках/деталях; новые поля — read-only в деталях профиля. HeaderProtectionKey в UI маскировать как секрет.
5. `docs/API.md`: обновить описание preset.

## Этап 6 — тесты

1. `obfuscation_gen_test.go`: три пресета проходят Validate; awg31 — S1–S4 ≥ 12, H=1..4, HPK непустой; awg1 — без S3/S4/I/3.x; алиасы auto/stealth/fast → awg2.
2. `parse_test.go`: сниппет с 3.x-ключами парсится; Itime/J1–J3 принимаются и игнорируются; HPK+S<12 → ошибка; on/off; u16-границы диапазонов.
3. `config_render_test.go`: golden-рендер сервер/клиент для трёх пресетов; awg2-профиль рендерится байт-в-байт как до изменений.
4. `amneziaurl_test.go`: 3.x-ключи в awg{}/last_config; protocol_version; awg2-экспорт не изменился.
5. Загрузка state v4 со старыми полями j1/itime → ok; state v6 → fail fast.
6. CI: `go vet ./...`, `go build -v ./...`, `go test ./... -count=1`, `npm run build`.

## Этап 7 — деплой и живая проверка

1. Rebuild образа (новые пины) → рестарт. Существующие профили: conf не перерендерился с новыми ключами → wire-поведение прежнее. Проверить handshake текущего клиента до любых изменений кода (изолировать риск апгрейда ядра).
2. Создать awg31-устройство через кабинет → импорт vpn:// в официальный клиент 5.x → handshake, rx/tx.
3. Импорт того же vpn:// в amneziette (после её собственного плана AWG31_PLAN.md — там сейчас 3.x-ключи вырезаются при импорте).
4. Создать awg1-устройство → проверить на роутере/старом клиенте, если есть под рукой.
5. Бэкап/restore WG_PATH со schema v5.

## Порядок

Этап 0 (изолированно, отдельный деплой) → 1 → 2 → 3 → 4 (ядро, один PR) → 5 (UI) → 6 → 7.

## Риски и решения

- **Дефолтный пресет кабинета.** awg31 у подписчиков со старыми приложениями Amnezia (< 5.x) не подключится. Решение: дефолт `awg2` на первые недели (нулевой риск), карточка awg31 — «recommended»; переключение дефолта на awg31 отдельным коммитом после обкатки. Опционально `WG_DEFAULT_PRESET` в env.
- **Апгрейд ядра v0.2.18 → v3.1** меняет data plane для всех существующих профилей. Митигация: этап 0 деплоится и проверяется отдельно, до кодовых изменений.
- **Downgrade-коррупция state** (v5 → старый бинарь молча теряет 3.x-поля) — закрывается верхней границей SchemaVersion (этап 1).
- **u16-диапазоны tools**: значения диапазонов > 65535 tools не примут — ловим в Validate, а не на awg-quick.
