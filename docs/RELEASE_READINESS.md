Historical initial-upload checklist; current consolidated-release results are in ../RELEASE_NOTES.md and ../RELEASE_VERIFICATION.md.

# Initial source commit readiness

Pinful is an initial **development preview**, not a stable release or a universal Pinterest resolver. The final clean-export checks below passed for this initial source snapshot. Publishing a stable release remains a separate decision.

- [x] Standalone source; personal data, local backups, launchers, probes and private screenshots excluded.
- [x] Local Docker build, loopback-only port, persistent saved-data volume and CI configuration.
- [x] Pinful display branding, upstream AGPL license and third-party/artwork provenance retained.
- [x] Real numeric recommendations/pagination and some verified opaque-ID resolutions tested.
- [x] Connected and anonymous comparisons performed; per-pin upstream limitations documented.
- [x] Seamless previous-wall/search/playlist/group-feed fallback, enlarged selected image, saved hover controls and local Ctrl-click navigation implemented.
- [x] Old purple-status UI and Pinful Lens helper integration removed from the active interface.
- [x] Owner confirmed the separate Bravely missing-image fix; Bravely is not required by Pinful.
- [x] Final clean staged export: JavaScript and Go tests, production build, exclusion/credential-marker and whitespace checks.

Before describing a stable release: complete the broader owner walkthrough (especially video/HLS downloads, mobile/narrow layouts, repeated tab navigation and restart recovery); add tutorial screenshots if desired. Existing smoke tests and individual owner checks do not prove all these flows.

Known limitations: some Pinterest pins yield no recommendations even when connected; original opaque-ID recovery covers only verified cases. Navigation fallback reuses prior-page media, not pin-specific recommendations. Context snapshots and connected credentials are memory-only; direct or expired context URLs may show image-only recovery. The app is local/single-user, without access authentication, and must not be exposed publicly.

See [verification](VERIFICATION.md) and [provenance](PROVENANCE.md).
