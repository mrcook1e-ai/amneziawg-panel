# AWG 3.0 / 3.1 — справка по совместимости (исследование 2026-08-29)

Источник: исходники amnezia-vpn/amneziawg-go (тег v3.1.20260828, HEAD master),
amneziawg-tools (master), amnezia-client (dev, 5.x), docs.amnezia.org.

## Линейка версий и ядра

| AWG | Ядро amneziawg-go | Что добавилось |
|-----|-------------------|----------------|
| 1.0 | v0.1.x | Jc/Jmin/Jmax, S1/S2, H1–H4 (фиксированные значения) |
| 1.5 (бета) | v0.2.13-beta-awg-1.5* | I1–I5 (CPS-пакеты) |
| 2.0 | v0.2.x (последний v0.2.19, 2026-06-17) | S3/S4, H1–H4 как диапазоны `lo-hi` |
| 3.0 | v3.0.0 (2026-07-24), модуль `github.com/amnezia-vpn/amneziawg-go/v3` | HeaderProtectionKey, ContentPaddingAddition, рандомизированные таймеры |
| 3.1 | v3.1.20260812+ (август 2026) | RandomTrailers, DisableCookies |

- Itime и J1–J3 из беты 1.5 **выпилены окончательно**: их нет ни в amneziawg-go
  (v0.2.19 и v3), ни в amneziawg-tools. Наше решение не эмитить их — верное навсегда.
- Официальный клиент Amnezia помечает всю линейку v3 строкой версии **"3.1"**.
- Kernel-модуль amneziawg-linux-kernel-module тоже получил 3.0 (2026-07-30) и 3.1 (2026-08-12).

## Новые параметры (все — уровень Interface/device, кроме PersistentKeepalive)

Формат диапазона везде: `lo` или `lo-hi` (uint32, hi >= lo).

| .conf ключ | UAPI ключ | Тип | Семантика |
|---|---|---|---|
| `HeaderProtectionKey` | `header_protection_key` (hex) | base64-ключ 32 байта (`awg genkey`) | ChaCha20-шифрование служебных заголовков. **Обязан совпадать на обеих сторонах** |
| `ContentPaddingAddition` | `content_padding_addition` | диапазон | Случайный паддинг внутри шифруемого payload transport-пакетов. Односторонний |
| `RekeyAfterTime` | `rekey_after_time` | диапазон, сек | Рандомизация времени до рекея. Локальное поведение |
| `RekeyTimeout` | `rekey_timeout` | диапазон, сек | Рандомизация ретрая handshake. Локальное |
| `RejectAfterTime` | `reject_after_time` | диапазон, сек | Локальное |
| `KeepaliveTimeout` | `keepalive_timeout` | диапазон, сек | Локальное |
| `MaxHandshakeAttempts` | `max_handshake_attempts` | диапазон, шт | Локальное |
| `RandomTrailers` | `random_trailers` (bool) | `on`/`off` в conf | Случайный хвост после handshake init/response/cookie-reply. **Обязан совпадать**: приёмник с off отбрасывает пакеты size > expected |
| `DisableCookies` | `disable_cookies` (bool) | `on`/`off` | Отключает cookie-reply/underload-механизм (серверная сторона) |
| `PersistentKeepalive` (Peer) | `persistent_keepalive_interval` | теперь диапазон `25-35` | Рандомизация keepalive-интервала |

Плюс peer-опция `AdvancedSecurity = on/off` в tools (маркер AWG для wg-совместимых конфигов).

## Механика HeaderProtectionKey

- ChaCha20 (unauthenticated), ключ 32 байта; **nonce = первые 12 байт случайного
  S-префикса пакета** → отсюда жёсткая валидация: при заданном ключе **S1–S4 ≥ 12**,
  иначе UAPI-set отклоняется ("S%d must be more then 12").
- Шифруется тело служебного сообщения (включая 4-байтовый тип): приёмник берёт
  nonce из начала датаграммы, снимает XOR и по S/H-таблице определяет тип.
- При включённом HPK прятать H1–H4 больше не нужно: дефолты клиента 5.x —
  **H1..H4 = 1,2,3,4 (стандартные WireGuard!)**, защита целиком на шифровании.
- H1–H4 (диапазоны) не должны пересекаться — валидация "headers must not overlap".

## Обратная совместимость ядра v3

Ядро v3 при незаданных новых параметрах на проводе **байт-в-байт ведёт себя как
2.0/1.0**: HPK нулевой → шифр не применяется; таймеры-диапазоны нулевые → фолбэк
на стандартные WG-константы; RandomTrailers off → приёмник требует точный размер,
как раньше. Т.е. один бинарь на v3 обслуживает конфиги всех трёх поколений.

Ломающие несовпадения (device-wide, обе стороны): S1–S4, H1–H4,
HeaderProtectionKey, RandomTrailers. Односторонние (можно per-client):
Jc/Jmin/Jmax, I1–I5, ContentPaddingAddition, таймеры, PersistentKeepalive.

Go-модуль сменил import path: `github.com/amnezia-vpn/amneziawg-go/v3` (go 1.25).
Старые `Line unrecognized` в tools: master tools уже принимает все новые ключи,
но **старые tools (≤ v1.0.2026xx AWG2-эры) падают на новых ключах** — эмитить их
в conf только когда реально заданы.

## Теги I1–I5 (актуальный набор в v3)

`<b 0xHEX>` статические байты, `<r N>` случайные байты, `<rc N>` случайные
буквы a-zA-Z, `<rd N>` случайные цифры, `<t>` UNIX timestamp 4 байта.

## Детекция версии официальным клиентом 5.x (важно для vpn:// экспорта)

Порядок проверки (awgProtocolConfig.cpp):
1. **v3 ("3.1")**: непуст любой из HeaderProtectionKey / ContentPaddingAddition /
   Rekey*/Reject*/Keepalive*/MaxHandshakeAttempts, ИЛИ RandomTrailers/DisableCookies == on.
2. **v2**: задан S3 или S4, ИЛИ любой H содержит `-` (диапазон).
3. **v1.5**: непуст любой I1–I5.
4. иначе v1.0.

Ключи в JSON (awg{} и last_config) теперь **совпадают с conf-ключами**: "Jc",
"S1".."S4", "H1".."H4", "I1".."I5", "HeaderProtectionKey", "ContentPaddingAddition",
"RekeyAfterTime", "RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout",
"MaxHandshakeAttempts", "RandomTrailers", "DisableCookies". Поле protocol_version —
информационное, реальная детекция по маркерам выше.

## Дефолтный профиль 3.1 официального клиента (референс для генератора панели)

- Jc = rand(4..6), Jmin = 10, Jmax = 50
- S1, S2 = rand(12..150); S3 = rand(12..64); S4 = 12 (все ≥ 12 из-за HPK-nonce; проверка уникальности и анти-коллизий размеров как раньше)
- H1..H4 = 1, 2, 3, 4 (стандартные WG — маскирует HPK)
- I1 = `<r 2><b 0x8580...3737>` (DNS-мимикрия), I2–I5 пустые
- HeaderProtectionKey = свежий genkey
- ContentPaddingAddition = 10-100, RekeyAfterTime = 100-120, RekeyTimeout = 3-7,
  RejectAfterTime = 150-180, KeepaliveTimeout = 5-15, MaxHandshakeAttempts = 15-20,
  PersistentKeepalive = 25-35, RandomTrailers = on, DisableCookies = on

## TODO для панели

1. Profile: + 9 полей (7 строк-диапазонов/ключ + 2 bool), PersistentKeepalive → строка-диапазон.
2. Валидации: S1–S4 ≥ 12 при заданном HPK; H-диапазоны без взаимных пересечений.
3. Шаблоны conf: эмитить новые ключи только когда заданы (совместимость со старыми tools/2.0-клиентами).
4. vpn:// (amneziaurl.go): добавить новые ключи в awg{} и last_config; protocol_version "3.1" при наличии маркеров v3, иначе оставить "2".
5. Деплой: amneziawg-go → /v3 (новый import path), amneziawg-tools → master.
