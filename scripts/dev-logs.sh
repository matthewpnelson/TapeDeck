#!/bin/bash

# View logs from the development container on Unraid

UNRAID_HOST="${UNRAID_HOST:-tower.local}"  # Default to tower.local (Unraid default)
UNRAID_PATH="${UNRAID_PATH:-/mnt/user/tapedeck}"  # Path to tapedeck on Unraid

# Color output
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   TapeDeck Development Logs            ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""
echo "Connecting to $UNRAID_HOST..."
echo "Press Ctrl+C to stop following logs"
echo ""

ssh root@$UNRAID_HOST "cd $UNRAID_PATH && docker-compose -f docker-compose.dev.yml logs -f --tail=50 tapedeck-dev"
