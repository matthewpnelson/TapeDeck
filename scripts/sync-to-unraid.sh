#!/bin/bash

# Sync script - copies changed files to Unraid SMB share
# This script is called by the watcher when files change

SOURCE_DIR="/Users/mnelson/Documents/Projects/TapeDeck"
TARGET_DIR="/Volumes/tapedeck"

# Color output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if target directory is mounted
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: $TARGET_DIR not mounted"
    echo "Please mount the SMB share first"
    exit 1
fi

# Sync files
echo -e "${BLUE}Syncing to Unraid...${NC}"
rsync -a --delete \
    --exclude 'data/' \
    --exclude 'tmp/' \
    --exclude '.git/' \
    --exclude '.DS_Store' \
    --exclude '*.log' \
    --exclude 'node_modules/' \
    --exclude '.env.local' \
    "$SOURCE_DIR/" "$TARGET_DIR/"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Synced successfully${NC} $(date '+%H:%M:%S')"

    # Create .env from .env.example if it doesn't exist
    if [ ! -f "$TARGET_DIR/.env" ] && [ -f "$TARGET_DIR/.env.example" ]; then
        echo -e "${BLUE}Creating .env from .env.example...${NC}"
        cp "$TARGET_DIR/.env.example" "$TARGET_DIR/.env"
        echo -e "${GREEN}✓ .env created${NC}"
    fi
else
    echo "Error: Sync failed"
    exit 1
fi
