#!/usr/bin/env python3
"""
TapeDeck NFC Reader Service
Reads NFC tags and sends them to TapeDeck via webhook.
"""
import time
import requests
import logging
from logging.handlers import RotatingFileHandler
from smartcard.System import readers
try:
    from smartcard.pcsc.PCSCExceptions import EstablishContextException
except ImportError:
    from smartcard.Exceptions import EstablishContextException

from smartcard.Exceptions import NoCardException, CardConnectionException

# ---------- CONFIG ----------
# TapeDeck webhook endpoint
TAPEDECK_URL = "http://192.168.4.38:3001/api/webhook/nfc"  # Replace XX with TapeDeck server IP

# Optional webhook secret for security (must match webhook_secret in config.yml)
# If you set a webhook_secret in config.yml, uncomment and set this:
# WEBHOOK_SECRET = "your-secret-here"
WEBHOOK_SECRET = None

MAX_PCSD_RETRIES = 5
PCSC_RETRY_DELAY = 2  # seconds
TAPEDECK_RETRY_DELAY = 10  # seconds if TapeDeck is down

LOG_FILE = "/var/log/tapedeck_nfc.log"
LOG_MAX_BYTES = 5 * 1024 * 1024  # 5 MB
LOG_BACKUP_COUNT = 3

# ---------- LOGGING SETUP ----------
logger = logging.getLogger("TapedeckNFC")
logger.setLevel(logging.DEBUG)

# Also log to console
formatter = logging.Formatter("%(asctime)s [%(levelname)s] %(message)s")
console_handler = logging.StreamHandler()
console_handler.setFormatter(formatter)
logger.addHandler(console_handler)


# ---------- NFC FUNCTIONS ----------
def find_reader():
    """Attempt to find an NFC reader, retry if necessary."""
    for attempt in range(MAX_PCSD_RETRIES):
        try:
            r_list = readers()
            if not r_list:
                logger.warning(
                    f"No readers found, retry {attempt + 1}/{MAX_PCSD_RETRIES}"
                )
                time.sleep(PCSC_RETRY_DELAY)
                continue
            return r_list[0]
        except EstablishContextException as e:
            logger.warning(
                f"Failed to establish PC/SC context, retry {attempt + 1}/{MAX_PCSD_RETRIES}: {e}"
            )
            time.sleep(PCSC_RETRY_DELAY)
    raise RuntimeError("Could not establish PC/SC context after retries.")


def send_to_tapedeck(tag_id):
    """Send detected tag ID to TapeDeck webhook with retry if TapeDeck is down."""
    headers = {
        "Content-Type": "application/json",
    }

    # Add webhook secret header if configured
    if WEBHOOK_SECRET:
        headers["X-Webhook-Secret"] = WEBHOOK_SECRET

    data = {"tag_id": tag_id}

    while True:
        try:
            resp = requests.post(TAPEDECK_URL, json=data, headers=headers, timeout=5)
            if resp.status_code == 200:
                logger.info(f"TapeDeck accepted tag {tag_id}: {resp.status_code}")
                break
            else:
                logger.warning(
                    f"TapeDeck returned {resp.status_code}, retrying in {TAPEDECK_RETRY_DELAY}s"
                )
                time.sleep(TAPEDECK_RETRY_DELAY)
        except requests.exceptions.RequestException as e:
            logger.warning(
                f"Error connecting to TapeDeck: {e}, retrying in {TAPEDECK_RETRY_DELAY}s"
            )
            time.sleep(TAPEDECK_RETRY_DELAY)


def main():
    logger.info(f"TapeDeck NFC Reader starting - sending to {TAPEDECK_URL}")
    if WEBHOOK_SECRET:
        logger.info("Webhook secret authentication enabled")
    else:
        logger.warning("WARNING: No webhook secret configured - anyone can send tags!")

    while True:
        try:
            reader = find_reader()
            logger.info(f"Using reader: {reader}")
            connection = reader.createConnection()
            while True:
                try:
                    connection.connect()

                    # Send Get UID command (ISO 14443A standard APDU)
                    GET_UID = [0xFF, 0xCA, 0x00, 0x00, 0x00]
                    data, sw1, sw2 = connection.transmit(GET_UID)

                    # Check if successful (0x90 0x00 = success)
                    if sw1 == 0x90 and sw2 == 0x00:
                        tag_id = "".join("{:02x}".format(b) for b in data)
                        logger.info(f"Tag detected: {tag_id}")
                        send_to_tapedeck(tag_id)
                        time.sleep(1)  # small delay to avoid double reads
                    else:
                        logger.warning(f"Failed to get UID: SW1={sw1:02x} SW2={sw2:02x}")
                        time.sleep(0.5)

                except NoCardException:
                    time.sleep(0.5)
                except CardConnectionException as e:
                    logger.warning(f"Connection error: {e}")
                    time.sleep(1)
                except Exception as e:
                    logger.error(f"Error reading card UID: {e}")
                    time.sleep(1)
        except RuntimeError as e:
            logger.warning(
                f"PC/SC reader unavailable: {e}, retrying in {PCSC_RETRY_DELAY} sec"
            )
            time.sleep(PCSC_RETRY_DELAY)
        except KeyboardInterrupt:
            logger.info("Exiting...")
            break
        except Exception as e:
            logger.error(f"Unexpected error: {e}, retrying...")
            time.sleep(1)


if __name__ == "__main__":
    main()
