# Pinful — consolidated release, 2026-10-20

This source combines the installed functionality and the latest development UI.
Start.cmd (Windows) or Docker Compose builds this repository independently.
Keep your private .env and data volumes when upgrading. Never publish either.

## Fixed in this release

- Save/load reusable login hashes, separate from ordinary info exports.
- Menu info export/import and confirmed Clear everything.
- Per-item compare-and-save revisions prevent silent collection overwrites.
  Conflicts preserve unsaved browser data until an explicit choice; download
  that copy before discarding either version. Unrelated collection edits still save.
- Damaged stores fail closed and offer explicitly selected recovery copies.
- First-sync copies are bounded to 20 files, 128 MiB and 30 days, retaining the
  newest; daily recovery retains 14 dates.
- Durable operation journals finish interrupted import/reset operations before
  new data is accepted. Never manually delete an unfinished operation.json.
- Compose instances have independent names, image tags and volumes.

## Existing installations

Set PINFUL_DATA_VOLUME to the existing volume before starting an upgrade.
Set PINFUL_SOURCE when a deployment wrapper should build this source tree.
Use COMPOSE_PROJECT_NAME and PINFUL_PORT for a separate development instance.
Internal PINLESS_DATA and pinless-* keys remain compatible.

## Verification scope

See [RELEASE_VERIFICATION.md](RELEASE_VERIFICATION.md) for exact test results.
Tests use synthetic credentials and disposable stores. Browser destructive actions
are mocked on installed apps. No real reset/import is part of the upgrade.
External Pinterest/YouTube availability and expired credentials remain outside
this application's control. This was a targeted review of custom code and build
configuration, not an audit of every upstream dependency.

Cache audit follow-up: Back/Forward restored pages recheck saved-data revisions and
reset epochs while preserving pending edits.
Every template now uses a content-derived interface asset version. HTML/data
responses are no-store; interface files revalidate while media keeps its own policy.
