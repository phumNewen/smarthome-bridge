# SmartHome Bridge

Сетевой сервис на Go, который отслеживает сообщения от MQTT-датчиков умного дома и отправляет уведомления в Telegram по настраиваемым правилам.

## Возможности

- **MQTT-подписка** — подключение к любому MQTT-брокеру (Mosquitto, Zigbee2MQTT, Tasmota и др.)
- **Гибкие правила** — фильтрация по topic (regex), сравнение полей JSON (>, <, ==, !=, >=, <=)
- **Логика AND/OR** — комбинирование нескольких условий в одном правиле
- **Временные окна** — срабатывание только в заданные часы (с учётом часового пояса)
- **Защита от повторов** — настраиваемый cooldown для каждого устройства
- **Мульти-пользовательский Telegram** — разные chat_id для разных правил
- **Шаблоны сообщений** — Go-шаблоны с доступом ко всем полям датчика
- **Graceful shutdown** — корректное завершение при SIGINT/SIGTERM

## Быстрый старт

### 1. Установка

```bash
git clone https://github.com/your/smarthome-bridge.git
cd smarthome-bridge
go mod download
make build
```

### 2. Настройка

```bash
cp config.example.yaml config.yaml
# Отредактируйте config.yaml:
#   - mqtt.broker — адрес вашего MQTT-брокера
#   - telegram.bot_token — токен бота из @BotFather
#   - rules[].chat_ids — ваши Telegram chat ID
```

### 3. Запуск

```bash
make run
# или
./bin/bridge -config config.yaml -debug
```

## Формат конфигурации

```yaml
mqtt:
  broker: "tcp://localhost:1883"
  subscriptions:
    - topic: "zigbee2mqtt/#"
      qos: 1

telegram:
  bot_token: "123456:ABC-DEF..."

rules:
  - name: "high_temp"
    enabled: true
    topic_filter: "^zigbee2mqtt/.+"    # regex на topic MQTT
    conditions:
      - field_path: "temperature"       # gjson путь в JSON-нагрузке
        operator: "gt"                  # eq, ne, gt, gte, lt, lte
        value: 30
        value_type: "number"            # number, string, boolean
    condition_logic: "and"              # and | or
    time_window:                        # опционально
      start: "07:00"
      end: "23:00"
      timezone: "Europe/Moscow"
    cooldown_minutes: 30
    chat_ids: [123456789]
    message_template: |
      🌡️ Температура {{ .Fields.temperature }}°C
      Устройство: {{ .Device }}
```

### Переменные шаблона сообщений

| Переменная | Описание |
|---|---|
| `{{ .RuleName }}` | Название сработавшего правила |
| `{{ .Description }}` | Описание правила |
| `{{ .Device }}` | Идентификатор устройства |
| `{{ .Topic }}` | MQTT topic сообщения |
| `{{ .Fields.имя_поля }}` | Значения полей из условий |
| `{{ .Payload }}` | Полный JSON-текст сообщения |
| `{{ .Time }}` | Время срабатывания (time.Time) |
| `{{ formatTime .Time "15:04:05" }}` | Форматированное время |

### Функции шаблонов

- `escapeHTML` — экранирование HTML-символов
- `formatTime` — форматирование времени (принимает layout)

## Структура проекта

```
smarthome-bridge/
├── cmd/bridge/main.go            # Точка входа
├── internal/
│   ├── config/config.go          # Парсинг и валидация YAML-конфигурации
│   ├── mqtt/subscriber.go        # MQTT-клиент (paho wrapper)
│   ├── engine/
│   │   ├── engine.go             # Оркестратор: каналы, пул воркеров
│   │   ├── evaluator.go          # Оценка правил: topic, условия, окна
│   │   ├── condition.go          # Вычисление одного условия
│   │   ├── debounce.go           # Трекер cooldown-периодов
│   │   └── template.go           # Кэш и рендеринг шаблонов
│   ├── notifier/
│   │   ├── notifier.go           # Интерфейс Notifier
│   │   └── telegram.go           # HTTP-клиент Telegram Bot API
│   └── app/app.go                # Сборка компонентов, graceful shutdown
├── config.example.yaml
├── Makefile
└── README.md
```

## Зависимости

| Библиотека | Назначение |
|---|---|
| `paho.mqtt.golang` | MQTT-клиент с авто-переподключением |
| `tidwall/gjson` | Быстрое извлечение полей из JSON (dot-path) |
| `gopkg.in/yaml.v3` | Парсинг YAML-конфигурации |

Всё остальное — стандартная библиотека Go (`net/http`, `log/slog`, `text/template`, `sync`, `context`).

## Makefile

```bash
make build       # Сборка бинарника
make test        # Запуск тестов с race detector
make test-cover  # Тесты + coverage report
make lint        # Линтер (golangci-lint)
make run         # Сборка и запуск
make clean       # Очистка артефактов
```

## Лицензия

MIT
