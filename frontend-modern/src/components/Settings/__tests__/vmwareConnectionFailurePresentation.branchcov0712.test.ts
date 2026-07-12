/**
 * Branch-coverage tests for vmwareConnectionFailurePresentation.
 *
 * `normalizeOptionalText` is module-private (not exported); it is only
 * reachable through the exported `buildVMwareConnectionFailurePresentation`.
 * Each of its arms (optional-chain short-circuit on null/undefined, the
 * `.trim()` invocation arm on a real string, and the ternary's truthy vs
 * falsy operands when trim() yields a non-empty vs empty result) is driven
 * here through the observable `code` / `category` / `message` output fields
 * and through which `switch` arm the normalized category routes to.
 *
 * The exported builder's own branches — every `switch` arm, the default
 * fall-through, the `vmware_invalid_config` code guard, the message `??`
 * fallback, and the three `default* ?? <literal>` coalescences in the final
 * return — are each exercised with concrete object-shape assertions.
 */
import { describe, expect, it } from 'vitest';
import {
  buildVMwareConnectionFailurePresentation,
  type VMwareConnectionFailurePresentation,
} from '../vmwareConnectionFailurePresentation';

// The module-private input interface is not exported; derive it from the
// single exported builder so the fixture stays in sync with the real type.
type VMwareFailureInput = Parameters<typeof buildVMwareConnectionFailurePresentation>[0];

const makeInput = (overrides: Partial<VMwareFailureInput> = {}): VMwareFailureInput => ({
  fallback: 'Fallback message',
  ...overrides,
});

// ---- normalizeOptionalText (private) ---------------------------------------
//
// `normalizeOptionalText = (value) => { const trimmed = value?.trim(); return trimmed ? trimmed : undefined; }`
// Branches:
//   (a) value is null/undefined  -> `?.` short-circuits, trimmed is undefined, ternary falsy -> undefined
//   (b) value is a non-empty string -> `.trim()` invoked, result non-empty, ternary truthy -> trimmed value
//   (c) value is whitespace-only -> `.trim()` invoked, result '' empty, ternary falsy -> undefined
//   (d) value is '' empty string -> `.trim()` invoked, result '' empty, ternary falsy -> undefined
// All four are observable through the builder's `code` / `category` / `message` fields.

describe('normalizeOptionalText (private) via buildVMwareConnectionFailurePresentation', () => {
  it('short-circuits the optional chain for an undefined value (code -> undefined)', () => {
    const out = buildVMwareConnectionFailurePresentation(makeInput({ code: undefined }));
    expect(out.code).toBeUndefined();
  });

  it('short-circuits the optional chain for a null value (category -> undefined, routes to switch default)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ category: null, code: 'irrelevant_code' }),
    );
    expect(out.category).toBeUndefined();
    // category undefined hits the switch `default` arm, code is not the magic
    // value, so the final-return defaults are observed.
    expect(out.title).toBe('VMware connection test failed');
    expect(out.tone).toBe('danger');
  });

  it('trims a non-empty padded string and returns the trimmed value for code', () => {
    const out = buildVMwareConnectionFailurePresentation(makeInput({ code: '  AUTH_001  ' }));
    expect(out.code).toBe('AUTH_001');
  });

  it('returns undefined for a whitespace-only code (trim() -> "" -> falsy ternary)', () => {
    const out = buildVMwareConnectionFailurePresentation(makeInput({ code: '   \t  ' }));
    expect(out.code).toBeUndefined();
  });

  it('returns undefined for an empty-string code', () => {
    const out = buildVMwareConnectionFailurePresentation(makeInput({ code: '' }));
    expect(out.code).toBeUndefined();
  });

  it('trims a padded category and routes the normalized value to the matching switch arm', () => {
    const out = buildVMwareConnectionFailurePresentation(makeInput({ category: '  tls  ' }));
    expect(out.category).toBe('tls');
    expect(out.title).toBe('TLS validation failed');
  });
});

// ---- buildVMwareConnectionFailurePresentation: switch arms ------------------

describe('buildVMwareConnectionFailurePresentation — each named category switch arm', () => {
  it.each<{
    category: NonNullable<VMwareFailureInput['category']>;
    title: string;
    tone: VMwareConnectionFailurePresentation['tone'];
    guidance: string;
  }>([
    {
      category: 'unsupported_version',
      title: 'Unsupported vCenter version',
      tone: 'warning',
      guidance:
        'Use a supported vCenter release within the current VI JSON phase-1 floor, then retry this connection test.',
    },
    {
      category: 'tls',
      title: 'TLS validation failed',
      tone: 'warning',
      guidance:
        'Install a trusted certificate for vCenter, or enable Skip TLS verification only for controlled lab environments.',
    },
    {
      category: 'auth',
      title: 'Authentication failed',
      tone: 'danger',
      guidance: 'Verify the username, password, and account scope in vCenter before retrying.',
    },
    {
      category: 'permission',
      title: 'Permissions are insufficient',
      tone: 'warning',
      guidance:
        'Grant the minimum VMware read privileges required for phase-1 inventory and health reads, then retry.',
    },
    {
      category: 'network',
      title: 'Pulse could not reach vCenter',
      tone: 'danger',
      guidance:
        'Confirm DNS, reachability, port 443, and any firewall rules from the Pulse server to vCenter.',
    },
  ])(
    'returns the exact presentation for category "$category"',
    ({ category, title, tone, guidance }) => {
      const out = buildVMwareConnectionFailurePresentation(
        makeInput({ code: 'CODE_1', category, message: 'Real message' }),
      );
      expect(out).toStrictEqual({
        code: 'CODE_1',
        category,
        guidance,
        message: 'Real message',
        title,
        tone,
      });
    },
  );

  it('passes the normalized code/category/message through verbatim for a known category', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ code: '  NET_503  ', category: '  network  ', message: '  timed out  ' }),
    );
    expect(out).toStrictEqual({
      code: 'NET_503',
      category: 'network',
      guidance:
        'Confirm DNS, reachability, port 443, and any firewall rules from the Pulse server to vCenter.',
      message: 'timed out',
      title: 'Pulse could not reach vCenter',
      tone: 'danger',
    });
  });
});

// ---- buildVMwareConnectionFailurePresentation: switch default fall-through --
//
// A category that is not one of the five named values (including the
// normalized-undefined case) falls through the switch's `default -> break`
// and continues to the `vmware_invalid_config` guard / final return.

describe('buildVMwareConnectionFailurePresentation — switch default fall-through', () => {
  it('falls through the switch for an unknown category string', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ category: 'unknown_blob', code: 'CODE_X' }),
    );
    expect(out.category).toBe('unknown_blob');
    // code is not the magic value -> final return with defaults.
    expect(out.title).toBe('VMware connection test failed');
    expect(out.tone).toBe('danger');
    expect(out.guidance).toBe(
      'Review the vCenter endpoint and credentials, then retry the connection test.',
    );
  });
});

// ---- buildVMwareConnectionFailurePresentation: vmware_invalid_config guard --

describe('buildVMwareConnectionFailurePresentation — vmware_invalid_config code guard', () => {
  it('returns the invalid-config presentation when the normalized code matches (guard true arm)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ code: 'vmware_invalid_config', category: 'unknown_blob' }),
    );
    expect(out).toStrictEqual({
      code: 'vmware_invalid_config',
      category: 'unknown_blob',
      guidance: 'Review the host, port, username, and password fields before retrying.',
      message: 'Fallback message',
      title: 'Connection configuration is invalid',
      tone: 'danger',
    });
  });

  it('trims the code before comparing against the magic value', () => {
    // normalizeOptionalText trims first, so ' vmware_invalid_config ' still
    // satisfies the guard.
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ code: '  vmware_invalid_config  ', category: 'mystery' }),
    );
    expect(out.code).toBe('vmware_invalid_config');
    expect(out.title).toBe('Connection configuration is invalid');
  });

  it('skips the guard when the code is some other value (guard false arm -> final return)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ code: 'some_other_code', category: 'mystery' }),
    );
    expect(out.title).toBe('VMware connection test failed');
    expect(out.tone).toBe('danger');
  });
});

// ---- buildVMwareConnectionFailurePresentation: message ?? fallback ----------

describe('buildVMwareConnectionFailurePresentation — message ?? fallback', () => {
  it('uses the normalized message when it is a non-empty string (?? left operand)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ category: 'tls', message: '  vCenter rejected the cert  ', fallback: 'Unused' }),
    );
    expect(out.message).toBe('vCenter rejected the cert');
  });

  it('falls back to input.fallback when message is null (?? right operand)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ category: 'tls', message: null, fallback: 'Fallback message' }),
    );
    expect(out.message).toBe('Fallback message');
  });

  it('falls back to input.fallback when message is undefined', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ category: 'tls', message: undefined, fallback: 'Fallback message' }),
    );
    expect(out.message).toBe('Fallback message');
  });

  it('falls back to input.fallback when message is empty (normalizeOptionalText -> undefined)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ category: 'tls', message: '   ', fallback: 'Fallback message' }),
    );
    expect(out.message).toBe('Fallback message');
  });
});

// ---- buildVMwareConnectionFailurePresentation: final-return default* ?? ----
//
// The final return uses three nullish coalescences:
//   guidance: input.defaultGuidance ?? '<hardcoded>'
//   title:    input.defaultTitle    ?? '<hardcoded>'
//   tone:     input.defaultTone     ?? 'danger'

describe('buildVMwareConnectionFailurePresentation — final-return default* coalescences', () => {
  it('uses the hardcoded defaults when none of defaultGuidance/defaultTitle/defaultTone is supplied (all ?? right operands)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({ category: 'mystery', code: 'CODE_Y' }),
    );
    expect(out).toStrictEqual({
      code: 'CODE_Y',
      category: 'mystery',
      guidance:
        'Review the vCenter endpoint and credentials, then retry the connection test.',
      message: 'Fallback message',
      title: 'VMware connection test failed',
      tone: 'danger',
    });
  });

  it('honours caller-supplied defaultGuidance/defaultTitle/defaultTone (all ?? left operands)', () => {
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({
        category: 'mystery',
        code: 'CODE_Z',
        defaultGuidance: 'Custom guidance body.',
        defaultTitle: 'Custom title',
        defaultTone: 'warning',
      }),
    );
    expect(out).toStrictEqual({
      code: 'CODE_Z',
      category: 'mystery',
      guidance: 'Custom guidance body.',
      message: 'Fallback message',
      title: 'Custom title',
      tone: 'warning',
    });
  });

  it('treats empty-string default* values as present (?? is nullish, not falsy)', () => {
    // '' is not null/undefined, so the left operand wins even though it is falsy.
    const out = buildVMwareConnectionFailurePresentation(
      makeInput({
        category: 'mystery',
        code: 'CODE_W',
        defaultGuidance: '',
        defaultTitle: '',
      }),
    );
    expect(out.guidance).toBe('');
    expect(out.title).toBe('');
  });
});
