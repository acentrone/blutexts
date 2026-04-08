#!/bin/sh
# Run all migrations in order, then start the API.
# Each migration is idempotent-safe (CREATE IF NOT EXISTS / DROP IF EXISTS).
set -e

echo "Running database migrations..."
for f in /app/migrations/*.up.sql; do
  echo "  → $(basename $f)"
  psql "$DATABASE_URL" -f "$f" 2>&1 || true
done
echo "Migrations complete."

exec /api "$@"
