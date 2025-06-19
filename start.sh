#!/bin/sh
set -e

echo "[INFO] Actualizando código..."
cd /app
git pull origin main

echo "[INFO] Compilando nueva versión..."
go build -o server ./cmd

echo "[INFO] Iniciando servidor..."
exec ./server
