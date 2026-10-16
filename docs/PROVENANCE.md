# Source provenance

Pinful was extracted on October 16, 2026 from the owner's customized Pinless working source. The upstream is https://github.com/bunk-im/pinless. The supplied working tree has no Git history or recorded exact upstream commit; this extraction does not claim an upstream revision.

Included: root Go source/tests, go.mod/go.sum, LICENSE, .gitattributes, static assets and templates. Project-relative JavaScript regression harnesses cover the current features; historical backup comparisons are excluded. Build configuration, CI and project documentation were prepared for this standalone tree.

Excluded: backup folders, installed launchers, build stamps, container metadata, saved data, old upstream instance lists and workflows, personal screenshots and the surrounding workspace history.

The upstream LICENSE is retained unchanged (GNU AGPL version 3). The Go module remains `pinless` to minimize unnecessary compatibility changes. go.mod/go.sum record Go dependencies. Existing Gin and Lucide notices remain in templates/licenses.html. Vendored hls.js is static/hls-1.7.3.min.js with static/hls-LICENSE.txt retained unchanged.

The static/warning.png skull cassette artwork was created by jrpgloveless. The owner confirmed authorship on October 16, 2026 while preparing this repository for public upload. Credit: cassette artwork by jrpgloveless. It is supplied as part of this project under the repository license.

Local changes include JavaScript wall layout, search groups, collections/history/hidden pins, server-backed data storage, video/HLS handling, downloads, hover shortcuts, optional Bravely links and pin-error/media-recovery handling. The preparation tree additionally changes visible branding, local build configuration and documentation.
