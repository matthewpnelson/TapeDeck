# TapeDeck Development Setup for Unraid

This guide shows how to develop TapeDeck locally on your Mac while running the app with hot reload on your Unraid server.

## Overview

The development setup uses:
- **Local editing**: Edit code on your Mac with your preferred IDE
- **Automatic sync**: File watcher automatically syncs changes to Unraid via SMB
- **Remote execution**: Run the app in a Docker container on Unraid
- **Hot reload**: Air watches for file changes and automatically rebuilds/restarts
- **Auto-refresh**: Browser auto-refreshes when code changes (port 3002)

Changes made on your Mac appear in Unraid within 1-2 seconds.

## Quick Start (Recommended)

The fastest way to get started uses the automated helper scripts:

### Prerequisites

1. Unraid server with Docker installed and SSH access
2. TapeDeck repository cloned on your Mac at `/Users/mnelson/Documents/Projects/TapeDeck`
3. Network access between your Mac and Unraid
4. fswatch installed: `brew install fswatch`
5. SMB share mounted at `/Volumes/tapedeck` (see Setup below)

### One-Command Development Start

```bash
# Set your Unraid hostname/IP (do this once)
export UNRAID_HOST=your-unraid-ip

# Start everything (sync + container + watcher)
./scripts/dev-start.sh
```

This will:
1. Check SMB mount is available
2. Sync your code to Unraid
3. Start the development container on Unraid
4. Begin watching for file changes

Now just edit files in your IDE and save - changes automatically sync and reload!

### View Logs

```bash
./scripts/dev-logs.sh
```

### Stop Development

Press `Ctrl+C` in the dev-start terminal, then:

```bash
./scripts/dev-stop.sh
```

---

## Detailed Setup Instructions

If you prefer to understand and set up each piece manually, or if the quick start doesn't work for you:

### Step 1: Mount SMB Share

1. On Unraid, ensure you have an SMB share created for TapeDeck (usually at `/mnt/user/appdata/tapedeck`)

2. On your Mac, mount the SMB share:

   **Option A: Using Finder (Persists across reboots)**
   - Open Finder
   - Press `Cmd+K` (or Go → Connect to Server)
   - Enter: `smb://your-unraid-ip/appdata/tapedeck`
   - Click Connect
   - The share will appear at `/Volumes/tapedeck`

   **Option B: Using Terminal**
   ```bash
   # Mount SMB share (replace with your Unraid IP)
   mount_smbfs //root@your-unraid-ip/appdata/tapedeck /Volumes/tapedeck
   ```

3. Verify the mount:
   ```bash
   ls /Volumes/tapedeck
   ```

You should see your TapeDeck files (or an empty directory if first time).

### Step 2: Initial Setup and Sync

1. Install fswatch for automatic file watching:
   ```bash
   brew install fswatch
   ```

2. Perform initial sync to copy files to Unraid:
   ```bash
   ./scripts/sync-to-unraid.sh
   ```

### Step 3: Start Development Container on Unraid

Either use the helper script:

```bash
export UNRAID_HOST=your-unraid-ip
./scripts/dev-start.sh
```

Or manually via SSH:

```bash
ssh root@your-unraid-ip
cd /mnt/user/appdata/tapedeck

# Create .env if needed
cp .env.example .env

# Start container
docker-compose -f docker-compose.dev.yml up -d
exit
```

### Step 4: Start File Watcher

In a separate terminal on your Mac, start the file watcher:

```bash
./scripts/watch-and-sync.sh
```

This will watch for changes and automatically sync them to Unraid. Keep this terminal open while developing.

### Step 5: Access the Application

- **Development server with auto-refresh**: http://your-unraid-ip:3002
- **Direct app server**: http://your-unraid-ip:3001

Use port **3002** for development - it includes Air's proxy that auto-refreshes your browser when code changes.

### Step 6: Develop!

1. Edit code on your Mac in `/Users/mnelson/Documents/Projects/TapeDeck/`
2. Save the file
3. Watcher syncs to Unraid (< 1 second)
4. Air detects the change and rebuilds
5. Browser auto-refreshes (if using port 3002)
6. Changes appear in 1-2 seconds total

## Viewing Logs

Use the helper script:

```bash
./scripts/dev-logs.sh
```

Or manually via SSH:

```bash
ssh root@your-unraid-ip
docker-compose -f /mnt/user/appdata/tapedeck/docker-compose.dev.yml logs -f tapedeck-dev
```

You'll see messages like:
```
main.go has changed
building...
running...
```

## Stopping Development

Use the helper script:

```bash
# Stop file watcher (Ctrl+C in watch-and-sync.sh terminal)
# Then stop container:
./scripts/dev-stop.sh
```

Or manually:

```bash
ssh root@your-unraid-ip
cd /mnt/user/appdata/tapedeck
docker-compose -f docker-compose.dev.yml down
```

## Troubleshooting

### Changes not detected

1. Check that files are actually syncing to Unraid:
   ```bash
   # On Unraid
   ls -la /path/to/tapedeck-dev
   date && stat /path/to/tapedeck-dev/main.go
   ```

2. Check Air is running:
   ```bash
   docker-compose -f docker-compose.dev.yml logs tapedeck-dev | grep -i air
   ```

### Permission issues

If you see permission errors:

```bash
# On Unraid, fix ownership
cd /path/to/tapedeck-dev
chown -R 1000:100 .
```

### Build errors

Check the Air build logs:

```bash
docker-compose -f docker-compose.dev.yml logs tapedeck-dev | grep -i error
```

Or check the build log file inside the container:

```bash
docker exec tapedeck-dev cat /app/build-errors.log
```

### Container won't start

Check for port conflicts:

```bash
# On Unraid
netstat -tlnp | grep -E '3001|3002'
```

If ports are in use, stop your production container:

```bash
docker stop tapedeck
```

## Switching Back to Production

When done developing:

```bash
# Stop dev container
docker-compose -f docker-compose.dev.yml down

# Start production container
docker-compose up -d
```

## Tips

1. **Use port 3002** for development - auto-refresh is a huge time-saver
2. **Keep production container stopped** while developing to avoid port conflicts
3. **Commit frequently** if using Git sync method
4. **Check Air logs** if hot reload seems broken - it will show build errors
5. **Use DEV_MODE=true** in .env to skip TLS verification for local Home Assistant

## Development Workflow Example

### Using Helper Scripts (Recommended)

```bash
# Morning: Start development
export UNRAID_HOST=your-unraid-ip  # Set once per terminal session
./scripts/dev-start.sh

# Terminal shows file watcher running - keep it open
# Open http://your-unraid-ip:3002 in browser
# Edit code in your IDE, save, watch it reload automatically

# View logs in another terminal if needed
./scripts/dev-logs.sh

# Evening: Stop development
# Ctrl+C in dev-start terminal
./scripts/dev-stop.sh
```

### Manual Workflow

```bash
# Morning: Start development
ssh root@your-unraid-ip
cd /mnt/user/appdata/tapedeck
docker-compose -f docker-compose.dev.yml up -d
exit

# Start file watcher (new terminal)
./scripts/watch-and-sync.sh

# On Mac: Edit code in your IDE
# Open http://your-unraid-ip:3002 in browser
# Code, save, watch it reload automatically

# Evening: Stop development
# Ctrl+C in watcher terminal
ssh root@your-unraid-ip
cd /mnt/user/appdata/tapedeck
docker-compose -f docker-compose.dev.yml down
exit
```

## File Structure

```
TapeDeck/
├── Dockerfile.dev              # Development Docker image
├── docker-compose.dev.yml      # Development compose config
├── .air.toml                   # Air hot reload config (already exists)
├── scripts/                    # Helper scripts
│   ├── dev-start.sh           # Start dev environment (sync + container + watcher)
│   ├── dev-stop.sh            # Stop dev container on Unraid
│   ├── dev-logs.sh            # View container logs
│   ├── sync-to-unraid.sh      # Manual sync to Unraid
│   └── watch-and-sync.sh      # File watcher for auto-sync
├── .env                        # Environment variables
├── data/                       # Persisted data (mounted volume)
│   ├── tapedeck.db
│   ├── .session_secret
│   ├── .csrf_key
│   └── .encryption_key
└── ... (source code)
```

## Helper Scripts Reference

All scripts are in the `scripts/` directory:

- **`dev-start.sh`**: All-in-one startup (checks mount, syncs, starts container, starts watcher)
- **`dev-stop.sh`**: Stops the development container on Unraid
- **`dev-logs.sh`**: Tails the container logs via SSH
- **`sync-to-unraid.sh`**: One-time sync from local to Unraid
- **`watch-and-sync.sh`**: Watches for file changes and auto-syncs

All scripts read the `UNRAID_HOST` environment variable for your Unraid hostname/IP.

## Next Steps

- Read [CLAUDE.md](CLAUDE.md) for project architecture
- Run `./checks.sh` before committing to ensure code quality
- Check GitHub Actions for CI/CD status
