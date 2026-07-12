import { describe, expect, it } from 'vitest';
import { evaluateFilterStack, parseFilter, parseFilterStack } from '@/utils/searchQuery';
import type { ComparisonOperator, FilterStack, MetricCondition, ParsedFilter } from '@/utils/searchQuery';
import type { VM } from '@/types/api';

/**
 * Second branch-coverage companion to searchQuery.ts.
 *
 * Scope is limited to four named functions: `parseFilter`, `parseFilterStack`,
 * `evaluateMetricCondition`, and `evaluateTextCondition`. The latter two are
 * module-private (not exported) so they are driven through the exported
 * `evaluateFilterStack`, exactly as the sibling `searchQuery.branchcov.test.ts`
 * does. Helpers below mirror that file's fixture-builder style.
 *
 * Where the sibling file already exercised a branch with a single positive
 * case, this file adds the *opposite* arm or a precise boundary so each
 * conditional has a matched true/false pair. Branches that are genuinely
 * unreachable from the public string API are documented in GLM_REPORT.md
 * rather than forced with unsafe casts.
 */

// A plain-record base lets us `delete` keys to exercise the `'x' in guest`
// false arms (a key set to `undefined` still satisfies `in`).
const baseGuest = (): Record<string, unknown> => ({
  id: 'vm-1',
  vmid: 100,
  name: 'test-vm',
  node: 'node1',
  instance: 'qemu',
  status: 'running',
  type: 'qemu',
  cpu: 0.5, // 50%
  cpus: 2,
  memory: { usage: 1024, total: 2048, used: 1024, free: 1024 },
  disk: { usage: 50, total: 100, used: 50, free: 50 },
  networkIn: 100,
  networkOut: 200,
  diskRead: 300,
  diskWrite: 400,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  tags: ['production', 'web'],
  lock: '',
  lastSeen: '',
});

const makeGuest = (overrides: Record<string, unknown> = {}): VM =>
  ({ ...baseGuest(), ...overrides }) as unknown as VM;

/** Guest with the given keys removed entirely (so `'k' in guest` is false). */
const guestWithout = (keys: string[], overrides: Record<string, unknown> = {}): VM => {
  const g: Record<string, unknown> = { ...baseGuest(), ...overrides };
  for (const k of keys) delete g[k];
  return g as unknown as VM;
};

/** Build a single-filter stack from a raw ParsedFilter. */
const single = (filter: ParsedFilter): FilterStack => ({ filters: [filter], operators: [] });

const metric = (
  field: MetricCondition['field'],
  operator: ComparisonOperator,
  value: number,
): ParsedFilter => ({ type: 'metric', field, operator, value });

const text = (field: string, value: string): ParsedFilter => ({ type: 'text', field, value });

describe('parseFilter — both arms of every conditional', () => {
  describe('metric regex arm (lines 78-95)', () => {
    it('parses a tight metric term (no spaces) and lowercases the field', () => {
      expect(parseFilter('CPU>80')).toStrictEqual({
        type: 'metric',
        field: 'cpu',
        operator: '>',
        value: 80,
      });
    });

    it('parses the bare "=" operator distinctly from "=="', () => {
      expect(parseFilter('disk=50')).toStrictEqual({
        type: 'metric',
        field: 'disk',
        operator: '=',
        value: 50,
      });
      expect(parseFilter('disk==50')).toStrictEqual({
        type: 'metric',
        field: 'disk',
        operator: '==',
        value: 50,
      });
    });

    it('parses a decimal value and preserves the "<=" operator', () => {
      expect(parseFilter('memory<=12.5')).toStrictEqual({
        type: 'metric',
        field: 'memory',
        operator: '<=',
        value: 12.5,
      });
    });

    it('falls through to raw when the numeric tail is missing (operator only)', () => {
      expect(parseFilter('cpu>')).toStrictEqual({ type: 'raw', rawText: 'cpu>' });
    });

    it('falls through to raw when the value is non-numeric', () => {
      // The metric regex requires \d+(\.\d+)?; "high" fails it, and there is no
      // colon, so the raw fallback is reached.
      expect(parseFilter('cpu>high')).toStrictEqual({ type: 'raw', rawText: 'cpu>high' });
    });
  });

  describe('text regex arm (lines 98-105)', () => {
    it('trims whitespace around the value and lowercases the field', () => {
      expect(parseFilter('Name:   Prod   ')).toStrictEqual({
        type: 'text',
        field: 'name',
        value: 'Prod',
      });
    });

    it('keeps a colon inside the value (greedy value capture)', () => {
      expect(parseFilter('tags:key:value')).toStrictEqual({
        type: 'text',
        field: 'tags',
        value: 'key:value',
      });
    });

    it('returns raw (not text) when the value is empty after the colon', () => {
      // `^(\w+)\s*:\s*(.+)$` requires at least one value char, so "name:" misses
      // the text regex and lands in the raw fallback.
      expect(parseFilter('name:')).toStrictEqual({ type: 'raw', rawText: 'name:' });
    });

    it('returns raw when the term is a lone colon (no word field)', () => {
      expect(parseFilter(':value')).toStrictEqual({ type: 'raw', rawText: ':value' });
    });
  });

  describe('raw fallback arm (lines 109-112)', () => {
    it('preserves a multi-word raw term after trimming edges', () => {
      expect(parseFilter('  hello world  ')).toStrictEqual({
        type: 'raw',
        rawText: 'hello world',
      });
    });

    it('returns an empty rawText for a whitespace-only term', () => {
      expect(parseFilter('\t  \t')).toStrictEqual({ type: 'raw', rawText: '' });
    });
  });
});

describe('parseFilterStack — both arms of every conditional', () => {
  it('returns an empty stack for an empty string (the `!trimmed` early return)', () => {
    expect(parseFilterStack('')).toStrictEqual({ filters: [], operators: [] });
  });

  it('returns an empty stack for a whitespace-only string (the `!trimmed` early return)', () => {
    expect(parseFilterStack('   \t  ')).toStrictEqual({ filters: [], operators: [] });
  });

  it('parses a single filter with no operators into a one-element stack', () => {
    const result = parseFilterStack('cpu>80');
    expect(result).toStrictEqual({
      filters: [{ type: 'metric', field: 'cpu', operator: '>', value: 80 }],
      operators: [],
    });
  });

  it('threads each filter through parseFilter, mixing metric/text/raw kinds', () => {
    const result = parseFilterStack('cpu>80 AND name:prod OR loose-term');
    expect(result.filters).toStrictEqual([
      { type: 'metric', field: 'cpu', operator: '>', value: 80 },
      { type: 'text', field: 'name', value: 'prod' },
      { type: 'raw', rawText: 'loose-term' },
    ]);
    expect(result.operators).toStrictEqual(['AND', 'OR']);
  });

  it('normalizes a fully lowercase "and" operator to AND', () => {
    expect(parseFilterStack('cpu>80 and memory>10').operators).toStrictEqual(['AND']);
  });

  it('normalizes a fully lowercase "or" operator to OR', () => {
    expect(parseFilterStack('cpu>80 or memory>10').operators).toStrictEqual(['OR']);
  });

  it('keeps operator ordering stable across four filters', () => {
    const result = parseFilterStack('a:1 OR b:2 AND c:3 OR d:4');
    expect(result.filters).toHaveLength(4);
    expect(result.operators).toStrictEqual(['OR', 'AND', 'OR']);
  });

  it('treats a trailing operator token as part of the last filter (no split)', () => {
    // No whitespace follows the trailing "AND", so the split regex does not fire
    // there; the whole string survives as one part and parseFilter parses it as
    // a text condition whose value includes the trailing "AND".
    const result = parseFilterStack('a:1 AND');
    expect(result.filters).toStrictEqual([{ type: 'text', field: 'a', value: '1 AND' }]);
    expect(result.operators).toStrictEqual([]);
  });
});

describe('evaluateMetricCondition — field-switch arms (via evaluateFilterStack)', () => {
  describe('cpu field', () => {
    it('multiplies the decimal by 100; `>` true above and false at the threshold', () => {
      // 0.9 -> 90 ; 90 > 50 true
      expect(evaluateFilterStack(makeGuest({ cpu: 0.9 }), single(metric('cpu', '>', 50)))).toBe(true);
      // 0.4 -> 40 ; 40 > 50 false
      expect(evaluateFilterStack(makeGuest({ cpu: 0.4 }), single(metric('cpu', '>', 50)))).toBe(false);
    });
  });

  describe('memory field', () => {
    it('reads usage when present and compares with `<`', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ memory: { usage: 900, total: 1000, used: 900, free: 100 } }),
          single(metric('memory', '<', 1000)),
        ),
      ).toBe(true);
    });
  });

  describe('disk field', () => {
    it('reads usage when present (>= true at threshold)', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ disk: { usage: 50, total: 100, used: 50, free: 50 } }),
          single(metric('disk', '>=', 50)),
        ),
      ).toBe(true);
    });

    it('returns 0 when disk is null (falsy short-circuit)', () => {
      // null disk -> 0 ; 0 <= 0 true
      expect(
        evaluateFilterStack(
          makeGuest({ disk: null as unknown as VM['disk'] }),
          single(metric('disk', '<=', 0)),
        ),
      ).toBe(true);
    });
  });

  describe('uptime field', () => {
    it('uses uptime when running and truthy (> true)', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ status: 'running', uptime: 7200 }),
          single(metric('uptime', '>', 3600)),
        ),
      ).toBe(true);
    });

    it('coerces a falsy-but-present uptime (0) to 0 via the `|| 0` arm when running', () => {
      // status running + 'uptime' in guest (value 0) -> guest.uptime || 0 -> 0.
      // Pin the value to exactly 0: >= 0 true, > 0 false.
      expect(
        evaluateFilterStack(
          makeGuest({ status: 'running', uptime: 0 }),
          single(metric('uptime', '>=', 0)),
        ),
      ).toBe(true);
      expect(
        evaluateFilterStack(
          makeGuest({ status: 'running', uptime: 0 }),
          single(metric('uptime', '>', 0)),
        ),
      ).toBe(false);
    });

    it('returns 0 when status is not running (the `: 0` ternary arm)', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ status: 'stopped', uptime: 9999 }),
          single(metric('uptime', '>', 1)),
        ),
      ).toBe(false);
    });

    it('returns 0 when running but the uptime key is absent', () => {
      expect(
        evaluateFilterStack(guestWithout(['uptime']), single(metric('uptime', '>', 1))),
      ).toBe(false);
    });
  });

  describe('default field arm (switch-unlisted metric field)', () => {
    it('reads a numeric present field and compares with `>`', () => {
      expect(
        evaluateFilterStack(makeGuest({ networkOut: 200 }), single(metric('networkOut', '>', 150))),
      ).toBe(true);
      expect(
        evaluateFilterStack(makeGuest({ networkOut: 200 }), single(metric('networkOut', '>', 999))),
      ).toBe(false);
    });

    it('coerces a numeric-looking string field via Number() || 0', () => {
      // "300" -> 300 ; 300 >= 300 true
      expect(
        evaluateFilterStack(
          makeGuest({ diskRead: '300' as unknown as number }),
          single(metric('diskRead', '>=', 300)),
        ),
      ).toBe(true);
    });

    it('returns false when the field exists but its value is undefined', () => {
      expect(
        evaluateFilterStack(makeGuest({ networkIn: undefined }), single(metric('networkIn', '>', 0))),
      ).toBe(false);
    });

    it('returns false when the field is absent from the guest', () => {
      expect(
        evaluateFilterStack(guestWithout(['networkOut']), single(metric('networkOut', '>', 0))),
      ).toBe(false);
    });
  });

  describe('operator switch arms (>=, <=, >, <)', () => {
    // Columns: cpuPct, op, threshold, expected — value is cpuPct (cpu stored as
    // cpuPct/100, the function multiplies back by 100).
    it.each([
      [50, '>=', 50, true],
      [49, '>=', 50, false],
      [50, '<=', 50, true],
      [51, '<=', 50, false],
      [51, '>', 50, true],
      [50, '>', 50, false],
      [49, '<', 50, true],
      [50, '<', 50, false],
    ] as const)('cpu %i%% %s %i yields %s', (cpuPct, op, threshold, expected) => {
      expect(
        evaluateFilterStack(makeGuest({ cpu: cpuPct / 100 }), single(metric('cpu', op, threshold))),
      ).toBe(expected);
    });
  });

  describe('`=` / `==` epsilon arm (Math.abs(v - cond.value) < 0.01)', () => {
    it('matches within the 0.01 window for "="', () => {
      // 50.0 vs 50.005 -> diff 0.005 -> true
      expect(
        evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '=', 50.005))),
      ).toBe(true);
    });

    it('does not match outside the 0.01 window for "="', () => {
      // 50.0 vs 50.02 -> diff 0.02 -> false
      expect(
        evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '=', 50.02))),
      ).toBe(false);
    });

    it('matches exactly with "==" at the same value', () => {
      expect(
        evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '==', 50))),
      ).toBe(true);
    });

    it('does not match with "==" far from the value', () => {
      expect(
        evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '==', 51))),
      ).toBe(false);
    });
  });

  describe('operator default arm', () => {
    it('returns false for an unrecognized operator', () => {
      const filter = {
        type: 'metric',
        field: 'cpu',
        operator: '!=' as unknown as ComparisonOperator,
        value: 50,
      } as ParsedFilter;
      expect(evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(filter))).toBe(false);
    });
  });
});

describe('evaluateTextCondition — field-switch arms (via evaluateFilterStack)', () => {
  describe('name field', () => {
    it('matches case-insensitively and rejects a non-substring', () => {
      expect(evaluateFilterStack(makeGuest({ name: 'Prod-Web' }), single(text('name', 'prod')))).toBe(
        true,
      );
      expect(evaluateFilterStack(makeGuest({ name: 'Prod-Web' }), single(text('name', 'zzz')))).toBe(
        false,
      );
    });

    it('returns false when name is the empty string (falsy)', () => {
      expect(evaluateFilterStack(makeGuest({ name: '' }), single(text('name', 'x')))).toBe(false);
    });
  });

  describe('node field', () => {
    it('matches a substring and rejects a miss', () => {
      expect(
        evaluateFilterStack(makeGuest({ node: 'cluster-a' }), single(text('node', 'cluster'))),
      ).toBe(true);
      expect(
        evaluateFilterStack(makeGuest({ node: 'cluster-a' }), single(text('node', 'zone'))),
      ).toBe(false);
    });
  });

  describe('vmid field', () => {
    it('matches a numeric substring of vmid', () => {
      expect(evaluateFilterStack(makeGuest({ vmid: 100 }), single(text('vmid', '00')))).toBe(true);
    });

    it('returns false when vmid is falsy (0)', () => {
      expect(evaluateFilterStack(makeGuest({ vmid: 0 }), single(text('vmid', '0')))).toBe(false);
    });
  });

  describe('tags field', () => {
    it('matches when the second comma-separated search tag hits (OR over search tags)', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: ['production', 'web'] }),
          single(text('tags', 'staging,web')),
        ),
      ).toBe(true);
    });

    it('returns false when no search tag matches any guest tag', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: ['production', 'web'] }),
          single(text('tags', 'staging,db')),
        ),
      ).toBe(false);
    });

    it('matches case-insensitively across array tags and search value', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: ['Production'] }),
          single(text('tags', 'PROD')),
        ),
      ).toBe(true);
    });

    it('splits a comma-separated tags STRING and matches a segment', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: 'production,web,api' }),
          single(text('tags', 'api')),
        ),
      ).toBe(true);
    });

    it('returns false when the tags string contains only empty segments', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: ',,' }),
          single(text('tags', 'production')),
        ),
      ).toBe(false);
    });

    it('returns false when tags is a non-array, non-string value', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: 42 as unknown as string[] }),
          single(text('tags', 'x')),
        ),
      ).toBe(false);
    });

    it('returns false when an array holds only non-string entries', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: [1, 2] as unknown as string[] }),
          single(text('tags', 'x')),
        ),
      ).toBe(false);
    });

    it('returns false when the tags key is absent', () => {
      expect(evaluateFilterStack(guestWithout(['tags']), single(text('tags', 'x')))).toBe(false);
    });
  });

  describe('default field arm (non-name/node/vmid/tags)', () => {
    it('matches a string field via case-insensitive substring', () => {
      expect(evaluateFilterStack(makeGuest({ type: 'qemu' }), single(text('type', 'QE')))).toBe(true);
    });

    it('matches a numeric field via toString substring', () => {
      expect(evaluateFilterStack(makeGuest({ cpus: 2 }), single(text('cpus', '2')))).toBe(true);
    });

    it('boolean arm: a truthy `true` field matches the exact string "true"', () => {
      expect(
        evaluateFilterStack(makeGuest({ template: true }), single(text('template', 'true'))),
      ).toBe(true);
    });

    it('boolean arm uses EXACT equality (not substring): searching "tru" on true misses', () => {
      expect(
        evaluateFilterStack(makeGuest({ template: true }), single(text('template', 'tru'))),
      ).toBe(false);
    });

    // NOTE / suspected source bug documented in GLM_REPORT.md: a boolean `false`
    // field is filtered out by the outer `if (fieldValue)` truthiness guard and
    // can NEVER match, even when searching "false". This test pins the actual
    // (buggy) behavior so a future fix is caught.
    it('boolean arm: a falsy `false` field never matches because the outer truthiness guard skips it', () => {
      expect(
        evaluateFilterStack(makeGuest({ template: false }), single(text('template', 'false'))),
      ).toBe(false);
    });

    it('returns false for an object field value (no string/number/boolean arm)', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ cpus: { n: 1 } as unknown as number }),
          single(text('cpus', 'x')),
        ),
      ).toBe(false);
    });

    it('returns false when the default field is absent from the guest', () => {
      expect(evaluateFilterStack(guestWithout(['lock']), single(text('lock', 'x')))).toBe(false);
    });

    it('returns false when the default field value is falsy', () => {
      // 'lock' is '' on the base guest -> falsy -> the inner block is skipped.
      expect(evaluateFilterStack(makeGuest(), single(text('lock', 'x')))).toBe(false);
    });
  });
});
