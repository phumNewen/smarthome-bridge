# smarthome-bridge

MQTT-to-Telegram bridge на Go. Подписывается на MQTT-топики, проверяет условия из YAML-конфига и отправляет уведомления в Telegram.

## Требования

- Go 1.22+
- Git (для загрузки модулей)
- `C:\Program Files\Go\bin\go.exe` (go1.26.4)
- `C:\Program Files\Git\cmd\git.exe`

## Сборка и запуск

```powershell
# Обновить PATH (если не прописан глобально)
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")

# Сборка
go build -o bridge.exe .\cmd\bridge\

# Запуск
.\bridge.exe -config config.yaml
```

## Архитектура

```
cmd/bridge/main.go          — точка входа, загрузка конфига, graceful shutdown
internal/config/config.go   — YAML-конфиг, валидация, defaults
internal/mqtt/subscriber.go — MQTT-подписчик (paho), inbound канал
internal/engine/engine.go   — Worker pool, processMessage
internal/engine/evaluator.go — Проверка условий, time window, cooldown
internal/engine/condition.go — Сравнения (eq/ne/gt/gte/lt/lte), gjson
internal/engine/template.go — Go-шаблоны сообщений, кеширование
internal/engine/debounce.go — Cooldown-трекер с фондовой очисткой
internal/notifier/notifier.go — Интерфейс Notifier (один метод Send)
internal/notifier/telegram.go — Telegram Bot API, retry+backoff
internal/app/app.go         — Оркестратор, pipeline, shutdown из 3 фаз
```

### Pipeline данных

```
MQTT Broker → Subscriber → inboundCh (chan Message, buf=256)
    → Engine Workers (4 шт) → notifyCh (chan *TriggerResult, buf=64)
    → Notify Pump → Telegram API
```

### Ключевые решения

- Goroutines + channels вместо thread pool — три стадии пайплайна стыкуются через каналы без мьютексов
- Направленные каналы (`chan<-`, `<-chan`) — владение проверяется компилятором
- `select + default` для backpressure — при переполнении буфера сообщение дропается с логом, а не блокирует отправителя
- `select + time.After` в Subscriber — не дропает мгновенно, даёт 5 сек на разгреб очереди
- WaitGroup для graceful shutdown — 3 фазы: MQTT disconnect → drain engine → drain notifier
- `*bool` для `enabled` — nil = default (true), три состояния без enum
- CooldownTracker с RWMutex — reads (IsOnCooldown) massively outnumber writes (Set)
- Фоновая горутина-sweeper — удаляет просроченные cooldown-записи

### 9 точек потери сообщений

| # | Где | Механизм | Лог |
|---|-----|----------|-----|
| 1 | inboundCh | буфер 256 полон → таймаут 5 сек → дроп | `inbound queue stuck, dropping` |
| 2 | evaluator | невалидный JSON | `invalid JSON payload` |
| 3 | evaluator | topic regex не совпал | нет |
| 4 | evaluator | условия не сработали | нет |
| 5 | evaluator | time window не совпал | нет |
| 6 | evaluator | cooldown активен | нет (опасность: пустой deviceKey) |
| 7 | engine | ошибка шаблона | `template parse/render failed` |
| 8 | notifyCh | буфер 64 полон → дроп | `notify queue full` |
| 9 | telegram | retry исчерпан | `telegram send failed` |

## Последние изменения

- `subscriber.go`: замена мгновенного дропа на `time.After(5s)` с таймаутом (исправление точки потери №1)
- `evaluator.go`: `for i, cond` → `for _, cond` (убрана неиспользуемая переменная)
- `go.mod`: `gorilla/websocket v1.6.0` → `v1.5.3` (битая версия не существовала)

## Дальнейшие улучшения (обсуждались, не реализованы)

- Конкурентный notify pump (семафор из 4 горутин для параллельных запросов в Telegram)
- Dead Letter Queue на диске для непрошедших retry уведомлений
- Мониторинг дропов через метрики (Prometheus / expvar)
