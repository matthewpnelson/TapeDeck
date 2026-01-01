#!/bin/bash

# Development workflow starter
# Starts the dev container on Unraid and begins watching for changes

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
UNRAID_HOST="${UNRAID_HOST:-tower.local}"  # Default to tower.local (Unraid default)
UNRAID_PATH="${UNRAID_PATH:-/mnt/user/tapedeck}"  # Path to tapedeck on Unraid

# Color output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   TapeDeck Development Startup         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

# Step 1: Check if SMB share is mounted
echo -e "${YELLOW}[1/4]${NC} Checking SMB mount..."
if [ ! -d "/Volumes/tapedeck" ]; then
    echo -e "${RED}✗ /Volumes/tapedeck not mounted${NC}"
    echo ""
    echo "Please mount the SMB share first:"
    echo "  Finder → Go → Connect to Server → smb://$UNRAID_HOST/appdata/tapedeck"
    exit 1
fi
echo -e "${GREEN}✓ SMB share mounted${NC}"
echo ""

# Step 2: Initial sync
echo -e "${YELLOW}[2/4]${NC} Performing initial sync..."
"$SCRIPT_DIR/sync-to-unraid.sh"
echo ""

# Step 3: Start Docker container on Unraid
echo -e "${YELLOW}[3/4]${NC} Starting Docker container on Unraid..."
echo "Connecting to $UNRAID_HOST..."

ssh root@$UNRAID_HOST "cd $UNRAID_PATH && docker-compose -f docker-compose.dev.yml up -d"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Container started${NC}"
    echo ""
    echo "Access the app at:"
    echo -e "  ${BLUE}http://$UNRAID_HOST:3002${NC} (dev with auto-refresh)"
    echo -e "  ${BLUE}http://$UNRAID_HOST:3001${NC} (direct app server)"
else
    echo -e "${RED}✗ Failed to start container${NC}"
    echo ""
    echo "Try running manually on Unraid:"
    echo "  ssh root@$UNRAID_HOST"
    echo "  cd $UNRAID_PATH"
    echo "  docker-compose -f docker-compose.dev.yml up -d"
    exit 1
fi
echo ""

# Step 4: Start file watcher
echo -e "${YELLOW}[4/4]${NC} Starting file watcher..."
echo -e "${GREEN}✓ Development environment ready!${NC}"
echo ""
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "Edit files in your IDE and save"
echo -e "Changes will sync automatically to Unraid"
echo -e "Air will rebuild and browser will refresh"
echo -e ""
echo -e "Press ${YELLOW}Ctrl+C${NC} to stop watching"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo ""

# Run watcher in foreground
"$SCRIPT_DIR/watch-and-sync.sh"
