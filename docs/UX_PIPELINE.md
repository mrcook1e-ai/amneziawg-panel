# UX/UI Pipeline — итерация после ревью 2026-06-07

Источник: ручной обход прод-инстанса `vpn.4ch.me` + аудит исходников
(`web/src/views/*`, `components/organisms/TopBar.vue`, `utils/generator.ts`).

Каждый пункт: **где** → **что не так** → **как чиним** → **затронутые файлы**.
Сортировка по эффекту на пользователя, не по сложности.

---

## P0 — Видимые баги и дубли

### 1. Дубли кнопки «Добавить клиента» в админке
**Где:** `/` (главная клиентов).
**Сейчас:** `openCreate()` вызывается из **трёх** мест одновременно —
- `TopBar #actions` → `Новый клиент` ([ClientsView.vue:134](web/src/views/ClientsView.vue:134))
- секционный заголовок «Все клиенты» → `+ Добавить` ([ClientsView.vue:190](web/src/views/ClientsView.vue:190))
- empty-state CTA ([ClientsView.vue:226](web/src/views/ClientsView.vue:226))

Когда список непустой, видны **две одинаковые** кнопки одновременно — в шапке и в секции. Это и есть «дублируются».

**Фикс:** оставить **только** действие в `TopBar` (это глобальный primary action на странице). Из секционного заголовка убрать — там оставить только счётчик `· N/M`. Empty-state CTA сохранить (он показывается, только когда список пуст).

**Файлы:** `web/src/views/ClientsView.vue` (удалить блок 190–192).

---

### 2. QR в кабинете — на главной странице корректный, в мастере «нового устройства» — нет
**Где:** `/cabinet/{token}` → wizard `done` step vs карточка устройства → `openQr()`.
**Сейчас:** В step `done` (после создания устройства) рендерится **одиночный** статичный QR через `amneziaQr(deviceId)` ([CabinetView.vue:740](web/src/views/CabinetView.vue:740)) — это ссылка на эндпоинт, который отдаёт **один** PNG. Этот QR Amnezia-приложение часто не сканирует, потому что реальный AmneziaWG-конфиг с тяжёлым обфускейтом (S1+S2+I-tags) **не помещается в один QR** на разумной плотности — нужен chunked carousel.

Карточки устройств на главной странице кабинета используют `openQr()` → `cabinetDeviceAmneziaQrChunks()` ([CabinetView.vue:93](web/src/views/CabinetView.vue:93)) — она тянет **массив** chunks и рендерит карусель с переключением каждые 2.5с. Это работает.

**Фикс:** В wizard step `done` заменить статичный `<img :src="amneziaQr(...)">` на тот же chunked-carousel компонент, что используется в `openQr()`. Логику показа вынести в отдельный `<QrCarousel>` molecule, чтобы использовалась в обоих местах.

**Файлы:**
- новый: `web/src/components/molecules/QrCarousel.vue` (извлечь из CabinetView.vue:512–602)
- `web/src/views/CabinetView.vue` (заменить блок 737–749 на `<QrCarousel :device-id="justAdded.deviceId" />`).

---

### 3. Кнопка переключения темы в кабинете «висит в воздухе»
**Где:** `/cabinet/{token}`, верхний правый угол.
**Сейчас:** `absolute top-4 right-5` ([CabinetView.vue:311](web/src/views/CabinetView.vue:311)) — кнопка не привязана ни к шапке, ни к карточке, болтается над scroll-container. Визуально читается как «забытый элемент».

**Фикс:** Завернуть в мини-хедер вместе с тонкой полоской-брендом «AmneziaVPN · Личный кабинет». Сейчас бренд-чип отрисован в центре `<header>` ([CabinetView.vue:322](web/src/views/CabinetView.vue:322)), а toggle отдельно. Объединить в одну верхнюю строку:
```
[Shield · AmneziaVPN]            [☀/☾]
```
…после которой большой заголовок с именем подписки. Toggle получает якорь и перестаёт «летать».

**Файлы:** `web/src/views/CabinetView.vue` (перенос блока 311–319 внутрь `<header>` 322 как `flex justify-between`).

---

### 4. SubscriberDetailView — «изменить заметку» когда заметка уже есть
**Где:** `/subscribers/{id}`, под заголовком.
**Сейчас:** Если у подписки есть `notes`, рядом с текстом всегда висит кнопка `<edit> изменить заметку` ([SubscriberDetailView.vue:240](web/src/views/SubscriberDetailView.vue:240)). Дублирует — текст и так кликабельный должен быть; кнопка визуально шумит.

**Фикс:** Сделать сам блок `<p>{{ sub.notes }}</p>` кликабельным (по клику → `editingNotes = true`). Кнопка «изменить заметку» остаётся **только** когда `notes` пустые (вариант `добавить заметку`).
```vue
<p v-if="sub.notes"
   class="text-[13.5px] text-ink-500 hover:text-ink-900 cursor-text"
   @click="startEditNotes">{{ sub.notes }}</p>
<button v-else @click="startEditNotes" class="eyebrow ...">
  <Icon name="edit" :size="12" /> добавить заметку
</button>
```
Hover-стиль (`text-ink-700` + лёгкий underline через `decoration-dotted`) даёт affordance без визуального шума.

**Файлы:** `web/src/views/SubscriberDetailView.vue` (231–242).

---

## P1 — Архитектурные неточности

### 5. Раздельное туннелирование должно жить в кабинете клиента, не у админа
**Где:** сейчас `SplitTunnelPicker` смонтирован в `/clients/{id}` (admin) ([ClientDetailView.vue:385](web/src/views/ClientDetailView.vue:385)). В `/cabinet/{token}` — отсутствует.

**Проблема логики:** Админ не знает, какие сервисы пользователю нужно пускать через VPN. Это выбор владельца устройства. Сейчас:
- Админ зачем-то выбирает Telegram/YouTube/Discord за пользователя.
- Пользователь не имеет влияния на `AllowedIPs` в собственном конфиге.

**Фикс — двойное перемещение:**

**5a. В кабинете** (`/cabinet/{token}`):
Добавить **на карточку устройства** действие «Настроить трафик» (раскрывается раздвижная панель с `SplitTunnelPicker`). После выбора сервисов кнопка **скачать новый .vpn / обновить QR** — мы **не** ремонтируем тоннель серверной стороной, мы просто **отдаём другой файл** конфига с подмешанным `AllowedIPs`. Пользователь сам импортирует в Amnezia-приложение поверх старого профиля.

Эндпоинт уже есть (`api.cabinetDeviceAmneziaVpnUrl`) — нужен query-параметр `?allowedIPs=...` или новый POST-эндпоинт «выдать конфиг с этой маской». Серверной стороне это **не** требует менять peer'а — переопределение **только** в выдаваемом файле.

**5b. У админа** (`/clients/{id}`):
`SplitTunnelPicker` оставить, но переименовать секцию в **«Шаблон AllowedIPs»** с подписью «применяется к новым конфигам по умолчанию; пользователь может переопределить из своего кабинета». Это превращает админский пресет в дефолт, а не в принудительный.

**Файлы:**
- `web/src/views/CabinetView.vue` — новая под-панель карточки.
- `web/src/views/ClientDetailView.vue` — переписать footer секции «Allowed IPs» (385–389).
- backend: добавить query-param к `/cabinet/:token/devices/:id/amnezia.vpn` (отдельная задача).

---

### 6. «Идентификация» в ClientDetailView — урезанный список параметров
**Где:** `/clients/{id}` → секция «Идентификация» ([ClientDetailView.vue:347–355](web/src/views/ClientDetailView.vue:347)).

**Сейчас показано:** IP, Public key, дата добавления. **Не хватает:**
- Профиль (interface) — где живёт peer.
- Listen endpoint (host:port) — что клиент видит в `Endpoint =`.
- Preshared key — present/absent (без значения).
- Persistent keepalive — текущее значение.
- Последний реальный handshake (точное время, не «10 мин назад»).
- IPv6-адрес если есть.
- Подписка-владелец (сейчас в breadcrumb, но не в секции).

**Фикс:** Расширить блок «Идентификация» до полного набора, разбить на два саб-блока «Сетевые параметры» / «Криптография».

**Файлы:** `web/src/views/ClientDetailView.vue` (347–355).

---

### 7. Секция «Настройки» на странице *клиента* — концептуально неверная
**Где:** `/clients/{id}` → «Настройки» ([ClientDetailView.vue:358–399](web/src/views/ClientDetailView.vue:358)).

**Сейчас:** В этом блоке смешано — description (заметка про устройство), expiresAt (срок), DNS/AllowedIPs/MTU override (сетевые параметры), и SplitTunnelPicker. Это рудимент времени, когда клиент == подписка.

**После рефактора подписка/клиент:**
- **Описание** относится к подписке (subscriber), не к устройству. → переместить на `SubscriberDetailView`.
- **Срок действия** — спорно. Сейчас на per-client уровне. Если подписка одна на пользователя — должно быть на subscriber. Уточнить у backend-семантики; вероятно перенести.
- **DNS / MTU override** — это уже свойство интерфейса (профиля), а не клиента. Должны жить в `/settings` → ProfileModal. На уровне клиента имеет смысл только **AllowedIPs override** (см. п.5).
- **SplitTunnelPicker** → см. п.5, переезжает в кабинет.

**Фикс:** Полностью убрать секцию «Настройки» с `/clients/{id}`. Оставить минимум:
- AllowedIPs override (как шаблон для конфига, см. п.5b)
- Описание устройства, если хочется отличать «iPhone Vasya» от «MacBook Vasya» — это уже **device-level metadata**, не subscriber-level. Можно оставить отдельным полем рядом с именем.

**Файлы:** `web/src/views/ClientDetailView.vue` (358–399 — сократить), `web/src/views/SubscriberDetailView.vue` (добавить «Описание»), `web/src/views/SettingsView.vue` (DNS/MTU в Profile).

---

### 8. «Доступ» — форматы и модалки нужно унифицировать с кабинетом
**Где:** `/clients/{id}` → «Доступ» ([ClientDetailView.vue:418–428](web/src/views/ClientDetailView.vue:418)).

**Сейчас:** В админке отдельные `QrModal` и `ConfigModal` (старые компоненты из `organisms/`). В кабинете — новый fullscreen-карусель с chunked QR и `.vpn`-кнопкой. UI разный, форматы тоже:
- admin: предлагает `.conf` (стандартный WireGuard) + `.vpn`
- cabinet: только `.vpn`

**Реальность:** Современный Amnezia-клиент работает только с `.vpn` (zip-конфигом с обфускейтом). `.conf` без S1/S2/I-tags не запустит AmneziaWG — это конфиг для **стандартного** WireGuard, который **не** пробьёт DPI. Admin-панель отдаёт `.conf` как технический артефакт для отладки, не для пользователей.

**Фикс:**
- Заменить `QrModal` админки на тот же `QrCarousel` (см. п.2).
- `ConfigModal` оставить, но переименовать кнопку в «Скачать (для отладки)» и спрятать за `<details>`. Основное действие — `.vpn`-файл (download-link, без модалки).
- В кабинете и админке использовать **общий** молекулярный `<DownloadActions :device-id>` (`.vpn` download + copy `vpn://` + open QR).

**Файлы:**
- новый: `web/src/components/molecules/DownloadActions.vue`
- `web/src/views/ClientDetailView.vue` (418–428)
- `web/src/views/CabinetView.vue` (449–488 — рефакторинг карточки)

---

## P2 — Функциональные дыры в кабинете

### 9. В кабинете нет генератора параметров
**Где:** `/cabinet/{token}` → wizard `createDevice()` ([CabinetView.vue:146–175](web/src/views/CabinetView.vue:146)).

**Сейчас:** `genCfg()` вызывается с **захардкоженными** параметрами:
```ts
genCfg({ version: '2.0', intensity: 'medium', profile: 'quic_initial',
         ... mimicAll: false, useTagC: false, useTagT: true, ... mtu: 1500,
         junkLevel: 5, iterCount: 0 })
```
Эти значения вшиты на все устройства всех подписок. Никакой пользовательской настройки.

`utils/generator.ts` существует (2106 строк), у него богатый API параметризации (intensity / profile / mimicry / junk levels / iterations / browser-fingerprint / router-mode). В админ-панели через `SettingsView` → ProfileModal эти параметры **доступны**. У пользователя — нет.

**Фикс — минимально:**
В wizard step `pick` добавить **раскрываемый** блок «Дополнительно» (закрыт по умолчанию):
- **Уровень обфускации**: `low` / `medium` / `high` / `extreme` (= `intensity`)
- **Маскировка под**: select из коротких пресетов (`quic_initial`, `http_2`, `webrtc`, `auto`) (= `profile`)
- **MTU**: input с дефолтом 1500
- галка «Усиленный режим (для строгих сетей)» = `intensity: 'extreme'` + `useExtremeMax: true`

Остальное оставить дефолтами. Сложные toggle (useTagC/useBrowserFp/iterCount) **не** показывать обычному пользователю — это admin-territory.

**Файлы:**
- `web/src/views/CabinetView.vue` (wizard `pick` step 645–698 — добавить collapse-блок).
- новый: `web/src/components/molecules/AdvancedCfgPicker.vue` (изолированный блок настроек, переиспользуется в админке).

---

### 10. В кабинете нет загрузки IPLIST для раздельного туннелирования
**Где:** `/cabinet/{token}`.
**Сейчас:** `SplitTunnelPicker` живёт только в админ-панели. Пользователь не может выбрать «через VPN только YouTube + Discord».

**Фикс:** Связано с п.5. Поведение:
1. На карточке устройства — кнопка `<Compass icon> Маршруты`.
2. Открывает sheet с `SplitTunnelPicker` + двумя радио-режимами:
   - **Весь трафик** (default, `AllowedIPs = 0.0.0.0/0`)
   - **Только выбранные сервисы** → показывается picker
3. Внизу: «Скачать обновлённый .vpn» / «Показать QR» — оба используют тот же эндпоинт с параметром `?allowedIPs=`.

**Файлы:** `web/src/views/CabinetView.vue`, `web/src/components/molecules/SplitTunnelPicker.vue` (уже есть, переиспользуем). Backend — добавить query-param на cabinet-endpoint.

---

## P3 — Полировка

### 11. На карточке устройства в кабинете три действия + удаление в один ряд
Сейчас 4 контрола (`QR`, `.vpn`, `copy`, `delete`) ([CabinetView.vue:448–489](web/src/views/CabinetView.vue:448)). На узких экранах (< 360 px) ряд переполняется. Объединить `copy` + `delete` в overflow-меню (`MoreHorizontal` → DropdownMenu).

### 12. `border-2 border-dashed` в empty-state кабинета и кнопке «Добавить устройство»
([CabinetView.vue:355](web/src/views/CabinetView.vue:355), [496](web/src/views/CabinetView.vue:496)) — dashed-border выбивается из общего стиля (мы убрали бордеры из `.card`/`.glass`). Заменить на лёгкий tinted-fill `bg-ink-100/60` + текст-инструкция. Сохранит «приглашающий» характер без визуального шума.

### 13. QR fullscreen в кабинете — собственный CSS вместо `.scrim`
([CabinetView.vue:521](web/src/views/CabinetView.vue:521)) — `style="background: rgba(0,0,0,0.94); backdrop-filter: blur(20px)"` инлайн. Заменить на класс `.scrim` с локальным override `--scrim-alpha: 0.94`. Получит keyframe-fade и will-change — лаг блюра, как у обычных модалок (если есть на этом overlay).

### 14. В админке `/clients/{id}` — Sparkline без оси Y
Удобно показывать пиковое значение и текущее в подписи под графиком (eyebrow-стилем). Сейчас «голый» график, цифры читаются только из карточек статов сверху.

### 15. SubscriberDetailView — поле «Описание»
После п.7 — добавить блок «Описание подписки» в шапку, наравне с заметкой. Notes = короткий тег (`@vasya`); Description = развёрнутый контекст («командировка в Грузию до 1 августа, отозвать»). Семантически разные поля.

---

## Порядок работ

| Этап | Состав | Эффект |
|---|---|---|
| **Sprint 1 (P0)** | 1, 2, 3, 4 | Видимые баги главной + кабинета. Юзеры замечают сразу. |
| **Sprint 2 (P1.5 + P2)** | 5a, 9, 10, 13 | Кабинет получает раздельное туннелирование, генератор и общий scrim. Это самый большой UX-сдвиг для конечного пользователя. |
| **Sprint 3 (P1)** | 6, 7, 8, 5b | Чистка `/clients/{id}` от рудиментов pre-subscriber-эпохи. Унификация модалок выдачи. Внутренняя гигиена админки. |
| **Sprint 4 (P3)** | 11, 12, 14, 15 | Полировка. |

---

## Открытые вопросы для backend

- **5a/10**: Эндпоинт `/cabinet/:token/devices/:id/amnezia.vpn?allowedIPs=...` — добавить query-параметр или сделать POST с body? POST чище (URL не светит IP-маски в access-log), но кэширование и `<a download>` теряются. Голос за **GET с query** + `Cache-Control: no-store`.
- **9**: Параметры генератора, выбранные пользователем — храним в device-row или каждый раз re-generate'им из per-device-stored params? Сейчас `device.snippet` — финальная строка; чтобы пересоздать с другими параметрами, нужно знать исходные. Предложение: добавить колонку `device.gen_params JSONB`.
- **7**: Куда переезжает `expiresAt` — subscriber или остаётся client-level? Зависит от семантики «срок» — это срок аккаунта (subscriber) или срок отдельного ключа на конкретный девайс (client)? Скорее первое.
