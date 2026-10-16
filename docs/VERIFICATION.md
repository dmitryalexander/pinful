# Current verification — October 16, 2026

The entries below the historical heading record development stages, including features subsequently removed. The current preview no longer loads purple-status UI or Pinful's Lens helper/jobs/callback; Bravely remains independent.

Current targeted checks include native/empty recommendation paths; prior recommendation and filtered search continuation; playlist snapshot validation and selected-image recovery; mixed group-feed/Restore/History handoff; duplicate-image filtering; bounded independent page contexts; enlarged top-left selected pin; self-link context preservation; saved-hover-compatible relative links; and Ctrl-click opening local contextual tabs while leaving the original tab unchanged.

Live HTTP checks exercised search-to-fallback navigation, a real active saved playlist (saved app-data unchanged), the reported group-feed pin, repeated selected-image self-navigation and deployed asset versions. Navigation JavaScript was also exercised with fixtures; these are not claims of a complete native Brave walkthrough. Connected-session comparisons showed both successes and successful empty upstream responses.

Final initial-commit check: exported all 91 staged files with git checkout-index into a new clean folder. All nine JavaScript harnesses/syntax/asset checks passed; go test ./... passed in the existing Go/FFmpeg test image with the export mounted read-only; the standalone production Dockerfile built successfully as pinful-initial-upload-candidate:local. An isolated temporary container began with empty app-data, rendered Pinful with the renamed source link, accepted and returned a disposable history record, then was stopped/removed. The installed container/data were untouched.

The staged export contained no saved-data/backups/probes/executables or local environment files. Common private-key/GitHub-token/AWS-key/local-workspace-path marker scan and staged whitespace check passed; these are limited checks, not a comprehensive security audit. License and artwork provenance are retained. Only documentation was updated after these runtime checks to record the results.

Repository renamed to https://github.com/dmitryalexander/pinful (same repository ID); local origin and About source link updated. This initial development-preview source snapshot was prepared from those checks; this preparation did not create a stable release or version tag.

Docker naming follow-up: Compose project/service/container now use pinful and the production image pinful:local. Compose validation passed with the existing standalone volume explicitly mapped as pinfull_pinfull-data, preserving its data across the project rename. The installed app separately retains its existing pinless-data backing volume. Legacy volume names are storage compatibility identifiers, not additional app containers.

## Historical development evidence

# Preparation verification — October 16, 2026

Passed:

- Dependency-free JavaScript syntax, four regression harnesses and referenced-template-asset checks.
- Docker test stage: `go test ./...` with FFmpeg.
- Production Compose image build.
- A clean export of the staged files into a separate folder: JavaScript checks and Docker Go test stage passed independently.
- Isolated Compose project `pinfull-prep-check`, localhost port 3002: landing page branding, initially empty saved-data API, disposable record save and persistence across restart.
- Numeric control pin 3659243441431987 returned a page without detail/related warnings. This was an HTTP check, not a browser walkthrough.
- Staged whitespace check and source scan for local workspace paths/common credential markers. No backup folders, saved data or private screenshots were included. Such a scan is not a comprehensive security audit.

The check container/network were stopped and removed. The separate `pinfull-prep-check_pinfull-data` volume retains only the disposable smoke-test record. The installed legacy container/source/data were not replaced or modified.

## Upstream comparison

Downloaded unmodified https://github.com/bunk-im/pinless at commit `87b560c911186b64b6337baa2e64658977891c7e` (August 6, 2026) into a separate comparison checkout. This is the revision compared, not a claim that the customized source originated at that exact revision.

Built its own Dockerfile and ran it on localhost port 3003. Its sunflower search returned 16 pin links, all encrypted `e1_` IDs. Opening the first two using the same local web session yielded zero featured images and zero related images, with upstream's “No related pins found” message. Numeric control 3659243441431987 yielded one featured image and 12 related images. The upstream process returned HTTP 200 even for the empty failed pages.

Both versions use BaseSearchResource for search, PinResource for details, and RelatedModulesResource for related pins. Upstream has no encrypted-ID resolver. Pinfull adds the detailed field set, media handling and explicit failure detection/recovery. Upstream omits that field set and still fails, so removing it is not a demonstrated fix.

Direct public-resource checks also returned HTTP 404/code 50 for a freshly returned encrypted search pin. Evidence supports a shared upstream compatibility problem; it does not establish deliberate targeting of Pinless. Numeric success also argues against a blanket block of every request on this connection.

Still unverified/unresolved: encrypted details/related success, real browser walkthrough, illustrated tutorials and recovery-page layout refinement. Do not call this first-upload candidate ready yet.

## Experimental Pinterest connection — October 16, 2026

Added Menu → Pinterest connection after the owner chose a local session form. The form opens Pinterest's own login page and explains copying the owner's _pinterest_sess and optional csrftoken values into masked local fields. It does not request a password or automatically inspect browser cookies. Credentials are cleared from the form after submission and retained only in server memory, outside saved-data storage/backups. They apply to this local single-user server's resource requests; Disconnect/restart removes the connection. Saved pins and the Pinterest browser login remain intact.

Tests cover origin/hostname restrictions, malformed payloads, explicit upstream authentication verification, failed replacements preserving a connection, Disconnect winning an in-flight validation, exact credential destinations, refused redirects, detail/related header propagation, and the actual JS form's submission/clearing/error/disconnect behavior. These use synthetic sessions. Real signed-in recommendation recovery remains unverified and the UI explicitly says so.

Live deployment additionally verified that the explicit session status route returns JSON without Gin's profile redirect, the Menu includes its assets, foreign-origin mutations are rejected, and a synthetic invalid session receives HTTP 401. Existing installed data volume was retained. No real session was supplied, so authenticated recommendations remain pending owner verification.

## Related pagination fix — October 16, 2026

Owner screenshots establish initial signed-in numeric pin details and related results for their examples. Investigated a distinct continuation failure: related requests switched to POST on page two, yielding HTTP 403; GET with the same continuation returned results. Kept related pagination on GET. Go tests passed, including a three-page regression with encoded cursors and end handling. Live deployed HTTP checks retrieved three pages; the browser wall grew from 13 to 24 to 52 cards. Verification after restart used anonymous numeric pins; owner must reconnect for authenticated continuation verification. Existing encrypted anonymous-ID limitations remain. Search/profile pagination was not changed.

### Search pagination follow-up (October 16, 2026)

Search continuation now keeps GET instead of switching to POST. The Go suite passed with a three-page regression checking query/scope, encoded bookmarks and terminal handling. Installed production build passed. Live anonymous Titanfall searches on localhost:3001 returned 18, 24 and 25 new unique pin links across three HTTP 200 pages, each with a continuation link. Signed-in browser continuation remains an owner check after reconnecting the memory-only session following deployment. Existing saved data volume was retained.

### Favorites and session help (October 16, 2026)

Favorites now progressively renders 40-item batches; shuffling still reorders the full saved list, then restarts progressive rendering. Search shuffle remains limited to loaded cards. The 105-item batch/reorder/empty regression passes alongside the other five JS harnesses. Session UI now has one masked input and a sanitized illustrated copying guide. Production image builds and deployed asset checks pass. No live user collection was modified during testing.

### Opaque pin investigation and partial repair

An isolated anonymous Go request succeeds for search with website headers and returns an opaque ID with is_eligible_for_web_closeup:false. Detail lookup for the exact ID fails with HTTP404/code50 across three field-set variants; the aggregate node ID is not a working pin ID either. Metadata preservation and compact single-pin fallback are repaired, but opaque detail/related resolution remains open. Full Go suite (including string/object preview metadata regression), six JS harnesses and production build passed. A fresh local anonymous search/detail pair confirmed retained metadata and compact fallback. No successful opaque recommendations claim is warranted.

## Automatic anonymous resolver — October 16, 2026

Implemented pin_resolver.go and retained pinOrigin evidence (source board, creation time, image signature and canonical reference) in search and related previews. A verified mapping redirects locally to the numeric pin URL with the complete query intact. This keeps recommendation continuations attached to a numeric seed even after the preview cache expires. Saved collections/history are not rewritten. Caches bound board embeds to 256 entries and opaque mappings to 4096; successful entries expire after one hour, failed lookups after two minutes. Per-key work is deduplicated, with at most eight concurrent loads per cache. Widget and candidate verification requests always use anonymous clients with four-second request timeouts and refused redirects.

The resolver first uses a canonical numeric URL only when canonical_pin.id equals the original opaque ID. It otherwise tries the public source-board embed. Both paths independently require the numeric response ID, image signature, actual media filename signature, source board ID and creation timestamp to match. Other canonical posts, near-duplicate images and ambiguous board candidates are rejected.

Go suite passed in the Docker test stage with FFmpeg; all six JavaScript harnesses and syntax/asset checks passed. New tests cover automatic local redirects/query preservation, numeric recommendation pagination after preview expiry, downstream source metadata, ambiguity, mismatched board/time/image/ID, widget errors/redirects, missing or unsafe source paths, shared concurrent work, bounded/expiring caches and canonical references that bypass an empty embed. These are fixture tests.

Live HTTP checks additionally used fresh sessions and verified connected:false on disposable localhost:3004 and installed localhost:3001. Search sunflower -> e1 -> local numeric1051379475531452298 rendered full details and recommendations, a follow-on e1 resolved to its own working wall, and page two loaded. Unresolvable children remained local with recovery states. The final canonical-enabled build also resolved e1_AfqPShziYHZEeAGM08m8B3BmzqdBg5CNoXrtBChXgrhExoomk7VD to numeric407575835008450785, keeping its exact image, and loaded two recommendation pages. Its 968-pin source board's embed contains50 pins and does not contain that image. The canonical reference identifies the same original opaque pin, and the numeric response matches board and creation time. This proves an actual outside-window case, not universal coverage. HTTP link counts include the featured pin; 13 links represent12 related pins plus the featured link. No real-browser walkthrough or user data mutation was performed.

Deployment retains pinless-data:/data and127.0.0.1:3001->3000. Memory-only connection cleared on restart. Previous image is tagged pinless-hls-preview:before-board-resolver; source backup is Technical files/Pinless downloads/backup-before-board-resolver. Standalone and installed source files are synchronized. Live scripts and response evidence remain outside the release repository under release-prep/probes.

## Purple recommendation-status borders — October 16, 2026

Visible media tiles are checked in the background, with hovered/focused tiles prioritized. A purple border identifies a currently unresolved link or a confirmed empty related response. A tooltip distinguishes the two. Unknown/failed/unchecked results keep their normal appearance. Checks are sequential and cached; server admission limits new probes across tabs. Numeric checks request a single related pin, and count actual upstream pin records independently of whether their media can be rendered.

Full Go suite and seven JavaScript harnesses pass. Live installed HTTP checks confirm template assets, purple CSS, invalid input, unresolved/cache state, numeric available state and verified original-canonical available state. The JS behavior test executes the actual client script with DOM/observer/fetch fixtures. No real browser rendering verification was performed. Saved-data volume retained on deployment.

## Pinful name update — October 16, 2026

Displayed app/shortcut branding is now Pinful, including dynamic Favorites/History titles. Existing storage keys, Compose/runtime identifiers, preparation path and GitHub source URL retain their legacy names for compatibility. Seven JS harnesses, Go suite and installed/standalone production builds passed. Live root/About HTML and dynamic-title script use Pinful; purple-border assets remain available. This update does not implement the proposed Google Lens board fallback.

## Lens workaround and hover-only status — October 16, 2026

Implemented temporary Lens jobs, scoped extension callbacks, independently verified exact-image pin/board candidates, and matched-post recommendation pagination while retaining the original pin and an explicit workaround label. Purple now appears only on hover; the original workaround image remains marked unresolved. No image uploads occur on page load/hover, and no saved records are rewritten.

Full Go suite, eight Pinful JavaScript harnesses and seven Bravely harnesses pass. A live backend test supplied a fixture Lens-result URL and verified real public Pinterest matching/recommendations, labelled original page and page two. It did not exercise Google/browser result extraction. A direct public-image Lens HTTP upload produced only a JavaScript search shell.

Bravely0.15.0 and the installed old Lens Wall0.12.4 bridge files are prepared. A real installed browser roundtrip remains unverified: Windows computer-use stopped because it could not confidently determine Brave's current URL. Reload the currently enabled extension at brave://extensions and hard-refresh Pinful to activate. No new optional host permission is required.

October16,2026: nine JS harnesses and full Go suite pass. Lens recovery hero uses one normal top-left cell. Live fixture-result backend check covers pending/ready layout, actual Pinterest recommendations and page2. Full Google/Bravely browser handoff remains unverified until extension reload/new search.

October16,2026: owner clarified the Lens fallback should import images from the Pinterest result page even without an exact-file match/API recommendations. Full Go and nine JS harnesses pass. Live Google result491455378103008822 and its rendered Pinterest images imported through actual browser callback: original+12 tiles, all13 loaded. Bravely0.17.0 helper behavior covered by nine extension harnesses; automatic live extension handoff remains unverified in the in-app browser. Screenshot/evidence are in the parent release-prep/probes folder.
