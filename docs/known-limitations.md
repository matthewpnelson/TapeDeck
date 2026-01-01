# Known Limitations

## Live TV / DVR Channels Not Supported on Apple TV

**Issue**: NFC cards cannot be mapped to Live TV channels when using Apple TV as the playback device.

**Why**: After extensive testing and research, we found that:

1. **Plex deep links don't support Live TV content** - The `plex://play/` URL scheme only works with regular library content (`/library/metadata/`), not Live TV provider endpoints
2. **Apple TV doesn't support direct video URLs** - Attempting to play m3u8 streams directly via Home Assistant's `play_media` service returns a 500 Internal Server Error
3. **IPTV apps lack deep link support** - Third-party IPTV players (GSE Smart IPTV, iPlayTV, etc.) don't expose URL schemes for launching specific channels on tvOS
4. **No community solutions exist** - The Home Assistant community has been trying to solve this since 2021 with no working solutions

**Attempted Solutions**:
- ✗ Plex deep link with provider-specific paths
- ✗ Plex preplay/details screen format
- ✗ Direct Tunarr HLS stream URLs
- ✗ IPTV app deep linking

**Workarounds**:

### Option 1: Use Regular Plex Libraries (Recommended)
If your Live TV content is also available in regular Plex libraries (Movies, TV Shows), map NFC cards to those library items instead. This works perfectly with the existing NFC system.

Example: Instead of mapping to "Channel 3 - The Grinch", map to the movie "The Grinch" in your Movies library.

### Option 2: Use Different Hardware for Live TV
Use a device with better m3u8 stream support for Live TV channels:
- **Chromecast/Google TV**: Better support for playing m3u8 URLs via Home Assistant
- **Fire TV**: Can play streams via Kodi or VLC
- Keep Apple TV for regular Plex content only

### Option 3: Remote Control Navigation (Not Recommended)
Theoretically possible to use `remote.send_command` to navigate Plex's UI with button presses, but:
- Extremely fragile (breaks when Plex updates UI)
- Requires precise timing
- Must map exact button sequences for each channel
- Only works reliably with <20 channels

**Related Issues**:
- [Home Assistant Community: Plex Play Specific Live TV Channel](https://community.home-assistant.io/t/plex-play-specific-live-tv-channel/920312) (August 2024)
- [Home Assistant Community: Plex media_player.play_media call to play Live TV](https://community.home-assistant.io/t/plex-media-player-play-media-call-to-play-live-tv-channel-from-tuner/346069) (2021)

**Status**: This is a platform limitation, not a TapeDeck bug. Apple TV's restrictive app ecosystem prevents the automation of Live TV playback via Home Assistant.

**Last Updated**: December 2024
