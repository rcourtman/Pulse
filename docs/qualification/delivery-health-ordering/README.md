# Delivery health request ordering — 5 September 2026

Run from repository root after root and frontend `npm ci --ignore-scripts`:

    pulse-heavy-run -- node scripts/check-delivery-health-ordering.mjs

Twelve real Chromium cases passed at 1440×900 and 390×900. The fixture imports
the production destinations caller, health hook, warning card and CSS; only API
responses, configuration props and confirmation acceptance are scripted.
It does not serve the complete application or exercise an installed backend.
The temporary Vite fixture is never part of the product bundle.

Four success/error orderings per width preserve latest attention and rendered
state. Two action cases per width exercise retry/dismissal overlap: an older
completion cannot clear loading while the post-action read remains pending,
and the latest healthy response clears attention. All 19 focused hook, caller
and card tests pass; reverting only the runtime hook produces nine failures in
the 16 hook/caller tests. TypeScript and changed-file ESLint pass.

The first browser attempt failed before UI verification because the isolated
Vite configuration omitted the repository's esnext dependency target. The
retained runner corrects that and uses the frontend working directory for CSS.
`browser-result.json` is the successful rerun's output; hashes bind the runner
and runtime source. Representative screenshots show actual fixture rendering.
Warning titles appear pale in this isolated CSS context; this receipt qualifies
ordering, not whole-app contrast or accessibility. Full-shell visual review,
installed delivery, release-line applicability and exact-candidate integration
remain separate qualification work. No public release is claimed.
