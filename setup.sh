#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# setup.sh — разворачивает smarthome-bridge в родительский каталог
# ---------------------------------------------------------------------------
# Использование:
#   cd ./<your_smarthome-bridge_dir>
#   git clone <repo-url>  <src>   # имя каталога любое
#   cd <src> && bash setup.sh
#
# Или указать другой каталог:
#   TARGET=/srv/bridge bash setup.sh
# ---------------------------------------------------------------------------
set -euo pipefail

REPO_ROOT="$(pwd)"
REPO_NAME="$(basename "$REPO_ROOT")"
TARGET="${TARGET:-$(dirname "$REPO_ROOT")}"
SECRETS_DIR="$(realpath "$TARGET/../../data/docker-secrets")"

echo "=== smarthome-bridge setup ==="
echo "Repo root : $REPO_ROOT"
echo "Repo name : $REPO_NAME"
echo "Target    : $TARGET"
echo ""

# Настраиваем обновление через git pull
git config --global --add safe.directory "$REPO_ROOT" 2>/dev/null || true
echo "  -> git safe.directory added"

# Запретим push для prod'а
git -C "$REPO_ROOT" config remote.origin.pushurl "This is prod copy of repo. You are not allowed to push"

# --- Шаг 1: Креды ---
echo "[1/3] Configuring credentials..."

# MQTT Broker
if [ -n "${MQTT_BROKER:-}" ]; then
    echo "  -> MQTT_BROKER from env"
else
    read -r -p "MQTT broker address [tcp://localhost:1883]: " MQTT_BROKER
    MQTT_BROKER="${MQTT_BROKER:-tcp://localhost:1883}"
fi

# MQTT Username
if [ -n "${MQTT_USERNAME:-}" ]; then
    echo "  -> MQTT_USERNAME from env"
else
    read -r -p "MQTT username (empty to skip): " MQTT_USERNAME
fi

# MQTT Password
if [ -n "${MQTT_PASSWORD:-}" ]; then
    echo "  -> MQTT_PASSWORD from env"
else
    read -r -s -p "MQTT password: " MQTT_PASSWORD
    echo
    read -r -s -p "MQTT password (confirm): " MQTT_PASSWORD_CONFIRM
    echo
    if [ "$MQTT_PASSWORD" != "$MQTT_PASSWORD_CONFIRM" ]; then
        echo "  -> Passwords do not match!"
        exit 1
    fi
fi

# Telegram Bot Token
if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    echo "  -> TELEGRAM_BOT_TOKEN from env"
else
    read -r -s -p "Telegram bot token: " TELEGRAM_BOT_TOKEN
    echo
fi

# Сохраняем секреты
mkdir -p -m 700 "$SECRETS_DIR"

if [ -n "${MQTT_PASSWORD:-}" ]; then
    echo "$MQTT_PASSWORD" > "$SECRETS_DIR/mqtt_password.txt"
    chmod 600 "$SECRETS_DIR/mqtt_password.txt"
    echo "  -> mqtt_password saved to secrets/"
fi

if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    echo "$TELEGRAM_BOT_TOKEN" > "$SECRETS_DIR/telegram_bot_token.txt"
    chmod 600 "$SECRETS_DIR/telegram_bot_token.txt"
    echo "  -> telegram_bot_token saved to secrets/"
fi

# --- Шаг 2: Разворачиваем окружение ---
echo "[2/3] Deploying runtime files..."

ln -sfn "$REPO_NAME" "$TARGET/src"
echo "  -> symlink src/ -> $REPO_NAME/"

if [ ! -f "$TARGET/compose.yml" ]; then
    cp "$REPO_ROOT/docker-compose.prod.yml" "$TARGET/compose.yml"
    sed -i "s|__SECRETS_DIR__|$SECRETS_DIR|" "$TARGET/compose.yml"
    echo "  -> compose.yml created"
else
    echo "  -> compose.yml already exists, skipping"
fi

if [ ! -f "$TARGET/config.yaml" ]; then
    cp "$REPO_ROOT/config.example.yaml" "$TARGET/config.yaml"
    echo "  -> config.yaml created (from example)"
else
    echo "  -> config.yaml already exists, skipping"
fi

# --- Шаг 3: Готово ---
echo "[3/3] Done."
echo ""
echo "Directory layout:"
echo "  $TARGET/"
echo "  ├── src/ -> $REPO_NAME/     ← symlink to repository"
echo "  ├── $REPO_NAME/             ← repository (untouched)"
echo "  ├── compose.yml"
echo "  ├── config.yaml"
echo "  └── .env"
echo ""
echo "Secrets stored in:"
echo "  $SECRETS_DIR/"
echo "  ├── mqtt_password.txt"
echo "  └── telegram_bot_token.txt"
echo ""
echo "Start the service:"
echo "  cd $TARGET && docker compose up -d"
echo ""
echo "To update later:"
echo "  git -C $TARGET/$REPO_NAME pull"
echo "  cd $TARGET && docker compose up -d --build"
