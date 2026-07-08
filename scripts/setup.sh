#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# setup.sh — разворачивает smarthome-bridge на сервере
# ---------------------------------------------------------------------------
# Запуск из корня репозитория:
#   bash scripts/setup.sh
# ---------------------------------------------------------------------------
set -euo pipefail

# Корень репозитория — на один уровень выше scripts/
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASE_DIR="${BASE_DIR:-/opt/smarthome-bridge}"

echo "=== smarthome-bridge setup ==="
echo "Repo root : $REPO_ROOT"
echo "Base dir  : $BASE_DIR"
echo ""

# --- Шаг 1: Создать каталог и скопировать исходники ---
echo "[1/3] Copying repository into $BASE_DIR/src/..."
mkdir -p "$BASE_DIR"

if [ -d "$BASE_DIR/src" ]; then
    echo "  -> src/ already exists, skipping"
else
    cp -r "$REPO_ROOT" "$BASE_DIR/src"
    echo "  -> src/ copied"
fi

# --- Шаг 2: Развернуть рабочие файлы ---
echo "[2/3] Deploying runtime files..."

if [ ! -f "$BASE_DIR/docker-compose.yml" ]; then
    cp "$BASE_DIR/src/docker-compose.prod.yml" "$BASE_DIR/docker-compose.yml"
    echo "  -> docker-compose.yml created"
else
    echo "  -> docker-compose.yml already exists, skipping"
fi

if [ ! -f "$BASE_DIR/config.yaml" ]; then
    cp "$BASE_DIR/src/config.example.yaml" "$BASE_DIR/config.yaml"
    echo "  -> config.yaml created (from example)"
    echo "  ==> EDIT $BASE_DIR/config.yaml BEFORE STARTING <=="
else
    echo "  -> config.yaml already exists, skipping"
fi

# --- Шаг 3: Финализация ---
echo "[3/3] Done."
echo ""
echo "Directory layout:"
echo "  $BASE_DIR/"
echo "  ├── src/                  ← untouched repository copy"
echo "  ├── docker-compose.yml"
echo "  └── config.yaml"
echo ""
echo "Next steps:"
echo "  1. Edit config.yaml:  nano $BASE_DIR/config.yaml"
echo "  2. Start the service: cd $BASE_DIR && docker compose up -d"
echo "  3. Check logs:        cd $BASE_DIR && docker compose logs -f"
echo ""
echo "To update later:"
echo "  git -C $BASE_DIR/src pull"
echo "  cd $BASE_DIR && docker compose up -d --build"
