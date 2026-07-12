import { describe, expect, it } from 'vitest';
import type { ClusterEndpoint } from '@/types/nodes';
import {
  buildClusterEndpointOverridesPayload,
  deriveNameFromHost,
} from '../nodeModalModel';

// Same fixture builder shape as the sibling nodeModalModel.test.ts suite so the
// private comparison/trim branches under test are driven through the single
// exported `buildClusterEndpointOverridesPayload` orchestrator.
const endpoint = (nodeName: string, ipOverride?: string): ClusterEndpoint => ({
  nodeId: `node/${nodeName}`,
  nodeName,
  host: `https://${nodeName}.local:8006`,
  ip: '10.0.0.1',
  ipOverride,
  online: true,
  lastSeen: '',
});

// ---- deriveNameFromHost -----------------------------------------------------
//
// Branches exercised below (in order of the function body):
//   * `host.trim()` then `if (!value) return ''` — empty + whitespace-only
//     inputs, plus the leading/trailing-trim path.
//   * ternary `value.includes('://') ? new URL(value) : new URL('https://' + v)`
//     — both the scheme-present (`https://...`, `file://...`) and scheme-absent
//     (`pve1.local`, `[::1]:8006`) arms.
//   * `url.hostname || value` — the truthy-hostname arm (normal hosts) AND the
//     falsy-hostname defensive fallback (a valid URL whose hostname is empty,
//     e.g. `file://`), which keeps the raw input.
//   * `catch` arm — inputs that fail WHATWG URL parsing (spaces in the host)
//     hit `value.replace(/^https?:\/\//, '')`.
//   * post-URL `.replace(/\/.*$/, '')` — strips a trailing path that survived
//     into the catch branch.
//   * post-URL `.replace(/^\[(.*)\]$/, '$1')` — unwraps the IPv6 brackets that
//     `URL.hostname` emits for an IPv6 literal.
//   * post-URL `.replace(/\s+/g, '-')` — collapses internal whitespace that
//     survived into the catch branch.

describe('deriveNameFromHost', () => {
  it('returns the empty string for an empty host (early return before URL parsing)', () => {
    expect(deriveNameFromHost('')).toBe('');
  });

  it('returns the empty string for a whitespace-only host (trim() -> empty -> early return)', () => {
    expect(deriveNameFromHost('   ')).toBe('');
  });

  it('trims surrounding whitespace before parsing (host with no scheme, success arm)', () => {
    expect(deriveNameFromHost('  pve1.local  ')).toBe('pve1.local');
  });

  it('parses a bare hostname via the no-scheme arm (new URL(`https://${value}`))', () => {
    expect(deriveNameFromHost('pve1.local')).toBe('pve1.local');
  });

  it('uses the scheme-present arm and drops port + path from url.hostname', () => {
    // `value.includes('://')` is true, so `new URL(value)` runs directly and
    // url.hostname is the truthy 'example.com'; the trailing /\/.*$/ and IPv6
    // regexes are no-ops on a plain hostname.
    expect(deriveNameFromHost('https://example.com:8006/path?q=1')).toBe('example.com');
  });

  it('unwraps IPv6 brackets emitted by URL.hostname for an IPv6 literal', () => {
    // No scheme -> `new URL('https://[::1]:8006')`; URL.hostname returns
    // '[::1]' (bracketed), which the `/^\[(.*)\]$/` regex reduces to '::1'.
    expect(deriveNameFromHost('[::1]:8006')).toBe('::1');
  });

  it('keeps the raw input when url.hostname is empty (falsy-hostname `|| value` fallback)', () => {
    // `file:///etc/hosts` parses (so no catch) but URL.hostname is '' for the
    // file scheme; the `|| value` fallback keeps 'file:///etc/hosts', then
    // /\/.*$/ strips from the first slash, leaving 'file:'.
    expect(deriveNameFromHost('file:///etc/hosts')).toBe('file:');
  });

  it('falls into the catch arm and collapses internal whitespace', () => {
    // 'foo bar' (no scheme) -> `new URL('https://foo bar')` throws; the catch
    // leaves 'foo bar' (no leading scheme to strip), and /\s+/g -> 'foo-bar'.
    expect(deriveNameFromHost('foo bar')).toBe('foo-bar');
  });

  it('strips a leading http(s):// scheme inside the catch arm', () => {
    // Scheme present -> `new URL('http://bad url')` throws; the catch regex
    // removes 'http://', leaving 'bad url', then whitespace collapse.
    expect(deriveNameFromHost('http://bad url')).toBe('bad-url');
  });

  it('strips a trailing path that survived into the catch arm', () => {
    // 'foo bar/baz' -> `new URL('https://foo bar/baz')` throws (space in
    // host); the catch keeps 'foo bar/baz', /\/.*$/ drops '/baz', then
    // whitespace collapse -> 'foo-bar' (not 'foo-bar/baz').
    expect(deriveNameFromHost('foo bar/baz')).toBe('foo-bar');
  });
});

// ---- buildClusterEndpointOverridesPayload ----------------------------------
//
// The sibling suite already covers the core happy/undefined paths; these tests
// target the comparison/trim nuances and the defensive `?? ''` fallback inside
// the `.map` that the happy-path inputs never reach:
//   * `(overrides[name] ?? '').trim()` — the OUTPUT trim of a non-empty form
//     value (siblings only asserted an untrimmed value or a whitespace-only
//     no-op).
//   * multi-member inclusion with an unchanged member in the middle (order
//     preservation + selective filtering).
//   * `value.trim() !== (endpoint.ipOverride ?? '')` where trim changes the
//     form value but it still equals the saved override -> treated as unchanged.
//   * the map's unreachable-in-practice `(overrides[name] ?? '')` defensive
//     fallback, driven via a Proxy that returns undefined on the second read.

describe('buildClusterEndpointOverridesPayload', () => {
  it('trims a non-empty form value when emitting the override', () => {
    const payload = buildClusterEndpointOverridesPayload([endpoint('pve1')], {
      pve1: '  10.0.0.11  ',
    });
    expect(payload).toStrictEqual([{ nodeName: 'pve1', ipOverride: '10.0.0.11' }]);
  });

  it('includes multiple changed members in order and skips an unchanged middle member', () => {
    const endpoints = [
      endpoint('pve1'),                       // no saved override -> '' !== '10.0.0.11' -> changed
      endpoint('pve2', '10.0.0.2'),           // '10.0.0.2' === '10.0.0.2'  -> unchanged (skipped)
      endpoint('pve3', '10.0.0.3'),           // '10.0.0.33' !== '10.0.0.3' -> changed
    ];
    const payload = buildClusterEndpointOverridesPayload(endpoints, {
      pve1: '10.0.0.11',
      pve2: '10.0.0.2',
      pve3: '10.0.0.33',
    });
    expect(payload).toStrictEqual([
      { nodeName: 'pve1', ipOverride: '10.0.0.11' },
      { nodeName: 'pve3', ipOverride: '10.0.0.33' },
    ]);
  });

  it('treats a whitespace-padded form value as unchanged when it trims to the saved override', () => {
    // value.trim() ('10.0.0.5') === (endpoint.ipOverride ?? '') ('10.0.0.5'),
    // so the member is filtered out and the result collapses to undefined.
    const payload = buildClusterEndpointOverridesPayload([endpoint('pve1', '10.0.0.5')], {
      pve1: '  10.0.0.5  ',
    });
    expect(payload).toBeUndefined();
  });

  it('hits the map defensive `(overrides[name] ?? "")` fallback via a second-read Proxy', () => {
    // The filter reads overrides.pve1 once (returns the defined value so the
    // member survives), then the map reads it again — the Proxy returns
    // undefined on that second access, forcing `(undefined ?? '').trim()` => ''.
    const target: Record<string, string> = { pve1: '10.0.0.11' };
    let accesses = 0;
    const flaky = new Proxy(target, {
      get(t, prop) {
        if (prop === 'pve1') {
          accesses += 1;
          return accesses === 2 ? undefined : t.pve1;
        }
        return Reflect.get(t, prop);
      },
    });
    const payload = buildClusterEndpointOverridesPayload([endpoint('pve1')], flaky);
    expect(payload).toStrictEqual([{ nodeName: 'pve1', ipOverride: '' }]);
    expect(accesses).toBe(2);
  });
});
