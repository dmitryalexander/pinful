# Pinful

Pinful is a customized [Pinless](https://github.com/bunk-im/pinless) fork for browsing public Pinterest images and videos in a compact, dark wall. It adds search groups, mixed discovery feeds, local collections, history, hidden pins and hover controls. The interface uses JavaScript.

**Consolidated release source.** Browse search groups, playlists and pin pages without requiring Bravely or a Pinterest login. When Pinterest provides no recommendations, Pinful can reuse images from the page you came from, with the selected pin enlarged on the left. These surrounding images are navigation fallback content, not recommendations generated for that pin. Pinterest compatibility varies; see [release readiness](docs/RELEASE_READINESS.md).

[Bravely](https://github.com/dmitryalexander/bravely) is an optional companion that brings the wall interface to Google Lens and supported art sites. Both projects operate independently.

## Run locally

Install Docker with Docker Compose (Docker Desktop on Windows), then run from this folder:

```sh
docker compose up -d --build
```

Open **http://localhost:3001/?pt_page=lists**, or run **Start.cmd** on Windows.
This builds Pinful from source and binds only to your computer. Use `PINFUL_PORT`
in a private `.env` to change the port. Keep a consistent hostname and port.

For a separate development instance set `COMPOSE_PROJECT_NAME=pinful-preview` and
`PINFUL_PORT=3002`. Each project uses its own containers, image and data volume.
Existing installations may set `PINFUL_DATA_VOLUME` to their existing volume;
legacy `pinless-*` data keys remain unchanged. See [release notes](RELEASE_NOTES.md).

```sh
docker compose logs --tail=100 pinful
docker compose stop
docker compose start
```

After source updates, run `docker compose up -d --build` again and hard-refresh existing tabs. Templates and static assets are embedded into the Go executable; editing a file requires a rebuild.

## Controls

The in-app Menu explains collections, groups, history and hidden pins. New installations contain no starter search terms.

## Optional Pinterest connection

Open **Menu → Pinterest connection**. Log in on Pinterest itself, then follow the form's instructions to copy your own `_pinterest_sess` cookie value into the local masked field. Pinful never asks for your Pinterest password. Treat the session value as an account credential; enter it only in your own local installation, never in chat or an issue report.

The server checks Pinterest's authentication flag before accepting the session. Connected means a fresh authentication check succeeded, including when opening Menu; a saved value alone never counts as connected. Saved sessions that fail verification show Not connected, with Check, Replace, Save hash and Clear hash still available. It then uses the saved session for website-resource requests, including new searches, pin details and related pins. Connected search/detail/recommendation flows have been tested, but a connection does not guarantee recommendations for every pin. Some pins return images anonymously and none when connected; others return none either way. Run a fresh search after connecting; older encrypted IDs may still fail.

The last verified connection is saved in a private `pinterest-session.json` file in the data volume and restored after server restart. It is shared by tabs of this single-user installation. It is excluded from app-data exports and backups. Menu checks the saved connection with Pinterest automatically; **Check connection** checks it again. Clear hash deletes the saved connection. Clear hash does not revoke the Pinterest browser session or delete pins already saved locally. Expired sessions require reconnection. Requests with non-local hostnames are refused; use localhost, 127.0.0.1 or ::1.

| Hover action | Control |
| --- | --- |
| Google Lens for an image/GIF | Blue strip or G |
| Open a pin page | O; video blue strip |
| Open full-size image; download/cancel video | Red strip or D |
| Play/pause video | Space |

Typing fields and open dialogs suppress the hover shortcuts. Illustrated tutorials are planned; screenshots are not yet included.

## Anonymous pin recovery

Search and recommendation clicks can recover some encrypted `e1_` pins automatically. Pinful checks Pinterest's original-pin canonical reference or a unique exact-image match in the source board's public embed, then verifies the numeric pin's image, board and creation time. Verified pins open their numeric Pinful page and use Pinterest's actual recommendations.

The board route covers only the embed's first 50 pins; original canonical references can recover some pins beyond that window. Pins without sufficient source metadata or a verified match can show their retained image and reuse images from the preceding search, recommendation wall, playlist or group feed. Native recommendations take priority. Saved collections are not rewritten.

## Saved data

The default Compose volume `pinful_data` stores `/data/store.json`, daily snapshots and first-sync browser backups. Playlists, history, hidden pins and search groups are mirrored from browser storage. Internal `pinless-*` keys and `PINLESS_DATA`/`PINLESS_LISTEN` environment variables are retained for compatibility. Legacy data is not imported automatically.

Stop the service before backing up the entire named volume using your Docker volume backup tooling. Restore into a stopped installation and keep a separate original backup. `docker compose down` keeps the volume; **`docker compose down -v` deletes it**. The installed legacy `pinless-data` volume is separate.

## Privacy and hosting

This is a local, single-user application without local access authentication. Anyone able to reach it can access the saved-data API; do not expose this configuration publicly. Requests to Pinterest come from the server: when self-hosting locally, that is your connection. With the optional connection, Pinterest can associate these requests with your account. Lens and external links open third-party sites directly. Pinful does not promise anonymity or prevent those sites from tracking visits.

## Development checks

Node.js 20 or later is needed only for the dependency-free JavaScript checks:

```sh
npm test
docker build --target test -t pinful-tests .
docker build -t pinful:local .
```

The test image runs `go test ./...` with FFmpeg available. The normal final-image build does not run the test stage. Alternatively, install Go 1.25.6 and FFmpeg and run `go test ./...` locally.

## Save, transfer and reset

Menu now includes **Save hash / Load hash**, plus **Export info / Import info /
Clear everything**. A hash file is a private, reusable credential file; keep it
separate from ordinary info exports. Loading a hash saves it even while Pinterest
is unavailable, but only Check connection can mark it authenticated. Expired
cookies still need replacement.

Info exports contain collections, searches, follows, history and settings.
Import replaces the current app information after confirmation and preserves the
saved connection. Clear everything requires typing `CLEAR EVERYTHING`, removes
the saved connection, app information and automatic app backups, and clears
browser restoration state and remembered wall contexts. It does not remove
downloaded media files.

Imports and resets reject stale writes. Ordinary collection saves compare per-item
revisions; conflicts pause saving and let you download your unsaved copy and choose
the saved or local version. A damaged store also pauses saving: use Menu → Recover
saved data to select an automatic copy. Interrupted import/reset operations resume
from durable intent before further saves are accepted.

## Credits

Retains the upstream [GNU AGPL v3 license](LICENSE). See [provenance and third-party notices](docs/PROVENANCE.md). Pinful is not affiliated with Pinterest. Media remains the property of its respective owners.


## Browsing fallback

Fallback pages keep the selected image in the top-left at the normal two-column pin-page size. Repeated image signatures are excluded. Search/recommendation fallback pages can continue loading from their original source; playlist and mixed-feed snapshots contain up to 80 images. There are no unresolved purple borders or fallback labels in the interface. The old Pinful-to-Bravely Lens button and callback integration have been removed.

Normal clicks stay in Pinful. **Ctrl-click** (Command-click on macOS) opens real and fallback pins in a new Pinful tab, preserving the source-wall context. The blue image strip uses its normal Google Lens action; it has no special Ctrl-click action.

Source-wall navigation contexts hold the 256 most recent snapshots in server memory. They are not saved recommendations and disappear on restart or eviction. After updating/restarting Pinful, refresh the source search/feed/playlist and reopen the pin. Direct links without a source context can still show an image-only recovery page when Pinterest returns no recommendations. A one-image source has no surrounding images to reuse.

Automatic daily copies keep the latest 14 days. First-sync copies keep at most
20 files, 128 MiB total, and 30 days of history (always retaining the newest copy).
These are recovery aids; use Export info and Save hash for your own backups.

Menu’s **Temporary storage** shows reusable browser data and app cache usage.
**Clear temporary data** removes only recreatable cache and this tab’s Restore
Feed snapshot. Collections, follows, history, settings, login hashes and recovery
backups are preserved. Browser-managed HTTP media cache and saved downloads
are not included. Browser byte counts estimate UTF-16 storage. Temporary download files are
removed after each download and at startup; cleanup waits for active downloads.
