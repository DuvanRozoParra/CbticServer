#!/bin/sh

echo "[INFO] Actualizando código desde GitHub..."
git pull origin main

echo "[INFO] Compilando nueva versión..."
go build -o server ./cmd

echo "[INFO] Ejecutando servidor..."
exec ./server
