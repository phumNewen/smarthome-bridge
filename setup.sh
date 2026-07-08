#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# setup.sh — разворачивает smarthome-bridge в родительский каталог
# ---------------------------------------------------------------------------
# Использование:
#   cd /opt/smarthome-bridge
#   git clone <repo-url>     # имя каталога любое
#   cd smarthome-bridge && bash setup.sh
#
# Или указать другой каталог:
#   TARGET=/srv/bridge bash setup.sh
# ---------------------------------------------------------------------------
set -euo pipefail

REPO_ROOT="$(pwd)"
REPO_NAME="$(basename "$REPO_ROOT")"
TARGET="${TARGET:-$(dirname "$REPO_ROOT")}"

echo "=== smarthome-bridge setup ==="
echo "Repo root : $REPO_ROOT"
echo "Repo name : $REPO_NAME"
echo "Target    : $TARGET"
echo ""

# --- Шаг 1: Разместить исходники ---
echo "[1/3] Placing repository at $TARGET/$REPO_NAME..."
mkdir -p "$TARGET"

if [ "$(realpath "$REPO_ROOT")" != "$(realpath "$TARGET/$REPO_NAME")" ]; then
    if [ -d "$TARGET/$REPO_NAME" ]; then
        echo "  -> $REPO_NAME/ already exists, skipping copy"
    else
        cp -r "$REPO_ROOT" "$TARGET/$REPO_NAME"
        echo "  -> copied to $REPO_NAME/"
    fi
else
    echo "  -> repo already in place, skipping copy"
fi

# Симлинк для docker-compose (всегда ссылается на актуальный каталог репо)
ln -sfn "$REPO_NAME" "$TARGET/src"
echo "  -> symlink src/ -> $REPO_NAME/"

# --- Шаг 2: Развернуть рабочие файлы ---
echo "[2/3] Deploying runtime files..."

if [ ! -f "$TARGET/docker-compose.yml" ]; then
    cp "$TARGET/src/docker-compose.prod.yml" "$TARGET/docker-compose.yml"
    echo "  -> docker-compose.yml created"
else
    echo "  -> docker-compose.yml already exists, skipping"
fi

if [ ! -f "$TARGET/config.yaml" ]; then
    cp "$TARGET/src/config.example.yaml" "$TARGET/config.yaml"
    echo "  -> config.yaml created (from example)"
    echo "  ==> EDIT $TARGET/config.yaml BEFORE STARTING <=="
else
    echo "  -> config.yaml already exists, skipping"
fi

# --- Шаг 3: Финализация ---
echo "[3/3] Done."
echo ""
echo "Directory layout:"
echo "  $TARGET/"
echo "  ├── src/ -> $REPO_NAME/     ← symlink to repository"
echo "  ├── $REPO_NAME/             ← repository (untouched)"
echo "  ├── docker-compose.yml"
echo "  └── config.yaml"
echo ""
echo "Next steps:"
echo "  1. Edit config:     nano $TARGET/config.yaml"
echo "  2. Start:           cd $TARGET && docker compose up -d"
echo "  3. Logs:            cd $TARGET && docker compose logs -f"
echo ""
echo "To update later:"
echo "  git -C $TARGET/$REPO_NAME pull"
echo "  cd $TARGET && docker compose up -d --build"
