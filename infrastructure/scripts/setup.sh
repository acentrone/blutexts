#!/bin/bash
# BlueSend Development Setup
# Run once to initialize the project

set -e

echo "🚀 BlueSend Setup"
echo "================="

# Check prerequisites
command -v docker >/dev/null 2>&1 || { echo "❌ Docker required"; exit 1; }
command -v go >/dev/null 2>&1 || { echo "❌ Go 1.22+ required"; exit 1; }
command -v node >/dev/null 2>&1 || { echo "❌ Node.js 20+ required"; exit 1; }

# Copy env file
if [ ! -f .env ]; then
    cp .env.example .env
    echo "✅ Created .env from .env.example — edit it before running"
fi

# Install frontend deps
echo "📦 Installing frontend dependencies..."
cd apps/web && npm install && cd ../..

# Download Go deps
echo "📦 Downloading Go API dependencies..."
cd services/api && go mod download && cd ../..
cd apps/device-agent && go mod download && cd ../..

# Start infrastructure
echo "🐳 Starting Docker services..."
docker compose up -d postgres redis

echo "⏳ Waiting for database..."
sleep 3

echo ""
echo "✅ Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Edit .env with your Stripe, GHL, and secret keys"
echo "  2. Run: make dev"
echo "  3. Open: http://localhost:3000"
echo ""
echo "To register a physical device:"
echo "  make device-register"
