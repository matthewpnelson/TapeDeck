# Known Limitations

## Plex tvOS App Ignores Deep Links (2026.18 and Later)

**Issue**: A tap opens the Plex app on the Apple TV, but the media doesn't play.

**Why**: Plex shipped a redesigned tvOS app in 2026.18.0 on 16 Sep 2026. It accepts the `plex://play/` link and comes to the foreground, but it never requests the item from the Plex server. It shows its last screen, Home, or the "Who's Watching?" profile picker. Home Assistant returns 200, so TapeDeck logs `Playback started` for a tap that does nothing.

Other findings from 25 Oct 2026:

- 2026.19.0, 2026.19.1 and 2026.20.0 have no deep link fix in their release notes.
- The app isn't a Plex remote-control player. It isn't listed with `provides=player` on plex.tv, and it doesn't answer on port 32500. The Home Assistant Plex integration can't control it.
- The app has no setting that affects this.

**Workaround**: Set `player: plex-infuse-fallback` in `config.yml`. See [Deep Links](home-assistant-setup.md#deep-links). Taps play in Infuse until Plex fixes deep links, and then they play in Plex again.

**Limits of the workaround**:

- Playlist cards have no Infuse link, so they still don't play.
- Infuse plays as one Plex user, the one signed in to its Plex share. A card's Plex user only affects which next episode TapeDeck picks.
- While Plex ignores links, each tap waits about 10s before Infuse starts: 5s to wake the Apple TV, then 5s for Plex.

**Last Updated**: October 2026

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
- ✗ Tunarr streams through Infuse `infuse://x-callback-url/play?url=` (October 2026). Each Tunarr mode failed:
  - HLS, HLS Alt, HLS Direct and HLS Direct v2: "An error occurred loading this content"
  - MPEG-TS: "Server does not report the size of the file"
  - Infuse ignores a URL that ends in `.m3u8`
  - The segments are H.264 and AAC, so the stream format isn't the cause
- ✗ Plex Live TV in Infuse. Infuse doesn't show Plex Live TV.

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

**Last Updated**: October 2026
