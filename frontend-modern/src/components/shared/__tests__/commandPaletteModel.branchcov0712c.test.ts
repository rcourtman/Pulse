import { describe, expect, it } from 'vitest';
import {
  filterCommandPaletteCommands,
  type CommandPaletteModalCommand,
} from '@/components/shared/commandPaletteModel';

// Complementary branch-coverage suite for `filterCommandPaletteCommands`.
// Each assertion is a concrete equality/strict-equality check against the real
// return shape (no truthiness-only checks). This file deliberately targets
// branches/angles that the sibling `branchcov0712` suite does not exercise:
//
//   A. Empty input array — drives BOTH the early-return arm (empty query ->
//      same [] reference) AND the `.filter` arm with zero elements (non-empty
//      query -> fresh empty array, NOT the same reference).
//   B. Present-but-EMPTY optional fields (`description: ''`, `shortcut: ''`,
//      `keywords: []`). The `??` operator returns its LEFT operand for every
//      non-null/undefined value, so an empty-string / empty-array value hits
//      the LEFT (present) arm of `??` with a falsy payload — a distinct branch
//      state from "present with content" and from "absent (undefined)".
//   C. All optionals absent at once (label-only command) — combines every
//      RIGHT (absent) arm of `??` in a single pass.
//   D. Cross-field join-boundary matching — substring spans the `join(' ')`
//      seam between two fields once whitespace is stripped.
//   E. Realistic multi-command subset selection + order preservation across a
//      mix of present/absent fields and case/whitespace variance.

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

describe('commandPaletteModel.branchcov0712c', () => {
  describe('filterCommandPaletteCommands', () => {
    describe('A. empty input array (early-return arm vs filter arm with 0 elements)', () => {
      it('returns the SAME empty array reference for an empty query (early-return arm)', () => {
        const commands: CommandPaletteModalCommand[] = [];
        // `!normalizedQuery` is true -> `return commands` (same reference).
        expect(filterCommandPaletteCommands(commands, '')).toBe(commands);
      });

      it('returns the SAME empty array reference for a whitespace-only query', () => {
        const commands: CommandPaletteModalCommand[] = [];
        // ' \t ' normalizes to '' -> falsy -> early return (same reference).
        expect(filterCommandPaletteCommands(commands, ' \t ')).toBe(commands);
      });

      it('returns a NEW empty array (different reference) for a non-empty query on [] (filter arm)', () => {
        const commands: CommandPaletteModalCommand[] = [];
        // `commands.filter(...)` always allocates a fresh array, even when it
        // stays empty, so the reference must differ from the input.
        const result = filterCommandPaletteCommands(commands, 'anything');
        expect(result).toStrictEqual([]);
        expect(result).not.toBe(commands);
      });
    });

    describe('B. present-but-empty optional fields (LEFT arm of ?? with falsy payload)', () => {
      it('treats `description: ""` as the LEFT arm of ?? and still matches on other fields', () => {
        // description is present but empty -> `description ?? ''` yields ''
        // (the LEFT operand), not via the RIGHT arm. Match must come from label.
        const command = makeCommand({
          id: 'empty-desc',
          label: 'Go to Docker',
          description: '',
          shortcut: 'g d',
          keywords: ['docker'],
        });
        const result = filterCommandPaletteCommands([command], 'docker');
        expect(result).toStrictEqual([command]);
      });

      it('does NOT match the literal "undefined" string when description is "" (LEFT arm intact)', () => {
        // A present empty string must never inject 'undefined' into the haystack.
        const command = makeCommand({
          id: 'empty-desc-2',
          label: 'Go to Docker',
          description: '',
          action: () => {},
        });
        const result = filterCommandPaletteCommands([command], 'undefined');
        expect(result).toStrictEqual([]);
      });

      it('treats `shortcut: ""` as the LEFT arm of ?? and matches on other fields', () => {
        const command = makeCommand({
          id: 'empty-shortcut',
          label: 'Go to Kubernetes',
          description: '/k8s',
          shortcut: '',
          keywords: ['k8s'],
        });
        const result = filterCommandPaletteCommands([command], 'kubernetes');
        expect(result).toStrictEqual([command]);
      });

      it('treats `keywords: []` as the LEFT arm of ?? (empty spread, no elements added)', () => {
        // keywords present but empty -> `keywords ?? []` yields [] (LEFT arm),
        // and the spread contributes nothing. Matching still works via label.
        const command = makeCommand({
          id: 'empty-keywords',
          label: 'Go to TrueNAS',
          description: '/truenas',
          shortcut: 'g n',
          keywords: [],
        });
        const result = filterCommandPaletteCommands([command], 'truenas');
        expect(result).toStrictEqual([command]);
      });

      it('matches nothing when ALL fields are present-but-empty except an unrelated label', () => {
        // description:'', shortcut:'', keywords:[] — every ?? takes its LEFT arm
        // with an empty payload, so only the label feeds the haystack.
        const command = makeCommand({
          id: 'all-empty-opt',
          label: 'Go to Settings',
          description: '',
          shortcut: '',
          keywords: [],
        });
        // A query that is NOT in the label must yield no matches.
        expect(filterCommandPaletteCommands([command], 'proxmox')).toStrictEqual([]);
        // But a label substring still matches.
        expect(filterCommandPaletteCommands([command], 'settings')).toStrictEqual([command]);
      });
    });

    describe('C. all optionals absent at once (RIGHT arm of every ?? simultaneously)', () => {
      it('builds the haystack from the label only and matches a label substring', () => {
        const command: CommandPaletteModalCommand = {
          id: 'label-only',
          label: 'Go to Alerts',
          action: () => {},
          // description, shortcut, keywords ALL omitted -> every ?? hits RIGHT arm.
        };
        const result = filterCommandPaletteCommands([command], 'alerts');
        expect(result).toStrictEqual([command]);
      });

      it('returns no match when the (label-only) haystack lacks the query', () => {
        const command: CommandPaletteModalCommand = {
          id: 'label-only-2',
          label: 'Go to Patrol',
          action: () => {},
        };
        const result = filterCommandPaletteCommands([command], 'docker');
        expect(result).toStrictEqual([]);
      });
    });

    describe('D. cross-field join-boundary matching', () => {
      it('matches a substring that spans the label/description seam after whitespace stripping', () => {
        // label 'Go' + description '/to' -> join(' ') -> 'Go /to' -> strip ws
        // -> 'go/to'. Query 'go/to' spans the join boundary.
        const command = makeCommand({
          id: 'seam-1',
          label: 'Go',
          description: '/to',
          shortcut: 'g g',
          keywords: ['unrelated'],
        });
        const result = filterCommandPaletteCommands([command], 'go/to');
        expect(result).toStrictEqual([command]);
      });

      it('matches a substring spanning the shortcut/keyword seam', () => {
        // shortcut 'g p' (-> 'gp' after strip) + keyword 've' -> join -> strip
        // -> '...gpve...'. Query 'gpve' crosses the shortcut|keyword boundary.
        const command = makeCommand({
          id: 'seam-2',
          label: 'Proxmox',
          description: '/px',
          shortcut: 'g p',
          keywords: ['ve', 'extra'],
        });
        const result = filterCommandPaletteCommands([command], 'gpve');
        expect(result).toStrictEqual([command]);
      });

      it('does NOT falsely match across a seam when the stripped substring is absent', () => {
        // Confirm the seam logic is exact: a near-miss query yields nothing.
        const command = makeCommand({
          id: 'seam-3',
          label: 'Alpha',
          description: 'beta',
          shortcut: 'g g',
          keywords: ['gamma'],
        });
        const result = filterCommandPaletteCommands([command], 'alphabetaz');
        expect(result).toStrictEqual([]);
      });
    });

    describe('E. realistic multi-command subset selection + order preservation', () => {
      it('selects a shortcut-omitted command (RIGHT arm of ??) from a mixed set', () => {
        const a = makeCommand({
          id: 'nav-proxmox',
          label: 'Go to Proxmox',
          description: '/proxmox',
          shortcut: 'g p',
          keywords: ['proxmox', 'pve'],
        });
        const b: CommandPaletteModalCommand = {
          id: 'nav-docker',
          label: 'Go to Docker',
          description: '/docker',
          // shortcut omitted (RIGHT arm of ??)
          keywords: ['docker', 'containers'],
          action: () => {},
        };
        const c: CommandPaletteModalCommand = {
          id: 'nav-truenas',
          label: 'Go to TrueNAS',
          // description, shortcut, keywords all omitted (all RIGHT arms)
          action: () => {},
        };

        // Query 'docker' matches ONLY b, whose `shortcut ?? ''` took the RIGHT
        // (absent) arm — proving omitted-field commands still filter correctly.
        const result = filterCommandPaletteCommands([a, b, c], 'docker');
        expect(result).toStrictEqual([b]);
      });

      it('selects a 2-element subset in input order across mixed field shapes', () => {
        const a = makeCommand({
          id: 'nav-proxmox',
          label: 'Go to Proxmox',
          description: '/proxmox',
          shortcut: 'g p',
          keywords: ['proxmox', 'pve'],
        });
        const b: CommandPaletteModalCommand = {
          id: 'nav-docker',
          label: 'Go to Docker',
          description: '/docker',
          keywords: ['docker', 'containers'],
          action: () => {},
        };
        const c: CommandPaletteModalCommand = {
          id: 'nav-truenas',
          label: 'Go to TrueNAS',
          action: () => {},
        };
        const d = makeCommand({
          id: 'nav-vmware',
          label: 'Go to vSphere',
          description: '/vmware',
          shortcut: 'g v',
          keywords: ['vmware', 'vsphere'],
        });

        // Query 'v' normalizes to 'v': matches a (via 'pve') and d (via
        // 'vsphere'/'vmware'/'g v'), but not b (docker/containers) nor c.
        const result = filterCommandPaletteCommands([a, b, c, d], 'v');
        expect(result).toStrictEqual([a, d]);
      });

      it('preserves order when every command matches', () => {
        const commands = [
          makeCommand({ id: 'first', keywords: ['shared'] }),
          makeCommand({ id: 'second', keywords: ['shared'] }),
          makeCommand({ id: 'third', keywords: ['shared'] }),
        ];
        const result = filterCommandPaletteCommands(commands, 'shared');
        expect(result.map((c) => c.id)).toStrictEqual(['first', 'second', 'third']);
        // Returned elements are the same object references, in order.
        expect(result).toStrictEqual(commands);
      });

      it('returns a fresh array (not the input reference) on the filter arm even when all match', () => {
        const commands = [makeCommand({ id: 'a' }), makeCommand({ id: 'b' })];
        const result = filterCommandPaletteCommands(commands, 'proxmox');
        // Same contents, different array object — confirms `.filter` allocation.
        expect(result).toStrictEqual(commands);
        expect(result).not.toBe(commands);
      });

      it('matches a single command out of many via a case-mixed query', () => {
        const commands = [
          makeCommand({ id: 'a', label: 'Go to Settings' }),
          makeCommand({ id: 'b', label: 'Go to Patrol' }),
          makeCommand({ id: 'c', label: 'Go to Alerts' }),
        ];
        // Mixed-case query normalizes to lowercase and matches only 'c'.
        const result = filterCommandPaletteCommands(commands, 'aLeRtS');
        expect(result.map((c) => c.id)).toStrictEqual(['c']);
      });

      it('matches via an internal-whitespace query token against a multi-word keyword', () => {
        const command = makeCommand({
          id: 'patrol',
          label: 'Go to Patrol',
          description: '/patrol',
          shortcut: 'g r',
          keywords: ['needs attention', 'patrol'],
        });
        // 'needs attention' in the keyword -> haystack strips the space ->
        // 'needsattention'. Query ' Needs   Attention ' normalizes (trim +
        // collapse) to 'needsattention' and matches.
        const result = filterCommandPaletteCommands([command], ' Needs   Attention ');
        expect(result).toStrictEqual([command]);
      });
    });
  });
});
