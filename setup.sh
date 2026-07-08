#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# setup.sh — разворачивает smarthome-bridge в родительский каталог
# ---------------------------------------------------------------------------
# Использование:
#   cd /opt/smarthome-bridge
#   git clone <repo-url>
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

# --- Шаг 1: Настроить git ---
echo "[1/3] Configuring git..."

git config --global --add safe.directory "$REPO_ROOT" 2>/dev/null || true
echo "  -> safe.directory added"

git -C "$REPO_ROOT" config remote.origin.pushurl "PUSH_BLOCKED__use_your_local_machine"
echo "  -> push blocked on server"

# --- Шаг 2: Создать симлинки и развернуть рабочие файлы ---
echo "[2/3] Deploying runtime files..."

ln -sfn "$REPO_NAME" "$TARGET/src"
echo "  -> symlink src/ -> $REPO_NAME/"

if [ ! -f "$TARGET/compose.yml" ]; then
    cp "$REPO_ROOT/docker-compose.prod.yml" "$TARGET/compose.yml"
    echo "  -> compose.yml created"
else
    echo "  -> compose.yml already exists, skipping"
fi

if [ ! -f "$TARGET/config.yaml" ]; then
    cp "$REPO_ROOT/config.example.yaml" "$TARGET/config.yaml"
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
echo "  ├── compose.yml"
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
