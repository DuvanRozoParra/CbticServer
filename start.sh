#!/bin/bash

# Ruta del repositorio (ajústalo a tu repo real)
REPO_URL="https://github.com/usuario/mi-servidor.git"
APP_DIR="/src"

echo "Actualizando servidor Go desde $REPO_URL..."
cd $APP_DIR

# Hacer pull de la última versión
git pull origin main

# Compilar el servidor
echo "Compilando servidor..."
go build -o /app/servidor .

# Ejecutar el servidor
echo "Iniciando servidor..."
exec /app/servidor
