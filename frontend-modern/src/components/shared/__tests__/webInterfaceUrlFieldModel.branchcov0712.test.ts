import { describe, expect, it } from 'vitest';
import {
  getWebInterfaceTargetLabel,
  validateWebInterfaceCustomUrl,
} from '@/components/shared/webInterfaceUrlFieldModel';
import type { WebInterfaceUrlFieldProps } from '@/components/shared/webInterfaceUrlFieldModel';

// Branch-coverage suite for the two still-uncovered pure helpers in
// webInterfaceUrlFieldModel.ts. Every assertion is a concrete equality check
// against the real return shape/string so it pins the exact branch taken.

describe('webInterfaceUrlFieldModel.branchcov2', () => {
  describe('validateWebInterfaceCustomUrl', () => {
    it('returns null for an empty value (the `!value` early-return arm)', () => {
      expect(validateWebInterfaceCustomUrl('')).toBeNull();
    });

    it('returns the parse-failure message for a non-URL string (catch arm)', () => {
      expect(validateWebInterfaceCustomUrl('not-a-url')).toBe(
        'Enter a valid URL (for example: https://198.51.100.100:8080).',
      );
    });

    it('returns the parse-failure message for a whitespace-only value (truthy string that URL rejects)', () => {
      // ' ' is a truthy string so it bypasses the `!value` guard but still
      // throws inside `new URL(...)`, exercising the catch arm via a different
      // inbound state than a plain word.
      expect(validateWebInterfaceCustomUrl(' ')).toBe(
        'Enter a valid URL (for example: https://198.51.100.100:8080).',
      );
    });

    it('returns the parse-failure message for a bare scheme like "http://" (URL constructor throws)', () => {
      expect(validateWebInterfaceCustomUrl('http://')).toBe(
        'Enter a valid URL (for example: https://198.51.100.100:8080).',
      );
    });

    it('rejects an ftp:// URL via the protocol guard (neither http: nor https:)', () => {
      expect(validateWebInterfaceCustomUrl('ftp://example.com')).toBe(
        'URL must start with http:// or https://.',
      );
    });

    it('rejects a file:// URL via the protocol guard (different non-http arm of the &&)', () => {
      expect(validateWebInterfaceCustomUrl('file:///etc/hostname')).toBe(
        'URL must start with http:// or https://.',
      );
    });

    it('accepts a well-formed http:// URL with host + port (protocol guard pass, returns null)', () => {
      expect(validateWebInterfaceCustomUrl('http://198.51.100.100:8080')).toBeNull();
    });

    it('accepts a well-formed https:// URL (https: arm of the protocol comparison)', () => {
      expect(validateWebInterfaceCustomUrl('https://pve1.local:8006')).toBeNull();
    });

    it('accepts an uppercase HTTPS:// scheme because new URL normalizes the protocol', () => {
      // Confirms the protocol equality check works against the lowercased
      // protocol produced by the URL parser, not the raw input.
      expect(validateWebInterfaceCustomUrl('HTTPS://example.com')).toBeNull();
    });

    it('accepts an IPv6 http URL (hostname populated, no protocol/hostname error)', () => {
      expect(validateWebInterfaceCustomUrl('http://[2001:db8::1]/')).toBeNull();
    });

    // NOTE on the `if (!parsed.hostname)` branch (lines 37-39): the WHATWG URL
    // parser throws for every http:/https: input that would otherwise yield an
    // empty hostname (verified: 'http://', 'http:///', 'http://?q', 'http://[]',
    // 'https://', ... all throw), and any input that parses to an empty hostname
    // is always a non-http/https scheme (e.g. 'file:///x') which is caught by the
    // earlier protocol guard. That branch is therefore unreachable from the
    // public `(value: string) => ...` entry point under the real `URL` runtime,
    // so it is left uncovered here rather than papered over with a cast that
    // could not actually land inside the branch.
  });

  describe('getWebInterfaceTargetLabel', () => {
    it('returns the trimmed targetLabel when present, overriding the kind default', () => {
      // kind 'guest' would default to 'workload', so returning 'container-7'
      // proves the explicit label wins.
      expect(
        getWebInterfaceTargetLabel('guest', '  container-7  '),
      ).toBe('container-7');
    });

    it('returns the trimmed non-empty label even for agent kind (label still wins over default)', () => {
      expect(getWebInterfaceTargetLabel('agent', '\tnginx\t')).toBe('nginx');
    });

    it('falls back to "agent" when the label is undefined and kind is agent (ternary true arm)', () => {
      expect(getWebInterfaceTargetLabel('agent', undefined)).toBe('agent');
    });

    it('falls back to "workload" when the label is undefined and kind is guest (ternary false arm)', () => {
      expect(getWebInterfaceTargetLabel('guest', undefined)).toBe('workload');
    });

    it('falls back to "workload" when the label is undefined and kind is docker (ternary false arm)', () => {
      expect(getWebInterfaceTargetLabel('docker', undefined)).toBe('workload');
    });

    it('treats a whitespace-only label as empty and falls back to the agent default', () => {
      // Exercises normalizeWebInterfaceUrl('   ').trim() === '' then the ternary.
      expect(getWebInterfaceTargetLabel('agent', '   ')).toBe('agent');
    });

    it('treats an empty-string label as empty and falls back to the workload default for guest', () => {
      expect(getWebInterfaceTargetLabel('guest', '')).toBe('workload');
    });

    it('treats a null label (via cast) as empty using normalizeWebInterfaceUrl\'s `value || ""` arm', () => {
      // null is not a valid string; cast through unknown to satisfy strict
      // null checks while still exercising the real `value || ''` fallback
      // inside normalizeWebInterfaceUrl at runtime.
      const nullLabel = null as unknown as Parameters<
        typeof getWebInterfaceTargetLabel
      >[1];
      expect(getWebInterfaceTargetLabel('docker', nullLabel)).toBe('workload');
    });

    it('covers all three metadataKind values against the same empty-label input', () => {
      const kinds: WebInterfaceUrlFieldProps['metadataKind'][] = [
        'guest',
        'agent',
        'docker',
      ];
      const results = kinds.map((k) => getWebInterfaceTargetLabel(k, undefined));
      expect(results).toStrictEqual(['workload', 'agent', 'workload']);
    });
  });
});
