import { afterEach, describe, expect, it, vi } from 'vitest';
import { filterAssistantSlashCommands } from '../assistantSlashCommands';

// Branch-coverage companion to assistantSlashCommands.test.ts. Each test below
// drives a specific cold arm uncovered by the happy-path spec and asserts
// against concrete outputs rather than truthiness.
//
// Cold arms identified by scoped v8 coverage (coverage-final.json):
//   L66  - `import.meta.env.DEV || import.meta.env.MODE === 'test'`: the
//          right operand (`MODE === 'test'`) is never evaluated because DEV is
//          truthy in the test environment. Forcing DEV falsy exercises it.
//   L275 - commandMatchScore `return 2`: command name *contains* the query but
//          does not *start* with it (and no alias starts with it).
//   L276 - commandMatchScore `return 3`: an alias *contains* the query but no
//          alias/name *starts* with it and the name does not contain it.
//
// L214 (`if (!token) return null;`) is intentionally NOT exercised here: the
// regex `^(\S+)...` guarantees `match[1]` is a non-empty string, so `!token`
// is always false and the `return null` arm is provably unreachable through
// the public API. See GLM_REPORT_fe3-slashcmd.md for details.

const PROD_COMMAND_NAMES_WITHOUT_DEV = [
  'new',
  'sessions',
  'queue',
  'compact',
  'fork',
  'undo',
  'redo',
  'models',
  'providers',
  'status',
  'copy',
  'export',
  'help',
];

describe('assistantSlashCommands branch coverage', () => {
  describe('DEV-gated dev-command surface (L66 logical-OR right operand)', () => {
    const originalDev = import.meta.env.DEV;
    const originalMode = import.meta.env.MODE;

    afterEach(() => {
      vi.stubEnv('DEV', originalDev);
      vi.stubEnv('MODE', originalMode);
    });

    it('admits dev fixture commands when DEV is off but MODE is test (right operand true)', () => {
      vi.stubEnv('DEV', false);
      vi.stubEnv('MODE', 'test');
      const names = filterAssistantSlashCommands('', undefined, {}).map((command) => command.name);
      expect(names).toContain('fixture');
      // Sanity: the rest of the production surface is still present and ordered.
      expect(names).toEqual([...PROD_COMMAND_NAMES_WITHOUT_DEV, 'fixture']);
    });

    it('hides dev fixture commands when both DEV and MODE deny the dev surface (right operand false)', () => {
      vi.stubEnv('DEV', false);
      vi.stubEnv('MODE', 'production');
      const names = filterAssistantSlashCommands('', undefined, {}).map((command) => command.name);
      expect(names).not.toContain('fixture');
      expect(names).toEqual(PROD_COMMAND_NAMES_WITHOUT_DEV);
    });
  });

  describe('commandMatchScore name-substring ranking (L275 `return 2`)', () => {
    it('ranks a name-substring match ahead of a description-substring match', () => {
      // Two commands match "ew", so the sort comparator actually invokes
      // commandMatchScore: "new" contains "ew" in its name (name-substring,
      // score 2) while "fork" only matches via its description ("...into a new
      // copy", score 4). New must therefore sort first, driving the `return 2`
      // arm that a single-match query never reaches (the comparator is not
      // called on a one-element array).
      expect(filterAssistantSlashCommands('ew').map((command) => command.name)).toEqual([
        'new',
        'fork',
      ]);
    });
  });

  describe('commandMatchScore alias-substring ranking (L276 `return 3`)', () => {
    it('ranks an alias-substring match ahead of description-substring matches', () => {
      // Three commands match "ear", so the comparator scores each. "new" has
      // alias "clear" which contains "ear" but neither the name nor any alias
      // starts with "ear" and the name does not contain it -> alias-substring
      // arm (score 3). "sessions" and "models" match only via their
      // descriptions ("Search..."), scoring 4. New sorts first, exercising the
      // `return 3` arm.
      expect(filterAssistantSlashCommands('ear').map((command) => command.name)).toEqual([
        'new',
        'sessions',
        'models',
      ]);
    });
  });
});
