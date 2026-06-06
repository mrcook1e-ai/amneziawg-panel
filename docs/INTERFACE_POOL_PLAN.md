# Interface Pool — миграция на AWG 2.0 с обратной совместимостью

**Статус:** план готов, ждёт ответов на 5 вопросов в конце документа.
**Стартовая точка:** `HEAD = b01ee62 Add factory-reset action` + Dockerfile/Up-retry (PR `66ec682`) — 2.0-capable userspace уже в контейнере, панель сама всё ещё AWG 1.0.
**Гринфилд:** клиентов на проде нет, миграционный код не нужен — clean break.

---

## Что меняем по существу

Сейчас панель — обёртка над **одним** `awg0` интерфейсом. После переезда — над **пулом** `awg0..awgN`, каждый со своим:

- UDP-портом (51820, 51821, …)
- keypair'ом сервера
- набором обфускации (Jc/Jmin/Jmax/S1/S2/H1-H4) + опционально I1..I5
- списком клиентов

Клиент привязан к одному профилю и наследует от него endpoint (host:port), server-pubkey, обфускацию. На карточке клиента — селект «Профиль».

Совместимость по поколениям AWG достигается естественно: на одном профиле I1..I5 пустые → стоковые 1.0-клиенты работают. На другом — заполнены строками CPS из внешнего генератора → подключаются только 1.5+.

---

## 1. Что есть сейчас (что переделываем)

### Backend `internal/awg/`
- `config.go::Server` — единственная server-структура (privkey, IP, Jc/Jmin/Jmax/S1/S2/H1-H4). **Превратим в `Profile`** и положим в map.
- `config.go::Config` — `{Server, Clients}`. **Станет `{Profiles map[string]*Profile, Clients map[string]*Client}`**.
- `manager.go::Manager` — один `cur *Config`, один `runner Runner`, один `store`. **Превратим в пул `profiles map[string]*profileState`**.
- `store.go` — пишет `awg0.json` + `awg0.conf`. **Перейдём на единый `state.json` (все профили) + per-profile `awgN.conf`**.
- `ipam.go` — выдаёт IP из общей `10.8.0.x`. **Оставляем общий пул** — IP уникален поверх всех профилей.
- `sync.go::Runner` — один Iface. **Инстанцируется на каждый профиль**.
- `amneziavpn.go::RenderAmneziaVPN` — endpoint от server. **Берёт port из конкретного профиля**.

### Backend `internal/api/`
- `handlers.go` — операции над «сервером»: regenerate-magic, restart, reset-clients. **Станут per-profile**: `POST /api/profiles/{id}/regenerate-magic`.
- `handlers.go::clientCreate` — требует `profileId` в body.
- `router.go` — добавятся routes под `/api/profiles/`.
- `stream.go::Broker` — `awg show dump` сейчас на одной фьеxе. **Объединяем дампы по всем интерфейсам**.

### Backend `internal/stats/`
- Collector сейчас опрашивает один интерфейс. **Опрашивает все, реконсилит peers поверх**.
- `peer_samples` таблица не меняется — keyed by `peer pubkey`, pubkey уникален поверх системы.

### Frontend
- `types.ts::ServerInfo` → `ProfileInfo`, плюс `Profile[]`.
- `stores/clients.ts` — клиент получает `profileId`.
- Новая секция в Settings: «Профили подключения» — CRUD.
- `NewClientModal` — селектор «Профиль».
- `TopBar` — суммарная статистика по всем профилям (rxRate глобальный, как сейчас).

### Infra
- `docker-compose.yml` — пробросить диапазон портов: `51820-51829:51820-51829/udp`.
- iptables правила глобальные (MASQUERADE) → остаются.

---

## 2. Новая модель

### Profile

```go
type Profile struct {
    ID          string   `json:"id"`          // "default", "mimicry", "quic-rus" — admin slug
    Name        string   `json:"name"`        // человекочитаемое
    Iface       string   `json:"iface"`       // awg0, awg1, … — auto
    Port        int      `json:"port"`        // 51820, 51821, … — auto из range
    
    // Server identity per profile — каждый интерфейс это отдельный WG-сервер
    // со своим pubkey. Клиент видит уникальные (endpoint:port, server_pubkey)
    // на каждый профиль.
    PrivateKey string `json:"privateKey"`
    PublicKey  string `json:"publicKey"`
    
    // AWG 1.0 параметры (всегда заполнены)
    Jc, Jmin, Jmax int
    S1, S2         int
    H1, H2, H3, H4 string
    
    // AWG 1.5 параметры (опционально, opaque text). Пользователь вставляет
    // CPS-строки из внешнего генератора (например, AmneziaWG-Architect).
    // Панель НЕ генерирует — только хранит и рендерит в conf.
    I1, I2, I3, I4, I5 string `json:",omitempty"`
    
    Description string    `json:"description,omitempty"`
    CreatedAt   time.Time `json:"createdAt"`
}
```

### Client

```go
type Client struct {
    // … всё что было
    ProfileID string `json:"profileId"`  // обязательное; миграции нет, так что
                                          // bootstrap создаёт default-профиль
                                          // и все новые клиенты валидируются
}
```

### Manager

```go
type Manager struct {
    cfg config.Config
    
    mu       sync.Mutex
    profiles map[string]*profileState  // ID -> state
    clients  map[string]*Client         // ID -> client (плоский поверх профилей)
    ipam     *IPAM                      // общий IP-пул 10.8.0.x
    portIPAM *PortIPAM                  // выдаёт следующий свободный UDP-порт
    keys     Keys
}

type profileState struct {
    profile *Profile
    runner  Runner
}
```

---

## 3. План по фазам

### Фаза 0 — снос текущего стейта (5 минут)
На проде по SSH:
```bash
ssh root@155.212.226.85 'rm /var/lib/docker/volumes/vpn-panel-bojlcp_amnezia-state/_data/awg*.{json,conf} 2>/dev/null; sqlite3 /var/lib/docker/volumes/vpn-panel-bojlcp_amnezia-state/_data/panel.db "DELETE FROM peer_samples; DELETE FROM peer_daily; DELETE FROM events;" 2>/dev/null'
```
Или просто factory-reset через UI и удалить `awg0.*` руками.

### Фаза 1 — модель + Port IPAM (1.5 часа)
- `internal/awg/profile.go` — новый файл с `Profile` структурой.
- `internal/awg/config.go::Config` — `{Profiles, Clients}`. Снести `Server`.
- `internal/awg/portipam.go` — `PortIPAM{rangeStart, rangeEnd}` + `Next(used)` + `IfaceFor(port)` (детерминированный мап `port-rangeStart → awgN`).
- `Client.ProfileID` — добавить поле.

### Фаза 2 — Manager как пул (3 часа)
- Переписать `Manager` под `profiles map`.
- `Start()` — поднять все профили по очереди (Up для каждого).
- `Shutdown()` — пройтись по всем.
- `CreateProfile(spec) → Profile`, `DeleteProfile(id) → error` (блокировать если есть клиенты).
- `CreateClient(name, profileId)` — добавляет в общий map, рендерит conf нужного профиля.
- `MoveClient(clientID, toProfileID)` — atomic: убирает из старого conf, добавляет в новый, рендерит оба.
- `ListClients()` — плоский список с `profileId` в каждом.
- `ProfileInfo(id) → ProfileView`, `ListProfiles() → []ProfileView`.
- `RegenerateMagic(profileID)`, `RestartInterface(profileID)`.
- `bootstrap()` — создаёт первый профиль `default` на 51820 без I-полей.

### Фаза 3 — Storage (1 час)
- `store.go` — single-file `state.json` с {profiles, clients, schemaVersion: 2}.
- При загрузке если файла нет — bootstrap дефолтного профиля.
- При сохранении: пишет `state.json`, потом N `awgN.conf` файлов рядом.
- Старый код для `awg0.json` снести.

### Фаза 4 — Render (1 час)
- `RenderProfile(profile, peersForProfile) → conf` — peers фильтруются по `client.ProfileID == profile.ID`.
- `RenderClient(profile, client) → conf` — endpoint из profile (host из cfg, port из profile.Port).
- `RenderAmneziaVPN(profile, client) → vpn://`.
- I1..I5 эмитятся в conf и в .vpn если непустые.

### Фаза 5 — API (1.5 часа)
- `GET /api/profiles/` — список.
- `POST /api/profiles/` — создать `{id, name, description, i1?, i2?, i3?, i4?, i5?}`. Port и iface auto-allocated.
- `GET /api/profiles/{id}` — деталь (server pubkey, endpoint, params).
- `PATCH /api/profiles/{id}` — поменять I-поля, описание, name.
- `DELETE /api/profiles/{id}` — удалить (только если нет клиентов, и это не последний профиль).
- `POST /api/profiles/{id}/regenerate-magic` — пересоздать H1-H4 (опционально с новыми J/S).
- `POST /api/profiles/{id}/restart` — bounce интерфейса.
- `POST /api/clients/` — body `{name, profileId}`. profileId обязателен.
- `PATCH /api/clients/{id}/profile` — переезд клиента на другой профиль.

### Фаза 6 — Stats + SSE (45 минут)
- `stats/collector.go` — итерируется по `manager.IfaceNames()`, делает `awg show dump` для каждого, объединяет в одну таблицу.
- `Broker` — то же самое.
- События: `profile.created`, `profile.deleted`, `profile.regen_magic`, `client.moved`, plus existing client events.

### Фаза 7 — Frontend (3 часа)
- `types.ts` — `ProfileInfo`, `Client.profileId`.
- `lib/api.ts` — методы профилей.
- `stores/profiles.ts` — новый pinia store.
- `views/SettingsView.vue` — секция «Профили подключения»: таблица (имя, порт, мимикрия on/off, кол-во клиентов), кнопки «Добавить», «Редактировать», «Удалить», «Перегенерировать».
- Modal «Создать/редактировать профиль»: name, description, чекбокс «Мимикрия 1.5», 5 textarea для I1..I5 (опционально), линк-подсказка «Получить CPS-строки — AmneziaWG-Architect».
- `NewClientModal` — селект «Профиль» (default выбран, запоминается в `localStorage`).
- Карточка клиента — бейдж с именем профиля, кнопка «Переместить» в меню.
- Защита UX: нельзя удалить профиль с клиентами (toast «Сначала переместите клиентов»).

### Фаза 8 — Docker (15 мин)
- `docker-compose.yml` — `ports: ["51820-51829:51820-51829/udp", "51821:51821/tcp"]`.
- `.env.example` — `WG_PORT_RANGE_START=51820`, `WG_PORT_RANGE_END=51829`.
- `internal/config/config.go` — добавить `PortRangeStart, PortRangeEnd`.

### Фаза 9 — тесты + чистка (45 минут)
- Сценарий: создать default → клиент A → создать mimicry-профиль → клиент B → оба коннектятся.
- Move A → mimicry. Старый conf не имеет его, новый имеет. Без рестарта (`syncconf` если получится; иначе bounce).
- Delete профиль mimicry с клиентом B → 400 error.
- Backup/restore — обновить под новый формат `state.json`.

---

## 4. Открытые вопросы (нужны решения)

1. **Имена профилей** — admin-задаваемый slug (`mimicry-quic`) или auto (`profile-1, profile-2`)?
   - **Дефолт:** admin-задаваемый, валидируем `[a-z0-9-]{2,32}`.

2. **Удаление профиля с клиентами** — блокировать или каскадно удалять клиентов?
   - **Дефолт:** блокировать. Безопаснее, юзер видит сначала клиентов.

3. **Default-профиль защищать?** — UI не даёт удалить последний / именованный `default`?
   - **Дефолт:** да, защищён `if len(profiles) == 1`.

4. **CPS-строки I1..I5** — textarea для ручной вставки или встроить простой генератор?
   - **Дефолт:** textarea. Юзер вставляет из AmneziaWG-Architect. Прямая ссылка в подсказке.

5. **Подсеть** — общая `10.8.0.x` поверх профилей или подсеть на профиль (`10.8.{N}.x`)?
   - **Дефолт:** общая. Один источник IP, проще IPAM.

---

## 5. Оценка времени

| Фаза | Время |
|---|---|
| 0. Снос старого стейта | 5 мин |
| 1. Модель + Port IPAM | 1.5 ч |
| 2. Manager pool | 3 ч |
| 3. Storage | 1 ч |
| 4. Render | 1 ч |
| 5. API | 1.5 ч |
| 6. Stats/SSE | 45 мин |
| 7. Frontend | 3 ч |
| 8. Docker | 15 мин |
| 9. Тесты | 45 мин |
| **Итого** | **~12 часов** |

---

## 6. Что НЕ делаем

- ❌ Свой генератор CPS — overkill, оставляем юзеру внешний инструмент.
- ❌ Per-peer обфускацию — AWG её не поддерживает (issue #101 amneziawg-go).
- ❌ Per-profile подсети — пока не нужно, общая `10.8.0.x` хватит.
- ❌ Per-profile DNS/MTU/AllowedIPs — берём из глобального config.
- ❌ Миграцию старого `awg0.json` — гринфилд, clean break.

---

## 7. Что уже готово к этой работе

- 2.0-capable userspace (`amneziawg-go v0.2.18` + `amneziawg-tools v1.0.20260223`) в Dockerfile.
- Retry для userspace UAPI-race в `sync.go::Up()`.
- SSE broker + stats collector — будут адаптированы под пул, но базовая логика есть.
- IPAM с общим пулом — без изменений.
- Auth + healthz + factory-reset + backup/restore — переживут переезд почти без правок.

---

## 8. Точка входа для следующей сессии

Открыть этот файл. Ответить на 5 вопросов из секции 4 (или подтвердить дефолты). Запустить с Фазы 0.

Команда старта:
```bash
ssh root@155.212.226.85 'rm /var/lib/docker/volumes/vpn-panel-bojlcp_amnezia-state/_data/awg*.{json,conf} 2>/dev/null; sqlite3 /var/lib/docker/volumes/vpn-panel-bojlcp_amnezia-state/_data/panel.db "DELETE FROM peer_samples; DELETE FROM peer_daily; DELETE FROM events;"'
```

Затем Dokploy rebuild чтобы убедиться что 2.0-userspace поднимается чисто на пустом стейте. Дальше — Фаза 1.
