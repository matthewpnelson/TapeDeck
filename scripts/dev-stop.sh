#!/bin/bash

# Stop the development environment on Unraid

UNRAID_HOST="${UNRAID_HOST:-tower.local}"  # Default to tower.local (Unraid default)
UNRAID_PATH="${UNRAID_PATH:-/mnt/user/tapedeck}"  # Path to tapedeck on Unraid

# Color output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   TapeDeck Development Shutdown        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

echo -e "${YELLOW}Stopping Docker container on Unraid...${NC}"
echo "Connecting to $UNRAID_HOST..."

ssh root@$UNRAID_HOST "cd $UNRAID_PATH && docker-compose -f docker-compose.dev.yml down"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Development environment stopped${NC}"
    echo ""
    echo "To restart development, run: ./scripts/dev-start.sh"
else
    echo -e "${RED}✗ Failed to stop container${NC}"
    echo ""
    echo "Try running manually on Unraid:"
    echo "  ssh root@$UNRAID_HOST"
    echo "  cd $UNRAID_PATH"
    echo "  docker-compose -f docker-compose.dev.yml down"
    exit 1
fi
