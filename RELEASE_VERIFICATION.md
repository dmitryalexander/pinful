# Release verification — October 20, 2026

- Pinful: Go regression suite and JavaScript/template checks passed.
- Individuous: 172 upstream Crystal examples, 21 custom JavaScript tests,
  eight synthetic session-service tests, five HTTP storage/restart/retention tests and
  PowerShell first-run/idempotence checks passed.
- Portable integration checks passed: account/collection round trip, native Int64
  playlist positions, invalid-import rollback, foreign origins, stale revisions,
  interrupted reset/restart recovery, damaged-file recovery, and simultaneous
  native account mutations versus reset.
- A fresh isolated installation passed real browser conflict handling and
  resolution, fetch/XHR epoch forwarding and native preference persistence.
- Four deployed instances passed Menu hash/info controls, confirmations,
  cancellation, wrong-app rejection and desktop/mobile layout checks. Destructive
  actions were mocked on existing installations. No real user reset/import ran.
- Existing app-data, account database and saved-session fingerprints matched
  before and after deployment. Installed volumes and private configuration kept.
- Source archives exclude .git, .env, stores, sessions, caches and installed-only
  material; scanned against the actual private configuration values. Build inputs
  are portable and include the pinned upstream test fixtures and license files.
- Both clean source archive builds were checked independently of the working trees.

These checks were completed before publication; published commits are recorded in Git history. External service availability and the
validity of an owner's live Pinterest/YouTube credential are not guaranteed by
synthetic tests. This is a targeted review of the custom code and release setup.

Cache audit: targeted mixed-asset, player/style/art refresh, unavailable-storage,
Back/Forward saved-data/epoch and pending-edit preservation regressions passed.
