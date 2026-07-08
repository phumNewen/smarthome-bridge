#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# setup.sh — разворачивает smarthome-bridge на сервере
# ---------------------------------------------------------------------------
# Запуск:
#   curl -fsSL https://raw.githubusercontent.com/<user>/smarthome-bridge/master/scripts/setup.sh | bash
#   или
#   mkdir -p /opt/smarthome-bridge && cd /opt/smarthome-bridge && bash <этот-скрипт>
# ---------------------------------------------------------------------------
set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/<user>/smarthome-bridge.git}"
BASE_DIR="${BASE_DIR:-/opt/smarthome-bridge}"
SRC_DIR="$BASE_DIR/src"

echo "=== smarthome-bridge setup ==="
echo "Base dir : $BASE_DIR"
echo "Repo URL : $REPO_URL"
echo ""

# --- Шаг 1: Создать структуру каталогов ---
echo "[1/4] Creating directory structure..."
mkdir -p "$BASE_DIR" "$SRC_DIR"

# --- Шаг 2: Клонировать репозиторий ---
echo "[2/4] Cloning repository into src/..."
if [ -d "$SRC_DIR/.git" ]; then
    echo "  -> Repository exists, pulling latest..."
    git -C "$SRC_DIR" pull --ff-only
else
    git clone "$REPO_URL" "$SRC_DIR"
fi

# --- Шаг 3: Развернуть рабочие файлы ---
echo "[3/4] Deploying runtime files..."

# docker-compose.yml — копируем шаблон и подставляем пути
if [ ! -f "$BASE_DIR/docker-compose.yml" ]; then
    cp "$SRC_DIR/docker-compose.prod.yml" "$BASE_DIR/docker-compose.yml"
    echo "  -> docker-compose.yml created"
else
    echo "  -> docker-compose.yml already exists, skipping"
fi

# config.yaml — копируем пример, если нет реального конфига
if [ ! -f "$BASE_DIR/config.yaml" ]; then
    cp "$SRC_DIR/config.example.yaml" "$BASE_DIR/config.yaml"
    echo "  -> config.yaml created (from example)"
    echo "  ==> EDIT $BASE_DIR/config.yaml BEFORE STARTING <=="
else
    echo "  -> config.yaml already exists, skipping"
fi

# --- Шаг 4: Права и финализация ---
echo "[4/4] Done."
echo ""
echo "Directory layout:"
echo "  $BASE_DIR/"
echo "  ├── src/                  ← untouched git repo"
echo "  ├── docker-compose.yml"
echo "  └── config.yaml"
echo ""
echo "Next steps:"
echo "  1. Edit config.yaml:  nano $BASE_DIR/config.yaml"
echo "  2. Start the service: docker compose -f $BASE_DIR/docker-compose.yml up -d"
echo "  3. Check logs:        docker compose -f $BASE_DIR/docker-compose.yml logs -f"
