# SmartHome Bridge

Сервис-мост между MQTT и Telegram. Подписывается на топики MQTT-брокера, оценивает JSON-сообщения от датчиков по настраиваемым правилам и отправляет уведомления в Telegram.

## Возможности

- **MQTT** — любой брокер (Mosquitto, Zigbee2MQTT, Tasmota), схемы `tcp://`, `ssl://`, `ws://`, `wss://`
- **Правила** — фильтрация по topic (regex), сравнение полей JSON (`>`, `<`, `==`, `!=`, `>=`, `<=`)
- **AND/OR** — комбинирование нескольких условий в одном правиле
- **Временные окна** — срабатывание только в заданные часы с учётом часового пояса
- **Cooldown** — подавление повторных уведомлений для одного устройства
- **Шаблоны** — Go-шаблоны сообщений с доступом ко всем полям JSON
- **Групповые чаты** — разные `chat_id` для разных правил, системные уведомления в отдельный чат
- **Graceful shutdown** — корректное завершение по SIGINT/SIGTERM с доставкой оставшихся уведомлений

---

## Быстрый старт (локальная сборка)

### Требования

- Go 1.22+
- Git
- MQTT-брокер и Telegram-бот

### Сборка и запуск

```bash
git clone https://github.com/<user>/smarthome-bridge.git
cd smarthome-bridge
go mod download
go build -o bridge.exe ./cmd/bridge/
```

Создать конфиг:

```bash
cp config.example.yaml config.yaml
```

Отредактировать `config.yaml` — указать `mqtt.broker`, `telegram.bot_token`, `rules[].chat_ids`.

Запустить:

```bash
./bridge.exe -config config.yaml
# или с отладочным логированием:
./bridge.exe -config config.yaml -debug
```

---

## Деплой в Docker Compose

`setup.sh` автоматизирует раскладку файлов на сервере. Исходники и рабочая директория разделены: репозиторий клонируется в `src/`, файлы для запуска лежат на уровень выше.

### 1. Клонировать репозиторий

```bash
mkdir -p /opt/smarthome-bridge
cd /opt/smarthome-bridge
git clone https://github.com/<user>/smarthome-bridge.git src
cd src
```

### 2. Запустить setup

```bash
bash setup.sh
```

Скрипт создаст:

```
/opt/smarthome-bridge/
├── src -> smarthome-bridge/      ← симлинк на репозиторий
├── smarthome-bridge/             ← клонированный репозиторий (нетронутый)
├── compose.yml                   ← production docker-compose
├── config.yaml                   ← конфиг правил (из config.example.yaml)
└── .env                          ← переменные окружения
```

И в отдельном каталоге — секреты:

```
/opt/data/docker-secrets/
├── mqtt_password.txt             ← пароль MQTT (chmod 600)
└── telegram_bot_token.txt        ← токен бота (chmod 600)
```

### 3. Заполнить `.env`

```ini
# /opt/smarthome-bridge/.env
MQTT_BROKER=tcp://192.168.1.10:1883
MQTT_USERNAME=homebridge
ADMIN_CHAT_ID=123456789
```

| Переменная | Назначение |
|-----------|-----------|
| `MQTT_BROKER` | Адрес MQTT-брокера |
| `MQTT_USERNAME` | Логин MQTT (оставить пустым если нет авторизации) |
| `ADMIN_CHAT_ID` | Telegram chat ID для системных уведомлений (старт сервиса). Можно несколько через запятую: `12345,67890` |

### 4. Заполнить секреты

```bash
# Пароль MQTT
nano /opt/data/docker-secrets/mqtt_password.txt

# Токен Telegram-бота (получить у @BotFather)
nano /opt/data/docker-secrets/telegram_bot_token.txt
```

### 5. Настроить правила

Отредактировать `/opt/smarthome-bridge/config.yaml` — прописать `chat_ids` и правила под свои датчики. Подробнее в разделе [Правила](#правила).

### 6. Запустить

```bash
cd /opt/smarthome-bridge
docker compose up -d
docker compose logs -f
```

### Обновление

```bash
cd /opt/smarthome-bridge
git -C src pull
docker compose up -d --build
```

Если `VERSION` в репозитории изменился — `compose.yml` и `config.yaml` перезапишутся при следующем `setup.sh`. Сделайте резервную копию `config.yaml` перед обновлением.

### Где лежат секреты в контейнере

| Файл на хосте | Путь в контейнере | Env var |
|---|---|---|
| `secrets/mqtt_password.txt` | `/run/secrets/mqtt_password` | `MQTT_PASSWORD_FILE` |
| `secrets/telegram_bot_token.txt` | `/run/secrets/telegram_bot_token` | `TELEGRAM_BOT_TOKEN_FILE` |

---

## Конфигурация

Все параметры в `config.yaml`. Значения `broker`, `username`, `password`, `bot_token` можно оставить пустыми — в Docker они подтягиваются из `.env` и `/run/secrets/*`. При локальном запуске заполняются напрямую в YAML.

### `mqtt`

| Параметр | Тип | По умолчанию | Описание |
|---------|-----|-------------|----------|
| `broker` | string | — | Адрес брокера: `tcp://host:1883`, `ssl://host:8883`, `ws://`, `wss://` |
| `client_id` | string | `smarthome-bridge-<hostname>-<pid>` | Идентификатор клиента MQTT |
| `username` | string | `""` | Логин (опционально) |
| `password` | string | `""` | Пароль (опционально) |
| `keep_alive_sec` | int | `60` | Keepalive-интервал |
| `connect_timeout_sec` | int | `30` | Таймаут подключения |
| `ping_timeout_sec` | int | `10` | Таймаут одного ping |
| `subscriptions` | list | — | Список подписок MQTT |

#### `subscriptions[]`

| Параметр | Тип | Описание |
|---------|-----|----------|
| `topic` | string | MQTT-топик (можно с wildcard: `#`, `+`) |
| `qos` | int | Quality of Service: `0`, `1` или `2` |

### `telegram`

| Параметр | Тип | По умолчанию | Описание |
|---------|-----|-------------|----------|
| `bot_token` | string | — | Токен бота из @BotFather |
| `api_base_url` | string | `https://api.telegram.org` | Базовый URL Telegram API |
| `retry_max` | int | `3` | Количество повторных попыток |
| `retry_backoff_ms` | list | `[200, 600, 1800]` | Задержки перед каждой попыткой (мс) |

### `engine`

| Параметр | Тип | По умолчанию | Описание |
|---------|-----|-------------|----------|
| `worker_count` | int | `4` | Количество goroutine-обработчиков |
| `inbound_queue_size` | int | `256` | Буфер входящих MQTT-сообщений |
| `notify_queue_size` | int | `64` | Буфер исходящих уведомлений |

---

## Правила

Правило описывает: **на какие сообщения реагировать** (topic, условия), **когда** (временное окно, cooldown) и **что отправить** (чат, шаблон).

### Параметры правила

| Параметр | Тип | Обязательно | Описание |
|---------|-----|:----------:|----------|
| `name` | string | да | Уникальное имя правила |
| `description` | string | нет | Описание (для документации) |
| `enabled` | bool | нет | `true`/`false`, по умолчанию `true` |
| `topic_filter` | string | нет | Regex для фильтрации MQTT-топика (например `^zigbee2mqtt/.+`) |
| `device_key_source` | string | нет | Источник ключа устройства для cooldown: `topic`, `field`, `rule` |
| `device_key_path` | string | нет | gjson-путь к полю в JSON, если `device_key_source: field` |
| `conditions` | list | да | Список условий (минимум одно) |
| `condition_logic` | string | нет | Логика комбинирования: `and` или `or` (по умолчанию `and`) |
| `time_window` | object | нет | Временной интервал срабатывания |
| `cooldown_minutes` | int | нет | Минимальный интервал между уведомлениями для одного устройства |
| `cooldown_on_startup` | bool | нет | Подавлять уведомления о всех устройствах при первом старте |
| `chat_ids` | list | да | Telegram chat ID (можно несколько) |
| `message_template` | string | да | Go-шаблон сообщения |

### Параметры условия (`conditions[]`)

| Параметр | Тип | Описание |
|---------|-----|----------|
| `field_path` | string | gjson-путь к полю в JSON (например `battery`, `contact`, `temperature`) |
| `operator` | string | Оператор: `eq`, `ne`, `gt`, `gte`, `lt`, `lte` |
| `value` | any | Пороговое значение |
| `value_type` | string | Тип значения: `number`, `string`, `boolean` |

### Параметры временного окна (`time_window`)

| Параметр | Тип | Описание |
|---------|-----|----------|
| `start` | string | Начало окна, `HH:MM` |
| `end` | string | Конец окна, `HH:MM`. Если `end < start` — окно считается ночным (например `22:00`–`06:00`) |
| `timezone` | string | Часовой пояс (например `Europe/Moscow`), по умолчанию `Local` |

### Переменные шаблона

| Переменная | Тип | Описание |
|-----------|-----|----------|
| `{{ .RuleName }}` | string | Имя сработавшего правила |
| `{{ .Description }}` | string | Описание правила |
| `{{ .Device }}` | string | Идентификатор устройства |
| `{{ .Topic }}` | string | MQTT-топик сообщения |
| `{{ .Fields.<поле> }}` | any | Значение поля из условия (например `.Fields.temperature`) |
| `{{ .Payload }}` | string | Полный JSON сообщения |
| `{{ .Time }}` | time.Time | Время срабатывания |
| `{{ formatTime .Time "<layout>" }}` | string | Форматированное время (layout в стиле Go: `15:04:05 02.01.2006`) |

### Функции шаблонов

| Функция | Описание |
|--------|----------|
| `escapeHTML` | Экранирование HTML-символов |
| `formatTime` | Форматирование времени, принимает layout |

### Примеры

Полные примеры правил — в [config.example.yaml](config.example.yaml).

### Синтаксис MQTT-топиков

В `subscriptions[].topic` используются wildcards MQTT:

| Символ | Значение | Пример |
|--------|---------|--------|
| `#` | Всё на любом уровне ниже | `zigbee2mqtt/#` — все подтопики |
| `+` | Один уровень | `zigbee2mqtt/+/state` — state любого устройства |

Подробнее:
- [MQTT Topics & Wildcards (HiveMQ)](https://www.hivemq.com/blog/mqtt-essentials-part-5-mqtt-topics-best-practices/)
- [MQTT 3.1.1 Specification §4.7](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html#_Toc398718110)

---

## Структура проекта

```
smarthome-bridge/
├── cmd/bridge/main.go              # Точка входа
├── internal/
│   ├── config/config.go            # Парсинг и валидация YAML, env/secrets override
│   ├── mqtt/subscriber.go          # MQTT-клиент (paho wrapper)
│   ├── engine/
│   │   ├── engine.go               # Оркестратор: каналы, пул воркеров
│   │   ├── evaluator.go            # Оценка правил: topic, условия, окна, cooldown
│   │   ├── condition.go            # Вычисление одного условия (gjson + сравнение)
│   │   ├── debounce.go             # Трекер cooldown с фоновой очисткой
│   │   └── template.go             # Кэш и рендеринг Go-шаблонов
│   ├── notifier/
│   │   ├── notifier.go             # Интерфейс Notifier
│   │   └── telegram.go             # HTTP-клиент Telegram Bot API с retry+backoff
│   └── app/app.go                  # Сборка компонентов, graceful shutdown
├── docker-compose.yml              # Локальная разработка
├── docker-compose.prod.yml         # Production-шаблон для setup.sh
├── setup.sh                        # Разворачивание на сервере
├── VERSION                         # Версия конфигурации деплоя
├── config.example.yaml             # Пример конфига с пояснениями
├── Makefile
└── README.md
```

## Зависимости

| Библиотека | Назначение |
|-----------|-----------|
| `eclipse/paho.mqtt.golang` | MQTT-клиент с авто-переподключением |
| `tidwall/gjson` | Быстрый доступ к полям JSON по dot-path |
| `gopkg.in/yaml.v3` | Парсинг YAML |

Остальное — стандартная библиотека Go (`net/http`, `log/slog`, `text/template`, `sync`, `context`, `os`, `regexp`).

## Лицензия

MIT
