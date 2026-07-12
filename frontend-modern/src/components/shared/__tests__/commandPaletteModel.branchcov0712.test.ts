import { describe, expect, it } from 'vitest';
import {
  filterCommandPaletteCommands,
  type CommandPaletteModalCommand,
} from '@/components/shared/commandPaletteModel';

// Branch-coverage suite focused on the still-uncovered
// `filterCommandPaletteCommands` export of commandPaletteModel.ts. Every
// assertion is a concrete equality/strict-equality check against the real
// return shape so it pins the exact branch taken (no truthiness-only checks).
//
// Branch map under test:
//   1. `if (!normalizedQuery) return commands` — empty/whitespace early-return.
//      Notably this returns the *same array reference* as the input.
//   2. `command.description ?? ''` — present arm vs absent arm.
//   3. `command.shortcut ?? ''`    — present arm vs absent arm.
//   4. `...(command.keywords ?? [])` — present arm (spread) vs absent arm.
//   5. `return haystack.includes(normalizedQuery)` — match arm vs no-match arm.
//   6. Query normalization: lowercase, trim, and internal-whitespace collapse
//      (delegated to `normalizeCommandPaletteQuery`).
//   7. Haystack normalization: `.join(' ').toLowerCase().replace(/\s+/g, '')`
//      — internal whitespace is removed from the haystack too.

function makeCommand(
  overrides: Partial<CommandPaletteModalCommand> = {},
): CommandPaletteModalCommand {
  return {
    id: 'cmd-1',
    label: 'Go to Proxmox',
    description: '/proxmox',
    shortcut: 'g p',
    keywords: ['proxmox', 'pve', 'backups'],
    action: () => {},
    ...overrides,
  };
}

describe('commandPaletteModel.branchcov2', () => {
  describe('filterCommandPaletteCommands', () => {
    describe('empty-query early return (`!normalizedQuery` arm)', () => {
      it('returns the exact same array reference for a literally empty query', () => {
        const commands = [makeCommand()];
        // The early `return commands` hands back the SAME array, not a copy.
        expect(filterCommandPaletteCommands(commands, '')).toBe(commands);
      });

      it('returns the same array reference for a whitespace-only query (normalizes to empty)', () => {
        const commands = [makeCommand(), makeCommand({ id: 'cmd-2' })];
        // '   ' -> normalizeCommandPaletteQuery -> '' -> falsy -> early return.
        expect(filterCommandPaletteCommands(commands, '   ')).toBe(commands);
      });

      it('returns the same array reference for a query that is only internal whitespace + tabs/newlines', () => {
        const commands = [makeCommand()];
        // '\t \n' trims to '' and is therefore falsy.
        expect(filterCommandPaletteCommands(commands, '\t \n')).toBe(commands);
      });

      it('returns the full input unchanged (contents identical) when query is empty', () => {
        const commands = [
          makeCommand({ id: 'a' }),
          makeCommand({ id: 'b' }),
          makeCommand({ id: 'c' }),
        ];
        const result = filterCommandPaletteCommands(commands, '');
        expect(result).toStrictEqual(commands);
        expect(result.map((c) => c.id)).toStrictEqual(['a', 'b', 'c']);
      });
    });

    describe('`description ?? ""` arm', () => {
      it('matches a query that only appears in description when description is present', () => {
        // description '/proxmox' is the only field containing the substring.
        const command = makeCommand({
          label: 'Unrelated Label',
          shortcut: 'x y',
          keywords: ['nothing', 'here'],
          description: '/unique-desc-token',
        });
        const result = filterCommandPaletteCommands([command], '/unique-desc-token');
        expect(result).toStrictEqual([command]);
      });

      it('does NOT leak "undefined" into the haystack when description is absent', () => {
        // If the `?? ''` arm were broken (e.g. raw `command.description`), the
        // string 'undefined' would appear in the haystack and a query of
        // 'undefined' would falsely match. Pin the absent-description arm.
        const command: CommandPaletteModalCommand = {
          id: 'no-desc',
          label: 'Go to Docker',
          shortcut: 'g d',
          keywords: ['docker'],
          action: () => {},
          // description intentionally omitted
        };
        const result = filterCommandPaletteCommands([command], 'undefined');
        expect(result).toStrictEqual([]);
      });
    });

    describe('`shortcut ?? ""` arm', () => {
      it('matches a query that only appears in shortcut when shortcut is present', () => {
        // After normalization both query 'gp' and haystack collapse the
        // shortcut 'g p' into 'gp', so the substring matches there and only
        // there (label/description/keywords avoid the literal 'gp' run).
        const command = makeCommand({
          label: 'Alpha',
          description: 'beta',
          keywords: ['gamma', 'delta'],
          shortcut: 'g p',
        });
        const result = filterCommandPaletteCommands([command], 'gp');
        expect(result).toStrictEqual([command]);
      });

      it('does NOT leak "undefined" into the haystack when shortcut is absent', () => {
        const command: CommandPaletteModalCommand = {
          id: 'no-shortcut',
          label: 'Go to Kubernetes',
          description: '/k8s',
          keywords: ['k8s'],
          action: () => {},
          // shortcut intentionally omitted
        };
        const result = filterCommandPaletteCommands([command], 'undefined');
        expect(result).toStrictEqual([]);
      });
    });

    describe('`...(command.keywords ?? [])` arm', () => {
      it('matches a query that only appears in a keyword when keywords are present', () => {
        const command = makeCommand({
          label: 'Alpha',
          description: 'beta',
          shortcut: 'g g',
          keywords: ['gamma', 'delta', 'unique-kw-token'],
        });
        const result = filterCommandPaletteCommands([command], 'unique-kw-token');
        expect(result).toStrictEqual([command]);
      });

      it('does NOT throw and does NOT match "undefined" when keywords are absent', () => {
        // If the `?? []` arm were broken, spreading `undefined` would throw at
        // runtime; if it coerced wrongly, 'undefined' would match. Pin both.
        const command: CommandPaletteModalCommand = {
          id: 'no-keywords',
          label: 'Go to TrueNAS',
          description: '/truenas',
          shortcut: 'g n',
          action: () => {},
          // keywords intentionally omitted
        };
        const result = filterCommandPaletteCommands([command], 'undefined');
        expect(result).toStrictEqual([]);
      });

      it('still matches on the label when keywords are absent (proves spread-of-empty is harmless)', () => {
        const command: CommandPaletteModalCommand = {
          id: 'no-keywords-2',
          label: 'Go to TrueNAS',
          description: '/truenas',
          shortcut: 'g n',
          action: () => {},
        };
        const result = filterCommandPaletteCommands([command], 'truenas');
        expect(result).toStrictEqual([command]);
      });
    });

    describe('match vs no-match arms of `haystack.includes`', () => {
      it('returns an empty array when the query matches no command', () => {
        const commands = [
          makeCommand({ id: 'a' }),
          makeCommand({ id: 'b', label: 'Go to Docker' }),
        ];
        const result = filterCommandPaletteCommands(commands, 'zzz-nope-zzz');
        expect(result).toStrictEqual([]);
      });

      it('selects only the matching commands from a mixed input (filter subset arm)', () => {
        const proxmox = makeCommand({ id: 'prox' });
        const docker = makeCommand({ id: 'docker', label: 'Go to Docker' });
        const k8s = makeCommand({
          id: 'k8s',
          label: 'Go to Kubernetes',
          description: '/k8s',
          shortcut: 'g k',
          keywords: ['k8s'],
        });
        const result = filterCommandPaletteCommands(
          [proxmox, docker, k8s],
          'docker',
        );
        expect(result).toStrictEqual([docker]);
      });

      it('preserves input order when multiple commands match', () => {
        const a = makeCommand({ id: 'a', keywords: ['shared', 'alpha'] });
        const b = makeCommand({ id: 'b', keywords: ['shared', 'beta'] });
        const c = makeCommand({ id: 'c', keywords: ['unrelated'] });
        const result = filterCommandPaletteCommands([a, b, c], 'shared');
        expect(result).toStrictEqual([a, b]);
      });

      it('matches on the label substring', () => {
        const command = makeCommand({ label: 'Go to Settings' });
        const result = filterCommandPaletteCommands([command], 'settings');
        expect(result).toStrictEqual([command]);
      });
    });

    describe('query normalization (lowercase / trim / whitespace collapse)', () => {
      it('matches case-insensitively (uppercase query vs lowercase haystack)', () => {
        const command = makeCommand({ label: 'Go to Proxmox' });
        // Query 'PROXMOX' -> normalize -> 'proxmox' -> matches label.
        const result = filterCommandPaletteCommands([command], 'PROXMOX');
        expect(result).toStrictEqual([command]);
      });

      it('collapses internal whitespace in the query before matching', () => {
        const command = makeCommand({
          label: 'Go to Proxmox',
          description: '/proxmox',
          shortcut: 'g p',
          keywords: ['proxmox'],
        });
        // 'Go To' normalizes to 'goto'; the label haystack also collapses to
        // 'gotopromox...' so 'goto' is a substring of the collapsed haystack.
        const result = filterCommandPaletteCommands([command], 'Go To');
        expect(result).toStrictEqual([command]);
      });

      it('trims leading/trailing whitespace from the query before matching', () => {
        const command = makeCommand({ label: 'Go to Patrol' });
        const result = filterCommandPaletteCommands([command], '  patrol  ');
        expect(result).toStrictEqual([command]);
      });
    });

    describe('haystack whitespace collapse (`.join(" ").replace(/\\s+/g, "")` arm)', () => {
      it('matches a query whose letters span the label after the label internal spaces are removed', () => {
        // Label 'Go to Machines' -> haystack join -> 'go to machines...' ->
        // whitespace stripped -> 'gotomachines...'. Query 'gotomachines' hits.
        const command: CommandPaletteModalCommand = {
          id: 'machines',
          label: 'Go to Machines',
          description: '/standalone',
          shortcut: 'g s',
          keywords: ['machines'],
          action: () => {},
        };
        const result = filterCommandPaletteCommands([command], 'gotomachines');
        expect(result).toStrictEqual([command]);
      });

      it('matches a multi-word keyword after internal whitespace collapse', () => {
        // Keyword 'needs attention' contains a space; in the haystack it is
        // joined then stripped to 'needsattention'.
        const command: CommandPaletteModalCommand = {
          id: 'patrol',
          label: 'Go to Patrol',
          description: '/patrol',
          shortcut: 'g r',
          keywords: ['needs attention', 'patrol'],
          action: () => {},
        };
        // Query 'needsattention' only matches because the haystack strips the
        // internal space inside the keyword.
        const result = filterCommandPaletteCommands([command], 'needsattention');
        expect(result).toStrictEqual([command]);
      });
    });

    describe('combined: all optional fields populated on every command', () => {
      it('uses the real values of all present fields (present-arms of every ??)', () => {
        const commands: CommandPaletteModalCommand[] = [
          {
            id: 'full-1',
            label: 'Go to vSphere',
            description: '/vmware',
            shortcut: 'g v',
            keywords: ['vmware', 'vsphere', 'networks'],
            action: () => {},
          },
          {
            id: 'full-2',
            label: 'Go to Alerts',
            description: '/alerts',
            shortcut: 'g a',
            keywords: ['alarms', 'notifications'],
            action: () => {},
          },
        ];
        // Match against full-2 via its description-only token.
        const result = filterCommandPaletteCommands(commands, '/alerts');
        expect(result).toStrictEqual([commands[1]]);
      });
    });
  });
});
