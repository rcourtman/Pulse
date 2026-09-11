# Disk mount visibility (#2051)

The Disks card previously clipped its mount list to 140px. Its thin nested
scrollbar was the only indication of additional mounts. The repair keeps all
mounts in the outer scrolling flow; aggregate usage and individual mount data
are unchanged. Large lists make the card taller rather than introducing another
scroll target. This deliberately avoids a global scrollbar-style change.

## Reproduce

Install locked root and frontend-modern npm dependencies and pinned Playwright
Firefox/Chromium browsers, then from the repository root:

```sh
pulse-heavy-run -- node tests/qualification/disk-mounts/check.mjs
```

The isolated Vite fixture imports the production card and stylesheet. It uses
synthetic mount data, no backend, credentials or customer installation. It checks
2 and 24 mounts in light/dark themes, list clipping and keyboard End/Tab access
to the final mount and following control. Vite is shut down after the run.
The qualification HTML is not an application entry or production build input.

## Evidence — 11 September 2026

Baseline 72809bc1cf71, Firefox 142.0.1: short list 54/54px; long list
clientHeight=140, scrollHeight=736, overflow=auto, maxHeight=140px. The no-clipping
assertion fails. After repair all eight Firefox 142.0.1 / Chromium 141.0.7390.37
cases pass: short 54/54px, long 736/736px, overflow=visible, maxHeight=none.
Keyboard checks pass in each case. Component coverage additionally preserves
aggregate usage, all mounts and the empty state.

This demonstrates the nested clipping and its removal with production CSS. It
does not establish why Firefox 140.15 ESR on the reporter's Debian desktop hides
its native scrollbar, nor reproduce their full drawer/estate. Reporter retest
and release availability remain distinct from this source/browser proof.

The reporter's subsequent before/after-scroll crops in comment
https://github.com/rcourtman/Pulse/issues/2051#issuecomment-5632905322 were also
inspected: no evident pre-scroll thumb, then a narrow pale mark beside the disk
bars after scrolling. Removing the nested overflow eliminates that card's native
track/thumb entirely, rather than claiming to repair Firefox painting. The
fixture proves there is no hidden card overflow before keyboard scrolling;
outer-page navigation still reaches every mount. Exact desktop painting remains
unreproduced.

Completion verification adds 600x500 and 1200x500 viewports: all 16 cases pass.
Full-page narrow Firefox dark and desktop Chromium light images were inspected;
all mounts and the focused following control remain visible without nested clipping.
