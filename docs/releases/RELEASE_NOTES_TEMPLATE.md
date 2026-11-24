# Pulse vX.Y.Z Release Notes

One short paragraph explaining the customer outcome of the release. Lead with
what feels better or works now, not how it was implemented.

## Highlights

- One to three of the release's biggest changes. Name the specific feature or
  fix and where users find it, starting with the page or feature and then
  saying what it now does. Describe only changes in this release, never a
  theme or benefit slogan such as "See problems and trends together" or "Spend
  less time on setup".
  Keep each under 140 characters with no Markdown, links or issue references.
  The update banner shows these before users install, so each must make sense
  on its own.

## Proxmox, PBS and backups

- One short, plain sentence explaining what users will notice and why it matters.

From v6.4.6 onward, group related changes under the relevant reader-facing
headings: Alerts and notifications, Disks and storage, Proxmox, PBS and backups,
TrueNAS, vSphere and Docker, Install, updates and agents, Updates and agents,
Pulse Pro, AI and hosted, Monitoring and service health, Security, and Other
improvements. Omit unused or thin groups instead of filling every heading. A
narrow patch can keep the simpler section below instead. Do not repeat a change
between groups or add empty sections.

## What's improved

- **Short outcome** - Explain where users notice it and why it matters.
- **Short outcome** - Keep each item concrete and independently useful.
- **Short outcome** - Include fixes here as outcomes rather than repeating
  them in a second section.

Write for someone scanning the in-app What's new dialog in under a minute, not
reading a commit dump. Use a concise set of meaningful improvements that users
will actually notice or act on, and leave minor defensive hardening, routine
retry or focus tweaks, and narrow edge cases to the full PR changelog. Each
user-visible change belongs in the grouped changes exactly once. Prefer
observable behavior over component names, group related implementation work into
one user-recognizable theme, and use plain language. Prefer one crisp sentence
per bullet (around 100 to 150 characters) rather than packing several fixes into
a multi-sentence paragraph. Each complete bullet, including Markdown links, must be no
more than 260 characters. New release notes must not contain semicolons or em
dashes.

For an RC, cover only changes since the immediately preceding RC (or the
previous stable release for RC1). Do not repeat improvements already announced
in an earlier RC. For a stable GA release, cover the complete release train
since the previous stable release and boil the full commit range down to a few
themes that explain what users will experience differently. Do not concatenate
the RC notes or attempt to list every commit. The generator supplies the range,
read-only repository and GitHub tools, an unbounded factual investigation, an
independent draft that cannot see that investigation, and a final model that
receives both. The models decide what to inspect, what matters, and how to tell
the release story. The harness owns the release boundary, public format, safety
constraints, and optional per-pass traces.

The post-update dialog is designed to be scanned quickly: Breaking changes (when
present) lead at the top, followed by Highlights and optional screenshots so
users immediately see what changed. Compact Known issues, short change lists,
and brief Upgrade notes appear directly inline, while long Known issues, large
multi-section change lists, and multi-item Upgrade notes collapse into compact
expandable rows with item counts (plus an Expand all control for change groups)
so large releases stay readable.
The visual investigation also reads the Visual evidence sections and images
from merged PRs in this release range. It may reuse a public GitHub image
attachment or immutable committed screenshot when inspected, safe and still
accurate for the release, or capture
fresh before and after views using identical demo data. Selected PR images are
copied into versioned release PNG assets. A before view must match the actual
comparison tag. A new feature may use one current view. No images is a valid
outcome when screenshots do not explain anything useful.

Do not add a separate `Fixes` section. That shape encourages the same change to
be described twice as both an improvement and a fix.

## Known issues

Describe remaining user symptoms and an actionable workaround or next step.
Put critical risks and required safety actions here, including risks that apply
before an upgrade. Keep the list brief and non-repetitive by combining related
cautions for the same underlying issue into one concise bullet instead of
splitting one problem across several bullets. Omit this section when there are
none. Do not substitute internal verification status for information users can
act on.

## Before you upgrade

Include only compatibility, migration, signing, companion-app, known-risk, or
required operator information. Keep each item short and actionable. Omit this
section when users do not need to do or understand anything before upgrading.
This section appears at the bottom of the post-update dialog as Upgrade notes.
Put critical warnings in Breaking changes or Known issues.

The release pipeline appends the `Install` and `Roll back` sections from
governed version metadata. Do not add release qualification, validation,
workflow, gate, governance, or promotion-metadata sections to the customer
notes.
