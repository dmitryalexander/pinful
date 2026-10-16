# Pinful

Pinful is a customized [Pinless](https://github.com/bunk-im/pinless) fork for browsing public Pinterest images and videos in a compact, dark wall. It adds search groups, mixed discovery feeds, local collections, history, hidden pins and hover controls. The interface uses JavaScript.

**Development preview.** Browse search groups, playlists and pin pages without requiring Bravely or a Pinterest login. When Pinterest provides no recommendations, Pinful can reuse images from the page you came from, with the selected pin enlarged on the left. These surrounding images are navigation fallback content, not recommendations generated for that pin. Pinterest compatibility varies; see [release readiness](docs/RELEASE_READINESS.md).

[Bravely](https://github.com/dmitryalexander/bravely) is an optional companion that brings the wall interface to Google Lens and supported art sites. Both projects operate independently.

## Run locally

Install Docker with Docker Compose (Docker Desktop on Windows), then run from this folder:

```sh
docker compose up -d --build
```

Open **http://localhost:3002/?pt_page=lists**. This configuration builds the actual Pinful source and binds only to your computer. Port 3002 is the standalone default; the existing installed Pinful may use port 3001. To change it, set `PINFUL_PORT` (legacy `PINFULL_PORT` is also accepted) in a local `.env` file. Use the same hostname and port consistently because browser storage belongs to that origin.

The app, Docker Compose project, service and container are named **pinful**; the image is `pinful:local`. The existing backing volume name `pinfull_pinfull-data`, legacy `PINFULL_PORT` setting, internal `pinless-*` storage keys and preparation folder name remain compatible so the rename does not discard saved data. Run one instance at a time with the `pinful` container name.

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

The server checks Pinterest's authentication flag before accepting the session. It then uses that session for website-resource requests, including new searches, pin details and related pins. Connected search/detail/recommendation flows have been tested, but a connection does not guarantee recommendations for every pin. Some pins return images anonymously and none when connected; others return none either way. Run a fresh search after connecting; older encrypted IDs may still fail.

Credentials stay in server memory, are shared by tabs of this single-user installation, and disappear on Disconnect or server restart. They are not written to app-data or backups. Disconnect does not revoke the Pinterest browser session or delete pins already saved locally. Expired sessions require reconnection. Requests with non-local hostnames are refused; use localhost, 127.0.0.1 or ::1.

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

The Compose volume `pinfull_pinfull-data` stores `/data/store.json`, daily snapshots and first-sync browser backups. Playlists, history, hidden pins and search groups are mirrored from browser storage. Internal `pinless-*` keys and `PINLESS_DATA`/`PINLESS_LISTEN` environment variables are retained for compatibility. Legacy data is not imported automatically.

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

## Credits

Retains the upstream [GNU AGPL v3 license](LICENSE). See [provenance and third-party notices](docs/PROVENANCE.md). Pinful is not affiliated with Pinterest. Media remains the property of its respective owners.


## Browsing fallback

Fallback pages keep the selected image in the top-left at the normal two-column pin-page size. Repeated image signatures are excluded. Search/recommendation fallback pages can continue loading from their original source; playlist and mixed-feed snapshots contain up to 80 images. There are no unresolved purple borders or fallback labels in the interface. The old Pinful-to-Bravely Lens button and callback integration have been removed.

Normal clicks stay in Pinful. **Ctrl-click** (Command-click on macOS) opens real and fallback pins in a new Pinful tab, preserving the source-wall context. The blue image strip uses its normal Google Lens action; it has no special Ctrl-click action.

Source-wall navigation contexts hold the 256 most recent snapshots in server memory. They are not saved recommendations and disappear on restart or eviction. After updating/restarting Pinful, refresh the source search/feed/playlist and reopen the pin. Direct links without a source context can still show an image-only recovery page when Pinterest returns no recommendations. A one-image source has no surrounding images to reuse.
