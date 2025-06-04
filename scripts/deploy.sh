#!/bin/bash

set -e  # Exit on error

# Optional: log the deploy time
echo "Deploy started at $(date)"

# Go to your app directory
cd /home/latte/PickemsBot

# Reset any local changes (optional safety)
git reset --hard

# Pull latest code from main
git pull origin main

# (Optional) Stop existing Docker containers
docker compose down

# Build and restart containers
docker compose up -d --build

# Log status
docker ps
