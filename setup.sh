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
REPO_VERSION=$(cat "$REPO_ROOT/VERSION")
DEPLOYED_VERSION=$(cat "$TARGET/.version" 2>/dev/null || echo "0")

echo "=== smarthome-bridge setup ==="
echo "Repo root : $REPO_ROOT"
echo "Repo name : $REPO_NAME"
echo "Target    : $TARGET"
echo ""

# Подхватываем заполненный .env (если есть)
if [ -f "$TARGET/.env" ]; then
    set -a && source "$TARGET/.env" && set +a
    echo "  -> .env loaded"
fi

# Настраиваем обновление через git pull
git config --global --add safe.directory "$REPO_ROOT" 2>/dev/null || true
# Запретим push для prod'а
git -C "$REPO_ROOT" config remote.origin.pushurl "This is prod copy of repo. You are not allowed to push"

# Проверим версию деплоя
if [ "$REPO_VERSION" != "$DEPLOYED_VERSION" ]; then
    echo "  -> version changed ($DEPLOYED_VERSION -> $REPO_VERSION), will overwrite deployed files"
    OVERWRITE=true
else
    OVERWRITE=false
fi

# Создаём каталог и шаблоны секретов
mkdir -p -m 700 "$SECRETS_DIR"

if [ ! -f "$SECRETS_DIR/mqtt_password.txt" ]; then
    echo "REPLACE_ME" > "$SECRETS_DIR/mqtt_password.txt"
    chmod 600 "$SECRETS_DIR/mqtt_password.txt"
    echo "  -> secrets/mqtt_password.txt created (fill in the value)"
fi

if [ ! -f "$SECRETS_DIR/telegram_bot_token.txt" ]; then
    echo "REPLACE_ME" > "$SECRETS_DIR/telegram_bot_token.txt"
    chmod 600 "$SECRETS_DIR/telegram_bot_token.txt"
    echo "  -> secrets/telegram_bot_token.txt created (fill in the value)"
fi

# --- Шаг 1: Разворачиваем окружение ---
echo "[1/2] Deploying runtime files..."

ln -sfn "$REPO_NAME" "$TARGET/src"
echo "  -> symlink src/ -> $REPO_NAME/"

if [ ! -f "$TARGET/compose.yml" ] || [ "$OVERWRITE" = true ]; then
    cp "$REPO_ROOT/docker-compose.prod.yml" "$TARGET/compose.yml"
    sed -i "s|__SECRETS_DIR__|$SECRETS_DIR|" "$TARGET/compose.yml"
    echo "  -> compose.yml created"
else
    echo "  -> compose.yml already exists, skipping"
fi

if [ ! -f "$TARGET/config.yaml" ] || [ "$OVERWRITE" = true ]; then
    cp "$REPO_ROOT/config.example.yaml" "$TARGET/config.yaml"
    echo "  -> config.yaml created (from example)"
else
    echo "  -> config.yaml already exists, skipping"
fi

if [ ! -f "$TARGET/.env" ]; then
    cat > "$TARGET/.env" << 'ENVEOF'
# MQTT broker address
MQTT_BROKER=tcp://localhost:1883
# MQTT username (leave empty to skip)
MQTT_USERNAME=
# Admin chat ID for system notifications (startup, errors)
# Can be a single ID or comma-separated list: 123456,789012
ADMIN_CHAT_ID=
ENVEOF
    echo "  -> .env created (fill in the values)"
else
    echo "  -> .env already exists, skipping"
fi

echo "$REPO_VERSION" > "$TARGET/.version"

# --- Шаг 2: Готово ---
echo "[2/2] Done."
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
