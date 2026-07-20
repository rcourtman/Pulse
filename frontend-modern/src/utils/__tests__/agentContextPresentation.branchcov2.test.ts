import { describe, expect, it } from 'vitest';
import type {
  AgentResourceContext,
  AgentResourceContextFact,
  AgentResourceContextRedaction,
  AgentResourceContextSection,
} from '@/api/agentContext';
import { formatAgentResourceContextForClipboard } from '@/utils/agentContextPresentation';

// `formatOptionalTime`, `formatProvenance`, `formatSectionHeader`, `formatFact`
// and `formatRedaction` are all module-private (not exported). Their branches
// are exercised transitively through `formatAgentResourceContextForClipboard`,
// which is the only public entry point — same approach the sibling
// `actionAuditPresentation.branchcov2` test takes for its private helper.

// Minimal valid context factory. Each test overrides only the fields it needs
// to drive a specific branch, keeping the intent of the assertion legible.
const baseContext = (overrides: Partial<AgentResourceContext> = {}): AgentResourceContext => ({
  canonicalId: 'host:node-1',
  resourceType: 'host',
  resourceName: 'node-1',
  activeFindings: [],
  pendingApprovals: [],
  recentActions: [],
  generatedAt: '2026-05-06T14:00:00Z',
  contextSections: [],
  ...overrides,
});

const baseSection = (
  overrides: Partial<AgentResourceContextSection> = {},
): AgentResourceContextSection => ({
  id: 'runtime',
  title: 'Runtime',
  source: '',
  trustTier: '',
  generatedAt: '2026-05-06T14:00:00Z',
  facts: [],
  ...overrides,
});

describe('agentContextPresentation — branch coverage (branchcov2)', () => {
  describe('formatAgentResourceContextForClipboard — header branches', () => {
    it('falls back to canonicalId when resourceName is empty (|| right operand)', () => {
      const text = formatAgentResourceContextForClipboard(baseContext({ resourceName: '' }));
      // resourceName '' is falsy -> `context.resourceName || context.canonicalId`
      // resolves to the canonical id.
      expect(text.startsWith('# Pulse resource context: host:node-1\n')).toBe(true);
      // And the resourceName must NOT appear as the header (it was empty).
      expect(text.startsWith('# Pulse resource context: \n')).toBe(false);
    });

    it('prefers resourceName when it is a non-empty string (|| left operand)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({ resourceName: 'pretty-name' }),
      );
      expect(text.startsWith('# Pulse resource context: pretty-name\n')).toBe(true);
    });

    it('omits the Technology line when context.technology is absent (if-false arm)', () => {
      const text = formatAgentResourceContextForClipboard(baseContext());
      expect(text).not.toContain('Technology:');
      // Sanity: the line that always follows the resource type is present.
      expect(text).toContain('Resource type: host');
      expect(text).toContain('Active findings: 0');
    });

    it('emits the Technology line when context.technology is set (if-true arm)', () => {
      const text = formatAgentResourceContextForClipboard(baseContext({ technology: 'docker' }));
      expect(text).toContain('Technology: docker');
    });
  });

  describe('formatAgentResourceContextForClipboard — full-output shape', () => {
    it('renders the exact minimal document with no sections and no technology', () => {
      // Pins the entire string — header fallback, formatOptionalTime('') on
      // generatedAt, the unconditional counts block, the trailing trim + '\n'.
      const text = formatAgentResourceContextForClipboard({
        canonicalId: 'host:node-1',
        resourceType: 'host',
        resourceName: '',
        activeFindings: [],
        pendingApprovals: [],
        recentActions: [],
        generatedAt: '',
        contextSections: [],
      });

      expect(text).toBe(
        [
          '# Pulse resource context: host:node-1',
          '',
          'Generated: ',
          'Canonical ID: host:node-1',
          'Resource type: host',
          'Active findings: 0',
          'Pending approvals: 0',
          'Recent actions: 0',
          '',
          'Context facts below are bounded, read-only Pulse context. Redacted values were withheld by policy.',
          '',
        ]
          .join('\n')
          .trim() + '\n',
      );
    });

    it('reflects non-zero counts verbatim from the three count arrays', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          activeFindings: [
            {
              id: 'f1',
              title: 't',
              severity: 'low',
              regressionCount: 0,
            },
          ],
          pendingApprovals: [
            { id: 'a1', riskLevel: 'low', requestedAt: 'x', expiresAt: 'y' },
            { id: 'a2', riskLevel: 'low', requestedAt: 'x', expiresAt: 'y' },
          ],
          recentActions: [
            {
              id: 'r1',
              capabilityName: 'c',
              state: 'done',
              success: true,
              createdAt: 'x',
              updatedAt: 'y',
            },
          ],
        }),
      );
      expect(text).toContain('Active findings: 1');
      expect(text).toContain('Pending approvals: 2');
      expect(text).toContain('Recent actions: 1');
    });
  });

  describe('formatOptionalTime (transitive via generatedAt / observedAt)', () => {
    it('returns ISO string for a valid timestamp (happy arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({ generatedAt: '2026-05-06T14:00:00Z' }),
      );
      // new Date(...).toISOString() normalises to the .000Z form.
      expect(text).toContain('Generated: 2026-05-06T14:00:00.000Z');
    });

    it('returns the raw value when it cannot be parsed as a date (NaN arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({ generatedAt: 'not-a-date' }),
      );
      expect(text).toContain('Generated: not-a-date');
      // and it must NOT have been coerced into an ISO string.
      expect(text).not.toContain('Generated: Invalid');
    });

    it('returns empty string for an empty timestamp (falsy arm)', () => {
      const text = formatAgentResourceContextForClipboard(baseContext({ generatedAt: '' }));
      expect(text).toContain('Generated: \n');
    });

    it('returns empty string for an undefined timestamp (falsy arm, via cast)', () => {
      // `generatedAt` is typed `string`; deliberately pass `undefined` to hit
      // the same `!value` short-circuit that a missing backend field would.
      const text = formatAgentResourceContextForClipboard(
        baseContext({ generatedAt: undefined as unknown as string }),
      );
      expect(text).toContain('Generated: \n');
    });

    it('also normalises valid observedAt inside section headers and fact provenance', () => {
      // Drives formatOptionalTime through both formatSectionHeader and
      // formatProvenance, confirming the ISO normalisation is shared.
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'Sec',
              source: 's',
              trustTier: 't',
              observedAt: '2026-01-02T03:04:05Z',
              facts: [
                {
                  label: 'L',
                  value: 'V',
                  observedAt: '2025-12-31T23:59:59Z',
                },
              ],
            }),
          ],
        }),
      );
      expect(text).toContain('## Sec (source=s, trust=t, observed=2026-01-02T03:04:05.000Z)');
      expect(text).toContain('- L: V (observed=2025-12-31T23:59:59.000Z)');
    });
  });

  describe('formatProvenance (transitive via facts)', () => {
    const sectionWithFacts = (facts: AgentResourceContextFact[]): AgentResourceContext =>
      baseContext({
        contextSections: [baseSection({ source: 's', trustTier: 't', facts })],
      });

    it('emits no provenance suffix when the fact has no optional fields (length===0 arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        sectionWithFacts([{ label: 'L', value: 'V' }]),
      );
      expect(text).toContain('- L: V\n');
      expect(text).not.toContain('- L: V (');
    });

    it('emits only source when only source is set', () => {
      const text = formatAgentResourceContextForClipboard(
        sectionWithFacts([{ label: 'L', value: 'V', source: 'agent' }]),
      );
      expect(text).toContain('- L: V (source=agent)\n');
    });

    it('emits only trust when only trustTier is set', () => {
      const text = formatAgentResourceContextForClipboard(
        sectionWithFacts([{ label: 'L', value: 'V', trustTier: 'verified' }]),
      );
      expect(text).toContain('- L: V (trust=verified)\n');
    });

    it('emits only observed when only observedAt is set', () => {
      const text = formatAgentResourceContextForClipboard(
        sectionWithFacts([{ label: 'L', value: 'V', observedAt: '2026-05-06T14:00:00Z' }]),
      );
      expect(text).toContain('- L: V (observed=2026-05-06T14:00:00.000Z)\n');
    });

    it('emits only redacted=true when only redacted is set', () => {
      const text = formatAgentResourceContextForClipboard(
        sectionWithFacts([{ label: 'L', value: 'V', redacted: true }]),
      );
      expect(text).toContain('- L: V (redacted=true)\n');
    });

    it('joins all four provenance parts with ", " in declared order (length>0 arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        sectionWithFacts([
          {
            label: 'L',
            value: 'V',
            source: 'agent',
            trustTier: 'verified',
            observedAt: '2026-05-06T14:00:00Z',
            redacted: true,
          },
        ]),
      );
      expect(text).toContain(
        '- L: V (source=agent, trust=verified, observed=2026-05-06T14:00:00.000Z, redacted=true)\n',
      );
    });

    it('treats falsy observedAt (invalid date) as raw text inside the provenance suffix', () => {
      // Combines the formatProvenance observedAt arm with the formatOptionalTime
      // NaN branch: `observed=not-a-date` is emitted verbatim.
      const text = formatAgentResourceContextForClipboard(
        sectionWithFacts([{ label: 'L', value: 'V', observedAt: 'garbage' }]),
      );
      expect(text).toContain('- L: V (observed=garbage)\n');
    });
  });

  describe('formatSectionHeader (transitive via sections)', () => {
    it('renders the plain header when no provenance fields are set (length===0 arm)', () => {
      // source and trustTier are typed as required strings, but empty strings
      // are falsy and exercise the `section.source ? ... : ''` arms.
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'Bare',
              source: '',
              trustTier: '',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      expect(text).toContain('## Bare\n');
      expect(text).not.toContain('## Bare (');
    });

    it('includes only source in the header when only source is non-empty', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'S',
              source: 'unified',
              trustTier: '',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      expect(text).toContain('## S (source=unified)\n');
    });

    it('includes only trust in the header when only trustTier is non-empty', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'S',
              source: '',
              trustTier: 'runtime-observed',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      expect(text).toContain('## S (trust=runtime-observed)\n');
    });

    it('includes only observed in the header when only observedAt is set', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'S',
              source: '',
              trustTier: '',
              observedAt: '2026-05-06T14:00:00Z',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      expect(text).toContain('## S (observed=2026-05-06T14:00:00.000Z)\n');
    });

    it('joins all three header provenance parts with ", " in declared order (length>0 arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'S',
              source: 'unified',
              trustTier: 'runtime-observed',
              observedAt: '2026-05-06T14:00:00Z',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      expect(text).toContain(
        '## S (source=unified, trust=runtime-observed, observed=2026-05-06T14:00:00.000Z)\n',
      );
    });
  });

  describe('formatRedaction (transitive via section.redactions)', () => {
    it('appends the reason with a " - " separator when reason is non-empty (if-true arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              facts: [{ label: 'L', value: 'V' }],
              redactions: [{ field: 'api-token', reason: 'secret' }],
            }),
          ],
        }),
      );
      expect(text).toContain('- Redaction: api-token - secret\n');
    });

    it('omits the separator and reason when reason is the empty string (if-false arm)', () => {
      // `reason` is typed as a required string; an empty string is a legal value
      // and exercises the falsy arm of `redaction.reason ? ... : ''` without
      // any cast.
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              facts: [{ label: 'L', value: 'V' }],
              redactions: [{ field: 'api-token', reason: '' }],
            }),
          ],
        }),
      );
      expect(text).toContain('- Redaction: api-token\n');
      expect(text).not.toContain('- Redaction: api-token -');
    });

    it('omits the separator when reason is undefined despite the typed contract (defensive arm)', () => {
      // Backend contract says reason is always present; this asserts the
      // defensive falsy branch still fires if the contract is violated.
      const redaction = {
        field: 'api-token',
        reason: undefined,
      } as unknown as AgentResourceContextRedaction;
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({ facts: [{ label: 'L', value: 'V' }], redactions: [redaction] }),
          ],
        }),
      );
      expect(text).toContain('- Redaction: api-token\n');
      expect(text).not.toContain('- Redaction: api-token -');
    });
  });

  describe('section iteration branches', () => {
    it('skips a section that has no facts and no redactions (continue arm, undefined redactions)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              id: 'skip-me',
              title: 'Skipped',
              source: 's',
              trustTier: 't',
              redactions: undefined,
            }),
            baseSection({
              id: 'keep-me',
              title: 'Kept',
              source: 's',
              trustTier: 't',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      expect(text).not.toContain('Skipped');
      expect(text).toContain('## Kept');
    });

    it('skips a section that has empty facts and an empty redactions array (continue arm, [])', () => {
      // `!section.redactions?.length` is true for length 0 as well as undefined.
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              id: 'skip-me',
              title: 'Skipped',
              source: 's',
              trustTier: 't',
              redactions: [],
            }),
          ],
        }),
      );
      expect(text).not.toContain('## Skipped');
    });

    it('keeps a section whose facts are empty but redactions are non-empty (renders header + redactions, no facts)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              id: 'redact-only',
              title: 'RedactOnly',
              source: 's',
              trustTier: 't',
              facts: [],
              redactions: [{ field: 'f', reason: 'r' }],
            }),
          ],
        }),
      );
      expect(text).toContain('## RedactOnly');
      expect(text).toContain('- Redaction: f - r');
      // And no fact bullet is emitted.
      expect(text).not.toMatch(/\n- [^R]/);
    });

    it('exercises the `?? []` right operand: facts present but redactions undefined', () => {
      // facts.length > 0 means the section is not skipped; `section.redactions`
      // is undefined, so the `?? []` fallback prevents a TypeError and the
      // redactions loop body is simply never entered.
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              id: 'facts-only',
              title: 'FactsOnly',
              source: 's',
              trustTier: 't',
              facts: [{ label: 'L', value: 'V' }],
              redactions: undefined,
            }),
          ],
        }),
      );
      expect(text).toContain('## FactsOnly');
      expect(text).toContain('- L: V\n');
      expect(text).not.toContain('Redaction:');
    });

    it('renders the summary line when section.summary is set (if-true arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'WithSummary',
              source: '',
              trustTier: '',
              summary: 'A short overview of the section.',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      // Summary is emitted between the bare header and the first fact bullet.
      expect(text).toContain('## WithSummary\nA short overview of the section.\n- L: V\n');
    });

    it('omits any summary line when section.summary is absent (if-false arm)', () => {
      const text = formatAgentResourceContextForClipboard(
        baseContext({
          contextSections: [
            baseSection({
              title: 'NoSummary',
              source: '',
              trustTier: '',
              facts: [{ label: 'L', value: 'V' }],
            }),
          ],
        }),
      );
      expect(text).not.toContain('overview');
      expect(text).toContain('## NoSummary\n- L: V\n');
    });
  });

  describe('output termination', () => {
    it('always ends with a single trailing newline after trim', () => {
      const text = formatAgentResourceContextForClipboard(baseContext());
      expect(text.endsWith('\n')).toBe(true);
      expect(text.endsWith('\n\n')).toBe(false);
    });
  });
});
