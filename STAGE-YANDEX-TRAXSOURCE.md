# CCML — Yandex Music / Traxsource metadata update

This update is intended to be extracted over the current CCML project after the Metadata Settings update.

## Added
- Experimental Yandex Music metadata source. Yandex does not publish a supported third-party Music API; the provider uses the private catalog endpoint used by Yandex clients. OAuth token is optional and compatibility may change.
- Experimental Traxsource public web-catalog source. Traxsource does not publish a developer/token API for this lookup; the source is disabled by default.
- Provider-page buttons in Settings for API documentation / credential registration for all supported sources.
- Whitelisted `OpenMetadataLink` backend binding; arbitrary URLs cannot be passed from the UI.

## Recommended verification
```powershell
go test ./...
cd frontend
npm run build
cd ..
wails dev
```

Open Settings -> Metadata sources, enable the desired providers, save, then test Find metadata on a known commercial track.
