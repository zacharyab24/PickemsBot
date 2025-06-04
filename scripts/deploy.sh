#!/usr/bin/sh
set -e  # Exit on error

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

echo "Deploy started at $(date)"
cd /home/latte/PickemsBot

git pull origin main
git config --global --add safe.directory /home/latte/PickemsBot

docker compose down
docker compose up -d --build
docker ps