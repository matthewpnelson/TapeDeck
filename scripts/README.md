# TapeDeck NFC Webhook Integration

This directory contains the updated Python script for your Raspberry Pi NFC reader that sends tags directly to TapeDeck via webhook instead of through Home Assistant.

## Changes from Previous Setup

**Before**: Raspberry Pi → Home Assistant webhook → Home Assistant automation → TapeDeck WebSocket

**Now**: Raspberry Pi → TapeDeck webhook (direct)

## Setup Instructions

### 1. Configure TapeDeck Webhook Secret (Optional but Recommended)

Edit your `config.yml` on the TapeDeck server:

```yaml
version: 1
plex_servers:
  # ... your existing Plex servers ...
home_assistant:
  url: http://192.168.4.38:8123
  webhook_secret: "your-secret-here"  # Add this line (use a strong random string)
apple_tvs:
  # ... your existing Apple TVs ...
```

Generate a secure secret:
```bash
# On your TapeDeck server or Raspberry Pi:
openssl rand -hex 32
```

### 2. Update the Python Script

1. Copy `tapedeck_nfc.py` to your Raspberry Pi:
   ```bash
   scp scripts/tapedeck_nfc.py pi@raspberrypi:/home/pi/
   ```

2. Edit the configuration at the top of the file:
   ```python
   # Replace XX with your TapeDeck server's IP address
   TAPEDECK_URL = "http://192.168.4.XX:3001/api/webhook/nfc"

   # If you set webhook_secret in config.yml, uncomment and set this:
   WEBHOOK_SECRET = "your-secret-here"  # Must match config.yml
   ```

3. Make it executable:
   ```bash
   chmod +x /home/pi/tapedeck_nfc.py
   ```

### 3. Update the Systemd Service

Edit your systemd service file (usually `/etc/systemd/system/tapedeck-nfc.service`):

```ini
[Unit]
Description=TapeDeck NFC Reader Service
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi
ExecStart=/usr/bin/python3 /home/pi/tapedeck_nfc.py
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Reload and restart the service:
```bash
sudo systemctl daemon-reload
sudo systemctl restart tapedeck-nfc
sudo systemctl status tapedeck-nfc
```

### 4. Test the Integration

1. Check the logs on the Raspberry Pi:
   ```bash
   sudo journalctl -u tapedeck-nfc -f
   ```

2. Tap an NFC card and watch for:
   - Raspberry Pi logs: `Tag detected: abc123...`
   - Raspberry Pi logs: `TapeDeck accepted tag abc123: 200`
   - TapeDeck logs: `Tag scanned tag_id=abc123...`

3. If the tag is already mapped, media should start playing on your Apple TV!

## Troubleshooting

### "Connection refused"
- Check that TapeDeck is running: `curl http://192.168.4.XX:3001/health`
- Verify the TAPEDECK_URL is correct in the script

### "401 Unauthorized"
- Check that WEBHOOK_SECRET in the script matches `webhook_secret` in `config.yml`
- If you didn't set a webhook secret in config.yml, set `WEBHOOK_SECRET = None` in the script

### "400 Bad Request"
- Check the Raspberry Pi logs for the exact error
- Verify the script is sending valid JSON with `tag_id` field

### Tags not triggering playback
- Check TapeDeck logs: `tail -f /path/to/DATA_DIR/tapedeck.log`
- Verify the tag is mapped in TapeDeck UI at `/mappings`
- Test the webhook manually:
  ```bash
  curl -X POST http://192.168.4.XX:3001/api/webhook/nfc \
    -H "Content-Type: application/json" \
    -H "X-Webhook-Secret: your-secret-here" \
    -d '{"tag_id": "test123"}'
  ```

## Benefits of Webhook Approach

1. **Simpler**: No Home Assistant automation needed
2. **Faster**: Direct connection, no intermediate steps
3. **More reliable**: Fewer points of failure
4. **Works alongside WebSocket**: You can still use the WebSocket integration for pairing in the UI

## Reverting to Home Assistant Webhook

If you want to go back to the Home Assistant approach, just change the Python script back to:
```python
HA_URL = "http://192.168.4.38:8123/api/webhook/tapedeck_nfc"
```

And keep your Home Assistant automation that fires `tag_scanned` events.
