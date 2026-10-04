# Final v6.4 line stylesheet

Status: Implemented locally; exact-graph checks, browser comparison and strict audit pending.

This is a qualification-only route for the directed last v6.4 release, not the
toolchain for main or later releases. The required full dependency audit cannot
accept the obsolete Tailwind 3 and jscpd 4 paths through braces. Main's Tailwind 4
migration also changes hundreds of callers; copying it into this frozen line is
unnecessary product change.

`src/final-line-styles.generated.css` is the complete Tailwind/PostCSS output of
the exact committed line `7d7e956077bb2d1514f7828a74923ca1f10e739f`, not a manually
translated stylesheet. The existing runtime imports it instead of recompiling
`src/index.css`. Authored CSS and the old configuration remain provenance inputs,
not build-time compiler calls. The new graph removes the CSS compiler and uses
the provenance-verified jscpd 5 graph. It preserves all seven runtime roots.

`final-line-styles.json` binds the output bytes, original generator identity and
all current non-test style consumers. Both Vite startup and the embedded-asset
build fail if the snapshot, a consumer, authored CSS or configuration drifts.
This prevents silently omitting a new class after a source edit. Test-only files
can change without regenerating product styles. Do not update hashes to make a
failed build pass: a product change needs exact-source regeneration and fresh
qualification. Subsequent releases cut from main use main's normal compiler.

Generation used the assigned offline source-proof VM, the original complete npm
graph and `tailwindcss --postcss --input src/index.css --output <temporary-file>`.
The source-proof identity and the derived byte hash are in the snapshot. Generation
does not establish the candidate build, browser rendering, a fresh advisory audit
or release qualification. The required full audit remains unchanged. Before
admission, compare the actual baseline and candidate App routes and controls on
desktop/phone in both themes, inspect screenshots, run the complete source checks
with the matching new graph, and record the exact-parent browser receipt.
