#!/usr/bin/env bash
set -e

echo "=== 🚀 Starting container entrypoint ==="

echo "=> Switching to /var/www/demo"
cd /var/www/demo

echo "=> Running Go update script"
go run bin/update.go

echo "=> Running Go custom script"
go run bin/custom.go

echo "=> Running Go plugins script"
go run bin/plugins.go

echo "=> Running Go translations script"
go run bin/translations.go

echo "=> Switching to /var/www/demo/storefront"
cd /var/www/demo/storefront

echo "=> Installing global npm packages (pm2, npm, bun)"
npm install -g pm2 npm bun

echo "=> Running sitemap generator"
go run bin/sitemap.go

echo "=> Running bun install"
bun install

echo "=== ✔️ Entrypoint finished. Starting CMD ==="
exec "$@"
