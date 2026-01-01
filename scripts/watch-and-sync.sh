#!/bin/bash

# File watcher that automatically syncs changes to Unraid
# Uses fswatch to detect file changes and trigger sync

SOURCE_DIR="/Users/mnelson/Documents/Projects/TapeDeck"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Color output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   TapeDeck Auto-Sync to Unraid        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

# Check if fswatch is installed
if ! command -v fswatch &> /dev/null; then
    echo -e "${RED}Error: fswatch is not installed${NC}"
    echo ""
    echo "Install with: brew install fswatch"
    exit 1
fi

# Check if target directory is mounted
if [ ! -d "/Volumes/tapedeck" ]; then
    echo -e "${RED}Error: /Volumes/tapedeck not mounted${NC}"
    echo ""
    echo "Please mount the SMB share first"
    exit 1
fi

# Do initial sync
echo -e "${YELLOW}Performing initial sync...${NC}"
"$SCRIPT_DIR/sync-to-unraid.sh"
echo ""

# Start watching
echo -e "${GREEN}✓ Watching for file changes...${NC}"
echo -e "${BLUE}Press Ctrl+C to stop${NC}"
echo ""

# Watch for changes and sync
# -r: recursive
# -e Updated: only watch for update events (not create/delete to avoid duplicate syncs)
# -l 0.5: latency of 0.5 seconds (debounce rapid changes)
fswatch -r \
    --exclude '\.git/' \
    --exclude 'data/' \
    --exclude 'tmp/' \
    --exclude '\.DS_Store' \
    --exclude '.*\.log$' \
    --exclude 'node_modules/' \
    -e Updated \
    -l 0.5 \
    "$SOURCE_DIR" | while read -r file
do
    echo -e "${YELLOW}Change detected:${NC} $(basename "$file")"
    "$SCRIPT_DIR/sync-to-unraid.sh"
done
