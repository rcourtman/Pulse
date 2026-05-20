import type { PlatformTableCellAlign } from './sharedPlatformPage';

/**
 * Canonical column kinds for platform tables.
 *
 * Every table that lives on a platform overview page (Proxmox / Docker /
 * Kubernetes / TrueNAS / vSphere top tables, the shared workloads bottom
 * table, services tables, deployments tables, and the like) should pick
 * a kind for each column from this enum. The kind drives header and cell
 * alignment via `PLATFORM_COLUMN_ALIGN_BY_KIND` so the same column type
 * lines up the same way everywhere.
 *
 * Pick the kind that matches what the cell *renders*:
 *
 *   - 'name'           Primary identifier (bold, usually the first column).
 *                       Host, Node, System, Service, Deployment.
 *
 *   - 'text'           Readable string content (single or multi-word).
 *                       Version, Datacenter, Cluster, vCenter, Runtime,
 *                       Roles, Kubelet, Image, Mode, Power, Swarm role,
 *                       Ports, Capacity-as-text.
 *
 *   - 'metric-bar'     A horizontal usage bar that fills the cell width,
 *                       with the percentage / size label riding at the
 *                       bar's fill edge.
 *                       CPU, Memory, Disk, Storage.
 *
 *   - 'numeric-value'  Scalar text with units, or pure digits, where
 *                       column-scanning the values matters.
 *                       Uptime, Temp, integer counts (VMs, CTs,
 *                       Containers, Pods, Pools, Datasets, Datastores,
 *                       Disks, Apps, Desired, Updated, Ready, Available),
 *                       Net I/O, Disk I/O.
 *
 *   - 'badge'          A pill or icon-only cell with no scannable value.
 *                       Backup, Tags, Update.
 *
 * Rationale for the mapping:
 *
 *   - 'metric-bar' is centered because the bar fills the cell and the
 *     percentage label slides along the fill edge depending on the
 *     value; the header sits over the column's visual center, where
 *     the eye naturally goes, instead of floating at a left or right
 *     edge that doesn't correspond to anything in the cell.
 *
 *   - 'numeric-value' is right-aligned so units ("d", "h", "GB",
 *     "MB/s") and digit endings line up vertically for fast column
 *     scanning (which host is highest? which has shortest uptime?).
 *
 *   - 'name' and 'text' are left-aligned for reading.
 *
 *   - 'badge' is centered because the rendered content is a square-ish
 *     pill or icon, not a scannable value.
 *
 * This file is the single source of truth. To change the canonical, edit
 * the map below; every table that consumes these helpers picks up the
 * change automatically. `scripts/canonical-platform-audit.mjs` enforces
 * that platform tables use canonical alignments for well-known column
 * labels, so drift breaks pre-push.
 */
export type PlatformTableColumnKind =
  | 'name'
  | 'text'
  | 'metric-bar'
  | 'numeric-value'
  | 'badge';

export const PLATFORM_COLUMN_ALIGN_BY_KIND: Record<
  PlatformTableColumnKind,
  PlatformTableCellAlign
> = {
  name: 'left',
  text: 'left',
  'metric-bar': 'center',
  'numeric-value': 'right',
  badge: 'center',
};

export const getPlatformColumnAlign = (
  kind: PlatformTableColumnKind,
): PlatformTableCellAlign => PLATFORM_COLUMN_ALIGN_BY_KIND[kind];
