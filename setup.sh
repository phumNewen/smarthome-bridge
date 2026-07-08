#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# setup.sh — разворачивает smarthome-bridge в родительский каталог
# ---------------------------------------------------------------------------
# Использование:
#   cd /opt/smarthome-bridge
#   git clone <repo-url> src
#   cd src && bash setup.sh
#
# Или указать другой каталог:
#   TARGET=/srv/bridge bash setup.sh
# ---------------------------------------------------------------------------
set -euo pipefail

REPO_ROOT="$(pwd)"
TARGET="${TARGET:-$(dirname "$REPO_ROOT")}"

echo "=== smarthome-bridge setup ==="
echo "Repo root : $REPO_ROOT"
echo "Target    : $TARGET"
echo ""

# --- Шаг 1: Разместить исходники ---
echo "[1/3] Placing repository..."
mkdir -p "$TARGET"

if [ "$(realpath "$REPO_ROOT")" = "$(realpath "$TARGET/src")" ]; then
    echo "  -> repo already at src/, skipping copy"
else
    if [ -d "$TARGET/src" ]; then
        echo "  -> src/ already exists, skipping"
    else
        cp -r "$REPO_ROOT" "$TARGET/src"
        echo "  -> src/ copied"
    fi
fi

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
echo "  ├── src/                  ← untouched repository"
echo "  ├── docker-compose.yml"
echo "  └── config.yaml"
echo ""
echo "Next steps:"
echo "  1. Edit config:     nano $TARGET/config.yaml"
echo "  2. Start:           cd $TARGET && docker compose up -d"
echo "  3. Logs:            cd $TARGET && docker compose logs -f"
echo ""
echo "To update later:"
echo "  git -C $TARGET/src pull"
echo "  cd $TARGET && docker compose up -d --build"
