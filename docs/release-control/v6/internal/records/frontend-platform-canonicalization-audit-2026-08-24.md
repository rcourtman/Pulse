# Frontend platform canonicalization audit — 2026-08-24

## Executive assessment

Pulse already rendered as a substantially coherent product before this slice:
the platform section navigation, framed page tables, dark column-header band,
compact single-line summary rows, semantic status treatment, responsive column
priority, drawer navigation, and History placement were genuinely shared. The
remaining drift risk was concentrated at the seams. Several nested drawer and
inline-detail tables independently rebuilt the same table chrome; four legacy
Proxmox inventory tables retained page-local row-disclosure wiring; and the
shared operator-state editor rebuilt compact native controls instead of using
the form primitives.

No P0 or P1 operator defect was found. The baseline felt like one product, but
the duplicated seams made future density, header, phone-disclosure, and form
changes likely to drift. This slice removes those seams and turns the visual
agreement into shared structure plus static enforcement.

Rendered evidence is stored in the task's `platform-audit` visual-artifact
directory. It contains a desktop and phone capture for every route in the
coverage matrix, representative open drawers and History tabs, post-change
captures prefixed with `post-`, and independent Computer Use evidence.

## Canonical design inventory

| Pattern | Canonical contract | Shared owner |
| --- | --- | --- |
| Platform section navigation | One scrollable section-tab treatment; active state and narrow overflow behavior are shared while section labels remain platform-specific. | `frontend-modern/src/features/platformPage/sharedPlatformPage.tsx` (`PlatformSectionTabs`) and `frontend-modern/src/components/shared/Subtabs.tsx` |
| Framed page table | One card border, title/header band, dark column-header band, overflow boundary, loading/empty/error presentation, and compact row density. | `sharedPlatformPage.tsx` (`PlatformTableShell`) over `TableCard.tsx`, `TableCardHeader.tsx`, and `Table.tsx` |
| Cardless nested table | Drawers and inline details reuse the page table's table/header/body structure without inventing a second card. Column content remains object-specific. | `sharedPlatformPage.tsx` (`PlatformDetailTable`, `PlatformDetailTableHeader`, `PlatformDetailTableBody`) |
| Summary row interaction | The whole row activates detail by pointer and Enter/Space, exposes focus, `aria-expanded`, and `aria-controls`, and ignores embedded interactive controls. | `frontend-modern/src/features/platformPage/PlatformResourceDetailTableRow.tsx` (`createPlatformResourceDetailState`, `getPlatformResourceDetailRowInteractionProps`) |
| Disclosure affordance | Desktop may show the shared disclosure button; phone removes that visual chevron because the row is the touch target. The accessible label remains available. | `PlatformResourceDetailTableRow.tsx` (`PlatformResourceDetailToggleButton`) and `SummaryRowActionButton.tsx` |
| Inline detail placement | Expanded content follows its owning row as a full-width table row and preserves table semantics. | `PlatformResourceDetailTableRow.tsx` and `InlineDetailTableRow.tsx` |
| Drawer hierarchy | Attention/problems and actionable context precede inventory detail; Overview does not repeat low-value row data; Manage owns operator overrides; discovery metadata appears only when enabled. | Platform drawers over the shared resource drawer/detail primitives and `ResourceOperatorStateSection.tsx` |
| History | History is a drawer tab, not duplicated on Overview. Range controls live with the chart and share the history component contract. | Shared drawer tabs and `HistoryChart` |
| Compact operator forms | Low-frequency drawer controls use an explicit compact form density while preserving labels, help relationships, errors, and touch behavior. | `frontend-modern/src/components/shared/Form.ts`, `FormSelect.tsx`, and `FormTextarea.tsx` |
| Loading/empty/error | Page tables use the same presentation and copy slots; domain-specific nouns may vary, but the placement, spacing, iconography, and retry boundary do not. | `sharedPlatformPage.tsx` (`PlatformTableLoading`, `PlatformTableEmpty`, `PlatformTableError`) |
| Status and metrics | Semantic meaning selects the status color; bars share the metric primitive. Different metrics are intentional content, not design variants. | Shared status and metric primitives consumed by platform pages |
| Responsive rows | Rows remain compact and single-line, with truncation and column priority. Tables own overflow; the document does not horizontally scroll. | `Table.tsx` and the exported platform table class constants in `sharedPlatformPage.tsx` |
| Windowed page scrolling | Large tables bound mounted DOM while preserving one browser-native vertical page scroller. Wheel input may prewarm the keyed runway; touch must update it only after the passive native scroll event. | `components/shared/windowedPageScroll.ts`, consumed by `usePlatformWindowedItems.ts`, `useUnifiedResourceTableViewportSync.ts`, `useStoragePoolsTableWindowing.ts`, and `useWorkloadViewportSync.ts` |

## Rendered route coverage

`D` means 1280×800 inspected and captured. `P` means 390×844 inspected and
captured. All listed routes passed both unless a state note follows.

| Platform | Route / meaningful section | D | P | Additional states exercised |
| --- | --- | :---: | :---: | --- |
| Proxmox | `/proxmox/overview` | ✓ | ✓ | node, guest, and storage drawers; node Overview and History |
| Proxmox | `/proxmox/storage` | ✓ | ✓ | physical disks, grouped/flat behavior, expanded storage rows |
| Proxmox | `/proxmox/replication` | ✓ | ✓ | grouped inventory and row detail |
| Proxmox | `/proxmox/backups/date` | ✓ | ✓ | backup-server row pointer plus Enter/Space expansion |
| Proxmox | `/proxmox/backups/coverage` | ✓ | ✓ | attention state, expanded restore evidence, nested table |
| Proxmox | `/proxmox/ceph` | ✓ | ✓ | expanded cluster detail and cluster drawer |
| Proxmox | `/proxmox/mail` | ✓ | ✓ | expanded gateway detail and gateway drawer |
| Docker | `/docker/overview` | ✓ | ✓ | host and container drawers; host History |
| Docker | `/docker/images` | ✓ | ✓ | image inventory and drawer |
| Docker | `/docker/storage` | ✓ | ✓ | volume/storage inventory and drawer |
| Docker | `/docker/networks` | ✓ | ✓ | network inventory and drawer |
| Docker | `/docker/swarm` | ✓ | ✓ | services, configs, secrets, nested service table |
| Kubernetes | `/kubernetes/overview` | ✓ | ✓ | cluster drawer and alert context |
| Kubernetes | `/kubernetes/nodes` | ✓ | ✓ | node drawer |
| Kubernetes | `/kubernetes/workloads` | ✓ | ✓ | pods, deployments/controllers, drawers |
| Kubernetes | `/kubernetes/services` | ✓ | ✓ | services/networking and drawers |
| Kubernetes | `/kubernetes/storage` | ✓ | ✓ | storage inventory and drawers |
| Kubernetes | `/kubernetes/configuration` | ✓ | ✓ | config, policy, autoscaling, namespaces/deployments detail |
| Kubernetes | `/kubernetes/events` | ✓ | ✓ | events and attention states |
| TrueNAS | `/truenas/overview` | ✓ | ✓ | system drawer and alerts |
| TrueNAS | `/truenas/storage` | ✓ | ✓ | topology, pools/datasets, drawers |
| TrueNAS | `/truenas/services` | ✓ | ✓ | service state and drawer |
| TrueNAS | `/truenas/apps` | ✓ | ✓ | apps and drawer |
| TrueNAS | `/truenas/vms` | ✓ | ✓ | virtual machines and drawer |
| TrueNAS | `/truenas/shares` | ✓ | ✓ | shares and drawer |
| TrueNAS | `/truenas/protection` | ✓ | ✓ | protection and alert/attention state |
| VMware | `/vmware/overview` | ✓ | ✓ | host drawer and History |
| VMware | `/vmware/storage` | ✓ | ✓ | datastore inventory and drawer |
| VMware | `/vmware/networks` | ✓ | ✓ | network inventory and drawer |
| VMware | `/vmware/health` | ✓ | ✓ | alert/attention state |
| VMware | `/vmware/activity` | ✓ | ✓ | activity/history presentation |
| Standalone | `/machines` | ✓ | ✓ | machine drawer, stale/disconnected examples |
| Standalone | `/availability` | ✓ | ✓ | availability-check drawer; intentionally no History |

Loading was independently observed in the native browser while the live local
frontend connected to the backend. Empty and error presentation were traced to
and protected by the shared state components and their focused tests; the mock
estate did not expose every empty/error variant on every route, so those states
were not artificially inferred from healthy pixels.

## Prioritized findings and disposition

### P2 — nested tables only looked shared

- **Rendered evidence:** Docker Swarm, Kubernetes configuration drawers,
  Proxmox Ceph, Proxmox Mail Gateway, and Proxmox backup coverage matched the
  page-table header and density in desktop and phone captures.
- **Baseline source consumers:** `SwarmServicesDrawer.tsx`,
  `K8sDeploymentsDrawer.tsx`, `K8sNamespacesDrawer.tsx`,
  `ProxmoxCephClusterDrawer.tsx`, `ProxmoxMailGatewayDrawer.tsx`, and
  `ProxmoxCoverageTable.tsx` independently composed raw table/header/body
  structure and copied canonical class constants.
- **Operator impact:** operators would notice only after the next shared header,
  density, border, or responsive change landed unevenly; the defect was
  architectural drift risk rather than current pixel breakage.
- **Disposition:** migrated every consumer to `PlatformDetailTable`,
  `PlatformDetailTableHeader`, and `PlatformDetailTableBody`. Registry auditing
  now requires those owners and consumers.

### P2 — legacy Proxmox disclosure behavior was page-local

- **Routes/viewports:** `/proxmox/backups/date`,
  `/proxmox/backups/coverage`, `/proxmox/ceph`, and `/proxmox/mail` at both
  viewports. Post-change phone evidence includes
  `post-proxmox-mail-expanded-390x844.png` and
  `post-proxmox-backup-coverage-expanded-390x844.png`.
- **Baseline source consumers:** `ProxmoxBackupServersTable.tsx`,
  `ProxmoxCoverageTable.tsx`, `ProxmoxCephTable.tsx`, and
  `ProxmoxMailGatewayTable.tsx` each carried local click/keyboard/aria state.
- **Operator impact:** a phone row or keyboard path could diverge while newer
  platforms changed centrally; visible duplicate chevrons were a recurring
  regression risk.
- **Disposition:** all four now consume the shared interaction owner. Browser
  verification confirmed pointer, Enter, and Space activation, focusability,
  `aria-expanded`, `aria-controls`, and absence of a visible phone disclosure
  button.

### P2 — compact operator forms bypassed the form system

- **Rendered surface:** the shared resource drawer Manage tab across platforms.
- **Baseline source owner:** `ResourceOperatorStateSection.tsx` used native
  select and textarea shells with page-local classes.
- **Operator impact:** label/help/error behavior and compact density could drift
  from other form controls in every resource drawer at once.
- **Disposition:** added documented `compact` density variants to `FormSelect`
  and `FormTextarea`, then migrated the shared Manage section. The controls stay
  out of Overview because overrides are low-frequency operator actions.

### P3 — drift detection did not encode these contracts

- **Baseline:** matching class strings could pass review without proving shared
  ownership, while a few guard patterns produced test-only false positives.
- **Disposition:** the shared-template audit now supports required alternative
  ownership patterns, requires the nested-table and row-interaction contracts,
  excludes tests from runtime chevron/native-control rules, and retains the
  canonical-platform lint gate.

### P3 — remaining shared-template findings resolved in follow-up

The follow-up complete shared-template audit now passes without exceptions.
Two reported consumers were inaccurate ownership declarations:
`ApprovalBanner` has no loading state and `UpdatesSettingsPanel` has no native
select, so those paths were removed from the respective required-consumer
inventories. The two real findings were migrated: `RelayPairingSection` now
composes `ExternalTextLink`, while the Patrol suppression reason and duration
compose `FormTextarea` and `FormSelect`. A Patrol textarea guard now protects
that additional form-control boundary and identified the same latent fork in
`PatrolObjectivesPanel`; both objective textareas now compose `FormTextarea` as
well.

## Decisions for previously ambiguous contracts

1. **Shared structure, object-specific content.** Platform columns, metrics,
   health explanations, and technical details remain domain-specific. Table
   chrome, density, state presentation, interaction, and responsiveness do not.
2. **Whole row is the phone affordance.** The disclosure button remains in the
   accessibility tree and may be visually present on desktop, but phone layouts
   do not show a redundant chevron.
3. **History is a drawer destination.** History does not compete with current
   operator context on Overview. Availability checks may omit History when the
   object has no meaningful historical series.
4. **Operator overrides belong in Manage.** Discovery metadata is conditional;
   operator override controls are not promoted into Overview merely to make
   them more visible.
5. **Cardless is a supported table variant, not a fork.** Nested tables should
   not receive a decorative second card, but they must consume the shared table
   anatomy.
6. **State copy may name the object.** Loading/empty/error shells are canonical;
   concise domain nouns are supported content rather than inconsistency.

## Remediation sequence applied

1. Established rendered baseline and source ownership across all routes.
2. Added the shared cardless table and whole-row interaction owners.
3. Migrated all known repeated nested-table and legacy Proxmox consumers.
4. Added compact form variants and migrated the cross-platform Manage surface.
5. Added focused interaction/structure/form tests and static registry rules.
6. Updated the frontend-primitives contract.
7. Re-ran representative desktop/phone pixels, expanded rows, nested evidence,
   alert/attention states, keyboard activation, and independent Computer Use.

## Patterns already correct and intentionally not churned

- Platform section tabs and narrow horizontal overflow.
- `PlatformTableShell` page-level card/table anatomy.
- Compact single-line summary-row density, truncation, and column priority.
- Shared status colors and metric bars; different provider metrics remain.
- Docker, Kubernetes, TrueNAS, VMware, Machines, and availability whole-row
  disclosure consumers already using the shared detail-state contract.
- Drawer tabs, Overview/History placement, range controls, alert-first
  hierarchy, and conditional discovery metadata.
- Loading/empty/error component ownership and healthy/stale/disconnected status
  semantics.
- Availability check drawers intentionally omitting History when no useful
  historical series exists.

## Verification closure

- The eight focused stale-contract files passed **38 tests** after their
  assertions were migrated to the canonical drawer, disclosure, sorting, and
  operator-information contracts.
- The four files that timed out only under the initial unbounded worker load
  passed **115 tests** in isolated reruns, confirming contention rather than a
  product or contract failure.
- The bounded complete frontend suite passed **1,149 test files** with
  **20,676 passing tests** and **3 intentionally skipped Windows-specific
  tests**. One test file is intentionally skipped; there were no failed files,
  unhandled worker errors, or RPC timeouts.
- `npm run lint` passed, including the theme, copy, and canonical-platform
  ownership audits. `npm run type-check` and the production `npm run build`
  also passed.
- The production build retained only the existing Vite advisory for modules
  that are both statically and dynamically imported; it did not affect build
  correctness or the canonical frontend contract.

## Post-audit correction: phone scroll ownership

The initial audit missed a gesture-level inconsistency that was not visible in
static phone captures, and the first correction addressed only one of its two
causes. On `/docker` at 390×844, the shared table shells had 2px of incidental
vertical overflow. Because `overflow-x: auto` computes the other axis to `auto`
unless it is specified, a vertical gesture beginning over either table could
first scroll that nested shell instead of the page. The canonical
`.table-scroll-shell` owner in `frontend-modern/src/index.css` therefore sets
`overflow-y: hidden`: tables own horizontal overflow while the page owns
vertical gestures.

Actual touch-device testing then exposed a second interception on large data
sets. The four virtualized table controllers attached a non-passive
`touchmove` listener to the app scroller and replaced the mounted keyed-row
runway before native page movement. Browser scroll anchoring could consume the
gesture: operators saw different table items while the page appeared fixed.
Proxmox Overview currently follows a different workload rendering path, which
explains why the same phone gesture did not show the defect there.

The canonical windowing contract now leaves touch entirely on the
compositor-native path. `components/shared/windowedPageScroll.ts` is the sole
listener-lifecycle owner: it registers passive native `scroll`, desktop wheel
prewarming, and resize, and exposes no touch registration. The four windowing
controllers consume that owner while retaining only their object-specific row
projection. An independent Computer Use swipe over Docker's windowed container
table at 390×844 moved the page from the host summary to the footer while
preserving the table as part of that single page scroll. Focused runtime and
source-contract tests protect the shared owner and every consumer.

Customer verification on Chrome Android then identified a distinct remaining
effect: the container table itself elastically stretched while the page stayed
fixed. The prior Computer Use proof was invalid for this path because the page
had initialized at desktop width before device emulation was enabled, leaving
the 12-row demo below the 140-row desktop window rather than exercising phone
windowing. Chrome's newer non-root overscroll effect also made the horizontal
table wrapper visibly stretch at its vertical boundary even after touch
listeners were removed.

The final canonical rule is structural. `Table` exposes the explicit
`phoneVerticalScrollOwner` variant, and every `PlatformDetailTable` selects the
page owner. Below 40rem the responsive platform table already fits its
prioritized columns, so `.table-scroll-shell-phone-page` uses `overflow: clip`
and is not a nested scrollport on either axis. Wider horizontal tables retain
their rail, with `overscroll-behavior-y: chain` where Chromium supports it so
vertical propagation has no table-local stretch effect. A fresh 390×844 Chrome
reload performed after emulation was enabled and with a forced six-item phone
window measured both Docker wrappers at `overflow-x: clip`, `overflow-y: clip`,
`scrollWidth === clientWidth === 372`, and vertical overscroll `chain`. The
production 36-item phone budget was restored after that diagnostic run.
Updated evidence is stored as
`post-docker-chrome-android-page-scroll-owner-390x844.jpg` alongside the audit
captures.

## Remaining exceptions

There are no known static-audit or platform-page exceptions to the canonical
table, summary-row, external-link, or labelled native-control contracts after
the follow-up. Route-specific cells, metrics, drawer sections, and technical
details are intentional object content inside the shared contract, not forks.
