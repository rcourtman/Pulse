import type { Resource } from '@/types/resource';

// Agent staleness helpers for platform pages.
//
// Platform pages gate their detail tabs (Docker images/networks/storage/swarm,
// and the equivalent on other runtimes) purely on whether the matching
// inventory is present in the resource snapshot. An agent that predates a given
// inventory feature simply never reports it, so those tabs silently disappear
// with no way for the user to tell "this host genuinely has none" from "this
// host's agent is too old to report it". These helpers let a page detect the
// second case by comparing each host's reported agent version against the
// server that is rendering the page.

export type ParsedAgentVersion = {
  major: number;
  minor: number;
  patch: number;
  // Pre-release identifiers, e.g. ['rc', '5']. Empty means a stable release,
  // which always outranks any pre-release of the same core version.
  prerelease: string[];
};

// Parse a semver-shaped version string. Tolerates a leading `v`/`V` and strips
// build metadata (`+git.151.g...`) which carries no precedence. Returns null
// for anything that is not at least `major.minor.patch`.
export function parseAgentVersion(raw?: string | null): ParsedAgentVersion | null {
  if (!raw) return null;
  let value = raw.trim();
  if (!value) return null;

  const buildIdx = value.indexOf('+');
  if (buildIdx >= 0) value = value.slice(0, buildIdx);
  value = value.replace(/^[vV]/, '');

  const dashIdx = value.indexOf('-');
  const core = dashIdx >= 0 ? value.slice(0, dashIdx) : value;
  const pre = dashIdx >= 0 ? value.slice(dashIdx + 1) : '';

  const parts = core.split('.');
  if (parts.length < 3) return null;
  const nums = parts.slice(0, 3).map((part) => Number.parseInt(part, 10));
  if (nums.some((n) => Number.isNaN(n))) return null;

  return {
    major: nums[0],
    minor: nums[1],
    patch: nums[2],
    prerelease: pre ? pre.split('.') : [],
  };
}

function isNumericIdentifier(value: string): boolean {
  return /^\d+$/.test(value);
}

// Compare pre-release identifier lists per semver §11.4.
function comparePrerelease(a: string[], b: string[]): number {
  if (a.length === 0 && b.length === 0) return 0;
  // A stable release (no pre-release) outranks a pre-release.
  if (a.length === 0) return 1;
  if (b.length === 0) return -1;

  const len = Math.max(a.length, b.length);
  for (let i = 0; i < len; i++) {
    // A larger set of fields outranks a smaller one when all preceding equal.
    if (i >= a.length) return -1;
    if (i >= b.length) return 1;

    const ai = a[i];
    const bi = b[i];
    const aNum = isNumericIdentifier(ai);
    const bNum = isNumericIdentifier(bi);

    if (aNum && bNum) {
      const diff = Number.parseInt(ai, 10) - Number.parseInt(bi, 10);
      if (diff !== 0) return diff < 0 ? -1 : 1;
    } else if (aNum) {
      // Numeric identifiers always rank lower than non-numeric ones.
      return -1;
    } else if (bNum) {
      return 1;
    } else if (ai !== bi) {
      return ai < bi ? -1 : 1;
    }
  }
  return 0;
}

// Returns -1 / 0 / 1 when both versions parse, or null when either does not.
export function compareAgentVersions(a?: string | null, b?: string | null): number | null {
  const pa = parseAgentVersion(a);
  const pb = parseAgentVersion(b);
  if (!pa || !pb) return null;
  if (pa.major !== pb.major) return pa.major < pb.major ? -1 : 1;
  if (pa.minor !== pb.minor) return pa.minor < pb.minor ? -1 : 1;
  if (pa.patch !== pb.patch) return pa.patch < pb.patch ? -1 : 1;
  return comparePrerelease(pa.prerelease, pb.prerelease);
}

// The agent version reported for a host. Docker hosts carry it on the docker
// meta; plain host agents carry it on the agent meta.
export function hostAgentVersion(host: Resource): string | undefined {
  return host.docker?.agentVersion?.trim() || host.agent?.agentVersion?.trim() || undefined;
}

// Normalise a version for display: strip build metadata, ensure a leading `v`.
export function formatAgentVersionDisplay(raw?: string | null): string {
  const parsed = parseAgentVersion(raw);
  if (!parsed) return '';
  const core = `${parsed.major}.${parsed.minor}.${parsed.patch}`;
  const pre = parsed.prerelease.length ? `-${parsed.prerelease.join('.')}` : '';
  return `v${core}${pre}`;
}

export type OutdatedAgentHost = { name: string; version: string };

// Hosts whose agent version is strictly behind the server version. Hosts with
// no reported version (or an unparseable one) are skipped rather than flagged,
// so we never raise a false alarm we cannot substantiate. Returns an empty list
// when the server version itself is unknown or unparseable (e.g. a build that
// does not report one), so a page never nags without a basis for comparison.
export function collectOutdatedAgentHosts(
  hosts: Resource[],
  serverVersion?: string | null,
): OutdatedAgentHost[] {
  if (!parseAgentVersion(serverVersion)) return [];

  const outdated: OutdatedAgentHost[] = [];
  for (const host of hosts) {
    const version = hostAgentVersion(host);
    if (!version) continue;
    const cmp = compareAgentVersions(version, serverVersion);
    if (cmp !== null && cmp < 0) {
      outdated.push({
        name: host.name?.trim() || host.id || 'host',
        version: formatAgentVersionDisplay(version),
      });
    }
  }
  return outdated;
}
