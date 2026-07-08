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
echo "  -> push blocked on server"

# --- Шаг 1: Симлинк и рабочие файлы ---
echo "[1/2] Deploying runtime files..."

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

# --- Шаг 2: Финализация ---
echo "[2/2] Done."
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
