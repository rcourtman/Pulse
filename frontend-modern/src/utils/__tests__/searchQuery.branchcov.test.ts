import { describe, expect, it } from 'vitest';
import { evaluateFilterStack, parseFilter, parseFilterStack } from '@/utils/searchQuery';
import type {
  ComparisonOperator,
  FilterStack,
  MetricCondition,
  ParsedFilter,
} from '@/utils/searchQuery';
import type { VM } from '@/types/api';

/**
 * Branch-coverage companion to searchQuery.test.ts.
 *
 * `evaluateMetricCondition` and `evaluateTextCondition` are module-private
 * (not exported) but reachable through the exported `evaluateFilterStack`.
 * We drive them by constructing `ParsedFilter` / `FilterStack` objects directly
 * so each guard / switch arm / ternary is exercised independently of the parser.
 */

// A plain record base lets us `delete` keys to exercise the `'x' in guest`
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

/** Metric filter helper (value defaults to a number so the metric arm runs). */
const metric = (
  field: MetricCondition['field'],
  operator: ComparisonOperator,
  value: number,
): ParsedFilter => ({ type: 'metric', field, operator, value });

describe('evaluateMetricCondition — field switch branches (via evaluateFilterStack)', () => {
  it('cpu: converts the decimal to a percentage and compares with >=', () => {
    // cpu 0.5 -> 50%; 50 >= 50 is true
    expect(evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '>=', 50)))).toBe(
      true,
    );
  });

  it('cpu: >= is false below the threshold', () => {
    expect(evaluateFilterStack(makeGuest({ cpu: 0.49 }), single(metric('cpu', '>=', 50)))).toBe(
      false,
    );
  });

  it('cpu: <= is true at the threshold and false above', () => {
    expect(evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '<=', 50)))).toBe(
      true,
    );
    expect(evaluateFilterStack(makeGuest({ cpu: 0.8 }), single(metric('cpu', '<=', 50)))).toBe(
      false,
    );
  });

  it('cpu: = operator matches within the 0.01 epsilon', () => {
    expect(evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '=', 50)))).toBe(true);
    expect(evaluateFilterStack(makeGuest({ cpu: 0.6 }), single(metric('cpu', '=', 50)))).toBe(
      false,
    );
  });

  it('cpu: == operator matches within the 0.01 epsilon', () => {
    expect(evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(metric('cpu', '==', 50)))).toBe(
      true,
    );
  });

  it('cpu: falsy cpu (0) falls back to 0 via the || 0 arm', () => {
    // 0 -> 0%; 0 >= 0 true, exercising guest.cpu || 0
    expect(evaluateFilterStack(makeGuest({ cpu: 0 }), single(metric('cpu', '>=', 0)))).toBe(true);
  });

  it('cpu: missing cpu key uses the : 0 ternary arm', () => {
    // 'cpu' not in guest -> value 0; 0 < 1 true
    expect(evaluateFilterStack(guestWithout(['cpu']), single(metric('cpu', '<', 1)))).toBe(true);
  });

  it('memory: falsy/missing memory yields 0', () => {
    // memory null -> guest.memory falsy -> 0; 0 > -1 true
    expect(
      evaluateFilterStack(
        makeGuest({ memory: null as unknown as VM['memory'] }),
        single(metric('memory', '>', -1)),
      ),
    ).toBe(true);
    // memory key absent -> 0; 0 <= 0 true
    expect(evaluateFilterStack(guestWithout(['memory']), single(metric('memory', '<=', 0)))).toBe(
      true,
    );
  });

  it('memory: compares the raw usage value when present', () => {
    expect(
      evaluateFilterStack(
        makeGuest({ memory: { usage: 900, total: 1000, used: 900, free: 100 } }),
        single(metric('memory', '>', 500)),
      ),
    ).toBe(true);
  });

  it('disk: falsy/missing disk yields 0', () => {
    expect(
      evaluateFilterStack(
        makeGuest({ disk: null as unknown as VM['disk'] }),
        single(metric('disk', '<', 1)),
      ),
    ).toBe(true);
    expect(evaluateFilterStack(guestWithout(['disk']), single(metric('disk', '<', 1)))).toBe(true);
  });

  it('uptime: running guest uses the uptime value', () => {
    expect(
      evaluateFilterStack(
        makeGuest({ status: 'running', uptime: 7200 }),
        single(metric('uptime', '>', 3600)),
      ),
    ).toBe(true);
  });

  it('uptime: non-running status yields 0', () => {
    expect(
      evaluateFilterStack(
        makeGuest({ status: 'stopped', uptime: 9999 }),
        single(metric('uptime', '>', 3600)),
      ),
    ).toBe(false);
  });

  it('uptime: running status but missing uptime key yields 0', () => {
    expect(evaluateFilterStack(guestWithout(['uptime']), single(metric('uptime', '>', 1)))).toBe(
      false,
    );
  });

  it('default field: a switch-unlisted metric field present on the guest is read numerically', () => {
    // 'networkIn' is a valid MetricCondition field but not in the switch; falls to default.
    // networkIn 100 > 50 -> true
    expect(
      evaluateFilterStack(makeGuest({ networkIn: 100 }), single(metric('networkIn', '>', 50))),
    ).toBe(true);
    expect(
      evaluateFilterStack(makeGuest({ networkIn: 100 }), single(metric('networkIn', '>', 999))),
    ).toBe(false);
  });

  it('default field: non-numeric field value coerces to 0 via Number() || 0', () => {
    // diskRead stored as a non-numeric string -> Number('abc') = NaN -> || 0 = 0
    expect(
      evaluateFilterStack(
        makeGuest({ diskRead: 'abc' as unknown as number }),
        single(metric('diskRead', '<', 1)),
      ),
    ).toBe(true);
  });

  it('default field: field present but value undefined returns false', () => {
    expect(
      evaluateFilterStack(
        makeGuest({ networkOut: undefined }),
        single(metric('networkOut', '>', 0)),
      ),
    ).toBe(false);
  });

  it('default field: field absent from the guest returns false', () => {
    expect(
      evaluateFilterStack(guestWithout(['networkIn']), single(metric('networkIn', '>', 0))),
    ).toBe(false);
  });

  it('operator default: an unknown operator returns false', () => {
    const filter = {
      type: 'metric',
      field: 'cpu',
      operator: '!=' as unknown as ComparisonOperator,
      value: 50,
    } as ParsedFilter;
    expect(evaluateFilterStack(makeGuest({ cpu: 0.5 }), single(filter))).toBe(false);
  });
});

describe('evaluateTextCondition — field switch branches (via evaluateFilterStack)', () => {
  const text = (field: string, value: string): ParsedFilter => ({ type: 'text', field, value });

  it('name: missing/empty name returns false', () => {
    expect(evaluateFilterStack(guestWithout(['name']), single(text('name', 'test')))).toBe(false);
    expect(evaluateFilterStack(makeGuest({ name: '' }), single(text('name', 'test')))).toBe(false);
  });

  it('name: case-insensitive substring match', () => {
    expect(evaluateFilterStack(makeGuest({ name: 'Prod-Web' }), single(text('name', 'prod')))).toBe(
      true,
    );
  });

  it('node: missing node returns false', () => {
    expect(evaluateFilterStack(guestWithout(['node']), single(text('node', 'x')))).toBe(false);
  });

  it('node: substring match', () => {
    expect(
      evaluateFilterStack(makeGuest({ node: 'cluster-a' }), single(text('node', 'cluster'))),
    ).toBe(true);
  });

  it('vmid: falsy vmid (0) returns false', () => {
    expect(evaluateFilterStack(makeGuest({ vmid: 0 }), single(text('vmid', '0')))).toBe(false);
  });

  it('vmid: numeric substring match', () => {
    expect(evaluateFilterStack(makeGuest({ vmid: 100 }), single(text('vmid', '10')))).toBe(true);
  });

  describe('tags field', () => {
    it('returns false when tags key is absent', () => {
      expect(evaluateFilterStack(guestWithout(['tags']), single(text('tags', 'x')))).toBe(false);
    });

    it('returns false when tags is null', () => {
      expect(evaluateFilterStack(makeGuest({ tags: null }), single(text('tags', 'x')))).toBe(false);
    });

    it('returns false for an empty tags array', () => {
      expect(evaluateFilterStack(makeGuest({ tags: [] }), single(text('tags', 'x')))).toBe(false);
    });

    it('returns false when an array contains only non-string entries', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: [1, 2] as unknown as string[] }),
          single(text('tags', 'x')),
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

    it('matches any tag in a comma-separated search (OR) — positive', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ tags: ['production', 'web'] }),
          single(text('tags', 'staging,web')),
        ),
      ).toBe(true);
    });

    it('splits a comma-separated tags string and matches', () => {
      expect(
        evaluateFilterStack(makeGuest({ tags: 'production,web,api' }), single(text('tags', 'api'))),
      ).toBe(true);
    });

    it('ignores empty segments in a comma-separated tag search', () => {
      // ',,' splits to [] after filtering -> no search tags -> some([]) is false
      expect(
        evaluateFilterStack(makeGuest({ tags: ['production'] }), single(text('tags', ',,'))),
      ).toBe(false);
    });
  });

  describe('default field (non-name/node/vmid/tags)', () => {
    it('matches a string field value (substring, case-insensitive)', () => {
      // 'type' is not in the TextCondition switch; falls to default with a string value.
      expect(evaluateFilterStack(makeGuest({ type: 'qemu' }), single(text('type', 'qe')))).toBe(
        true,
      );
      expect(evaluateFilterStack(makeGuest({ type: 'qemu' }), single(text('type', 'ZZZ')))).toBe(
        false,
      );
    });

    it('matches a numeric field value via toString', () => {
      // 'cpus' is numeric on the guest; search '2'
      expect(evaluateFilterStack(makeGuest({ cpus: 2 }), single(text('cpus', '2')))).toBe(true);
    });

    it('matches a boolean field value against its string form', () => {
      expect(
        evaluateFilterStack(makeGuest({ template: true }), single(text('template', 'true'))),
      ).toBe(true);
      expect(
        evaluateFilterStack(makeGuest({ template: true }), single(text('template', 'false'))),
      ).toBe(false);
    });

    it('returns false when the default field value is falsy', () => {
      // 'lock' is '' on the base guest -> falsy -> false
      expect(evaluateFilterStack(makeGuest(), single(text('lock', 'x')))).toBe(false);
    });

    it('returns false when the default field is absent from the guest', () => {
      expect(evaluateFilterStack(guestWithout(['lock']), single(text('lock', 'x')))).toBe(false);
    });

    it('returns false for a non-string/number/boolean field value', () => {
      expect(
        evaluateFilterStack(
          makeGuest({ cpus: { n: 1 } as unknown as number }),
          single(text('cpus', 'x')),
        ),
      ).toBe(false);
    });
  });
});

describe('evaluateFilterStack — top-level branches', () => {
  it('returns true for a single metric filter whose result is true (results.length === 1 fast path)', () => {
    expect(evaluateFilterStack(makeGuest({ cpu: 0.9 }), single(metric('cpu', '>', 50)))).toBe(true);
  });

  it('malformed metric filter (missing value) falls through to the true catch-all', () => {
    const filter = { type: 'metric', field: 'cpu', operator: '>' } as ParsedFilter;
    expect(evaluateFilterStack(makeGuest(), single(filter))).toBe(true);
  });

  it('text filter with a falsy value falls through to the true catch-all', () => {
    const filter = { type: 'text', field: 'name', value: '' } as ParsedFilter;
    expect(evaluateFilterStack(makeGuest(), single(filter))).toBe(true);
  });

  it('raw filter with empty rawText falls through to the true catch-all', () => {
    const filter = { type: 'raw', rawText: '' } as ParsedFilter;
    expect(evaluateFilterStack(makeGuest(), single(filter))).toBe(true);
  });

  it('an unknown filter type falls through to the true catch-all', () => {
    const filter = { type: 'bogus' } as unknown as ParsedFilter;
    expect(evaluateFilterStack(makeGuest(), single(filter))).toBe(true);
  });

  describe('raw text matching across fields', () => {
    it('matches raw text against node', () => {
      expect(
        evaluateFilterStack(makeGuest({ node: 'cluster-a' }), single(parseFilter('cluster'))),
      ).toBe(true);
    });

    it('matches raw text against vmid', () => {
      expect(evaluateFilterStack(makeGuest({ vmid: 100 }), single(parseFilter('100')))).toBe(true);
    });

    it('returns false when raw text matches no field', () => {
      const guest = makeGuest({ name: 'alpha', node: 'beta', status: 'running', tags: [] });
      expect(evaluateFilterStack(guest, single(parseFilter('zzz')))).toBe(false);
    });

    it('raw tag match is false when tags is not an array', () => {
      const guest = makeGuest({ tags: 'production,web' });
      // Raw matcher only inspects tags when Array.isArray(...) is true; the needle below
      // matches none of name/node/vmid/status, so the non-array tags branch yields false.
      const stack = single(parseFilter('production'));
      expect(evaluateFilterStack(guest, stack)).toBe(false);
    });
  });

  describe('operator combination between multiple filters', () => {
    it('AND short-circuits to false when the first operand is false', () => {
      const stack: FilterStack = {
        filters: [metric('cpu', '>', 50), { type: 'text', field: 'name', value: 'test' }],
        operators: ['AND'],
      };
      expect(evaluateFilterStack(makeGuest({ cpu: 0.1, name: 'test-vm' }), stack)).toBe(false);
    });

    it('OR yields true when the second operand is true', () => {
      const stack: FilterStack = {
        filters: [metric('cpu', '>', 50), { type: 'text', field: 'name', value: 'test' }],
        operators: ['OR'],
      };
      expect(evaluateFilterStack(makeGuest({ cpu: 0.1, name: 'test-vm' }), stack)).toBe(true);
    });

    it('OR yields false when both operands are false', () => {
      const stack: FilterStack = {
        filters: [metric('cpu', '>', 50), { type: 'text', field: 'name', value: 'zzz' }],
        operators: ['OR'],
      };
      expect(evaluateFilterStack(makeGuest({ cpu: 0.1, name: 'test-vm' }), stack)).toBe(false);
    });

    it('left-to-right reduction with three filters and two operators', () => {
      // (false OR true) AND false -> false
      const stack: FilterStack = {
        filters: [
          metric('cpu', '>', 99),
          { type: 'text', field: 'name', value: 'test' },
          metric('disk', '>', 99),
        ],
        operators: ['OR', 'AND'],
      };
      expect(evaluateFilterStack(makeGuest({ cpu: 0.5, name: 'test-vm' }), stack)).toBe(false);
    });

    it('stops reducing when operators run out before filters (operator array shorter)', () => {
      // 3 filters, only 1 operator -> third filter result is never combined.
      // (true AND false) ... ignored -> false
      const stack: FilterStack = {
        filters: [
          { type: 'text', field: 'name', value: 'test' },
          metric('cpu', '>', 99),
          metric('disk', '>', 99),
        ],
        operators: ['AND'],
      };
      expect(
        evaluateFilterStack(
          makeGuest({
            name: 'test-vm',
            cpu: 0.5,
            disk: { usage: 50, total: 100, used: 50, free: 50 },
          }),
          stack,
        ),
      ).toBe(false);
    });
  });
});

describe('parseFilter — uncovered parser branches', () => {
  it('parses a metric term with whitespace around the operator', () => {
    expect(parseFilter('cpu  >=  50')).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>=',
      value: 50,
    });
  });

  it('parses a metric term with a decimal value and == operator with spaces', () => {
    expect(parseFilter('cpu == 50.5')).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '==',
      value: 50.5,
    });
  });

  it('parses a text term with whitespace around the colon', () => {
    expect(parseFilter('name  :  prod')).toEqual({
      type: 'text',
      field: 'name',
      value: 'prod',
    });
  });

  it('preserves internal spaces in a text value', () => {
    expect(parseFilter('name:prod server')).toEqual({
      type: 'text',
      field: 'name',
      value: 'prod server',
    });
  });

  it('treats a lone dash as raw text', () => {
    expect(parseFilter('-')).toEqual({ type: 'raw', rawText: '-' });
  });

  it('treats an operator-led term (no field) as raw text', () => {
    expect(parseFilter('>50')).toEqual({ type: 'raw', rawText: '>50' });
  });

  it('treats an empty string after trim as raw text', () => {
    expect(parseFilter('   ')).toEqual({ type: 'raw', rawText: '' });
  });

  it('lowercases a mixed-case field name for text conditions', () => {
    expect(parseFilter('NODE:cluster')).toEqual({
      type: 'text',
      field: 'node',
      value: 'cluster',
    });
  });
});

describe('parseFilterStack — uncovered stack branches', () => {
  it('parses a lowercase "or" operator', () => {
    const result = parseFilterStack('name:a or name:b');
    expect(result.operators).toEqual(['OR']);
    expect(result.filters).toHaveLength(2);
  });

  it('parses with multiple spaces between filters and operator', () => {
    const result = parseFilterStack('cpu>80    AND    memory<50');
    expect(result.operators).toEqual(['AND']);
    expect(result.filters).toHaveLength(2);
  });

  it('uppercases a mixed-case "Or" operator', () => {
    const result = parseFilterStack('name:a Or name:b');
    expect(result.operators).toEqual(['OR']);
  });

  it('preserves operator order across a three-filter stack with mixed operators', () => {
    const result = parseFilterStack('cpu>80 AND name:prod OR tags:web');
    expect(result.filters).toHaveLength(3);
    expect(result.operators).toEqual(['AND', 'OR']);
  });

  it('parses a leading operator token as a single raw filter', () => {
    // No leading whitespace before the operator token, so it does not split.
    // The raw fallback preserves the original term casing.
    const result = parseFilterStack('AND cpu>80');
    expect(result.filters).toHaveLength(1);
    expect(result.filters[0]).toEqual({ type: 'raw', rawText: 'AND cpu>80' });
    expect(result.operators).toEqual([]);
  });
});
