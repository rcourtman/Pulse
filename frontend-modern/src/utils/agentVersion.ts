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

// Compare pre-release identifier lists per semver precedence rules.
function comparePrerelease(a: string[], b: string[]): number {
  if (a.length === 0 && b.length === 0) return 0;
  if (a.length === 0) return 1;
  if (b.length === 0) return -1;

  const len = Math.max(a.length, b.length);
  for (let i = 0; i < len; i++) {
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

// Normalise a version for display: strip build metadata, ensure a leading `v`.
export function formatAgentVersionDisplay(raw?: string | null): string {
  const parsed = parseAgentVersion(raw);
  if (!parsed) return '';
  const core = `${parsed.major}.${parsed.minor}.${parsed.patch}`;
  const pre = parsed.prerelease.length ? `-${parsed.prerelease.join('.')}` : '';
  return `v${core}${pre}`;
}
