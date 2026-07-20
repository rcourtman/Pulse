import { describe, expect, it } from 'vitest';
import type { PlatformNavigationVisibility } from '@/features/platformNavigation/platformNavigationModel';
import {
  buildCommandPaletteCommands,
  filterCommandPaletteCommands,
  normalizeCommandPaletteQuery,
  type CommandPaletteAssistantActions,
  type CommandPaletteCommandPaths,
  type CommandPaletteModalCommand,
} from '@/components/shared/commandPaletteModel';

// Branch-coverage suite for the three target exports of commandPaletteModel.ts:
//
//   1. buildCommandPaletteCommands  — COMPLETELY uncovered by the sibling
//      suites (branchcov0712 / branchcov0712c only test filterCommandPaletteCommands).
//      Every branch is exercised here: the `if (options.assistantActions)`
//      truthy/falsy arms, the `assistantOpenPresentation ?? {default}` LEFT vs
//      RIGHT arm, each of the six `primaryPlatformNavigationIsVisible(...)`
//      per-platform visible/not-visible arms, the always-pushed tail commands,
//      and the wiring of every `action` closure (platform actions call
//      `navigate(path)` with the correct path; assistant actions call the right
//      assistantActions method).
//   2. normalizeCommandPaletteQuery — only exercised INDIRECTLY via the filter
//      in the sibling suites. Here it is tested directly across every step of
//      its `toLowerCase().trim().replace(/\s+/g, '')` pipeline and every
//      whitespace class (\s = space, tab, newline, CR, FF, VT).
//   3. filterCommandPaletteCommands — heavily covered already; this file adds
//      ONLY complementary input-class angles the siblings do not exercise:
//      regex-metacharacter queries (treated literally by `.includes`), a query
//      equal to the entire normalized haystack (full-match boundary), a query
//      strictly longer than the haystack (no-match boundary), numeric-only
//      queries, and a single special-char query that matches many descriptions.
//
// Every assertion is a concrete equality / strict-equality check against the
// real return shape — no tautologies, no truthiness-only checks, no mock
// echoes. Closure-based spies (rather than vi.fn) match the import style of the
// sibling commandPaletteModel test suites.

// --- fixtures ----------------------------------------------------------------

function makePaths(
  overrides: Partial<CommandPaletteCommandPaths> = {},
): CommandPaletteCommandPaths {
  return {
    standalonePath: '/standalone',
    proxmoxPath: '/proxmox',
    dockerPath: '/docker',
    kubernetesPath: '/kubernetes',
    kubernetesWorkloadsPath: '/kubernetes/workloads',
    trueNasPath: '/truenas',
    vmwarePath: '/vmware',
    vmwareNetworksPath: '/vmware/networks',
    ...overrides,
  };
}

function makeVisibility(
  overrides: Partial<PlatformNavigationVisibility> = {},
): PlatformNavigationVisibility {
  return {
    proxmox: false,
    docker: false,
    kubernetes: false,
    truenas: false,
    vmware: false,
    standalone: false,
    ...overrides,
  };
}

// Records every path handed to `navigate`. The build function must NOT call
// navigate itself (actions are lazy closures); only invoking an action should
// push to `calls`, and with the exact path string for that command.
function makeNavigateSpy() {
  const calls: string[] = [];
  const navigate = (path: string): void => {
    calls.push(path);
  };
  return { navigate, calls };
}

// Builds a fully-populated CommandPaletteAssistantActions where every method is
// independently spy-counted, so we can assert exactly which method a given
// assistant command's action fires.
function makeAssistantActions(): {
  actions: CommandPaletteAssistantActions;
  spies: Record<keyof CommandPaletteAssistantActions, number>;
} {
  const spies: Record<keyof CommandPaletteAssistantActions, number> = {
    open: 0,
    help: 0,
    newSession: 0,
    sessions: 0,
    models: 0,
    providers: 0,
    status: 0,
    undo: 0,
    redo: 0,
  };
  const actions: CommandPaletteAssistantActions = {
    open: () => {
      spies.open += 1;
    },
    help: () => {
      spies.help += 1;
    },
    newSession: () => {
      spies.newSession += 1;
    },
    sessions: () => {
      spies.sessions += 1;
    },
    models: () => {
      spies.models += 1;
    },
    providers: () => {
      spies.providers += 1;
    },
    status: () => {
      spies.status += 1;
    },
    undo: () => {
      spies.undo += 1;
    },
    redo: () => {
      spies.redo += 1;
    },
  };
  return { actions, spies };
}

// ===========================================================================
// 1. buildCommandPaletteCommands
// ===========================================================================

describe('commandPaletteModel.branchcov0713', () => {
  describe('buildCommandPaletteCommands', () => {
    describe('`if (options.assistantActions)` — falsy arm (no assistant block)', () => {
      it('omits every assistant-* command when assistantActions is absent', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
        });
        const ids = commands.map((c) => c.id);
        expect(ids.filter((id) => id.startsWith('assistant-'))).toStrictEqual([]);
      });

      it('returns exactly the three always-pushed tail commands when nothing is visible and no assistant', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
        });
        expect(commands.map((c) => c.id)).toStrictEqual([
          'nav-alerts',
          'nav-patrol',
          'nav-settings',
        ]);
      });
    });

    describe('`if (options.assistantActions)` — truthy arm (nine assistant commands)', () => {
      it('prepends all nine assistant commands in the exact fixed order', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        // First nine entries are the assistant block, in source order.
        expect(commands.slice(0, 9).map((c) => c.id)).toStrictEqual([
          'assistant-open',
          'assistant-help',
          'assistant-new-session',
          'assistant-switch-session',
          'assistant-switch-model',
          'assistant-provider-settings',
          'assistant-status',
          'assistant-undo-last-turn',
          'assistant-redo-last-turn',
        ]);
      });

      it('emits nine assistant + three tail = twelve commands when no platform is visible', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        expect(commands).toHaveLength(12);
      });

      it('pins the exact static labels/descriptions of the eight non-open assistant commands', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        const byId = new Map(commands.map((c) => [c.id, c]));
        expect(byId.get('assistant-help')?.label).toBe('Show Assistant commands');
        expect(byId.get('assistant-help')?.description).toBe('/help');
        expect(byId.get('assistant-new-session')?.label).toBe('New Assistant session');
        expect(byId.get('assistant-new-session')?.description).toBe('/new');
        expect(byId.get('assistant-switch-session')?.label).toBe('Switch Assistant session');
        expect(byId.get('assistant-switch-session')?.description).toBe('/sessions');
        expect(byId.get('assistant-switch-model')?.label).toBe('Switch Assistant model');
        expect(byId.get('assistant-switch-model')?.description).toBe('/models');
        expect(byId.get('assistant-provider-settings')?.label).toBe(
          'Open Assistant provider settings',
        );
        expect(byId.get('assistant-provider-settings')?.description).toBe('/providers');
        expect(byId.get('assistant-status')?.label).toBe('Check Assistant status');
        expect(byId.get('assistant-status')?.description).toBe('/status');
        expect(byId.get('assistant-undo-last-turn')?.label).toBe('Undo last Assistant turn');
        expect(byId.get('assistant-undo-last-turn')?.description).toBe('/undo');
        expect(byId.get('assistant-redo-last-turn')?.label).toBe('Redo last Assistant turn');
        expect(byId.get('assistant-redo-last-turn')?.description).toBe('/redo');
      });
    });

    describe('`assistantOpenPresentation ?? {default}` — RIGHT (absent) arm', () => {
      it('falls back to the default label and description when presentation is omitted', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
          // assistantOpenPresentation intentionally omitted
        });
        const open = commands.find((c) => c.id === 'assistant-open');
        expect(open?.label).toBe('Ask about this view');
        expect(open?.description).toBe('Use the current Pulse view as context');
      });
    });

    describe('`assistantOpenPresentation ?? {default}` — LEFT (present) arm', () => {
      it('uses the caller-provided label and description verbatim', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
          assistantOpenPresentation: {
            label: 'Ask about the dashboard',
            description: 'Feed the current dashboard to the assistant',
          },
        });
        const open = commands.find((c) => c.id === 'assistant-open');
        expect(open?.label).toBe('Ask about the dashboard');
        expect(open?.description).toBe('Feed the current dashboard to the assistant');
      });

      it('does NOT contaminate the eight other assistant commands with the custom presentation', () => {
        // Only assistant-open consumes the presentation; the rest keep static copy.
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
          assistantOpenPresentation: {
            label: 'Unique Presentation Label',
            description: 'unique-presentation-desc',
          },
        });
        const others = commands.filter(
          (c) => c.id.startsWith('assistant-') && c.id !== 'assistant-open',
        );
        expect(others).toHaveLength(8);
        for (const cmd of others) {
          expect(cmd.label).not.toBe('Unique Presentation Label');
          expect(cmd.description).not.toBe('unique-presentation-desc');
        }
      });
    });

    describe('per-platform `primaryPlatformNavigationIsVisible` — not-visible arm (every platform)', () => {
      it('emits zero nav-* platform commands (only the always-pushed tail) when all flags are false', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
        });
        const navIds = commands
          .map((c) => c.id)
          .filter(
            (id) =>
              id.startsWith('nav-') && !['nav-alerts', 'nav-patrol', 'nav-settings'].includes(id),
          );
        expect(navIds).toStrictEqual([]);
      });
    });

    describe('per-platform visible arm — selective single-platform visibility', () => {
      it('emits only nav-proxmox (of the platform commands) when only proxmox is visible', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({ proxmox: true }),
          navigate,
        });
        const platformIds = commands
          .map((c) => c.id)
          .filter(
            (id) =>
              id.startsWith('nav-') && !['nav-alerts', 'nav-patrol', 'nav-settings'].includes(id),
          );
        expect(platformIds).toStrictEqual(['nav-proxmox']);
      });

      it('emits only nav-docker when only docker is visible', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({ docker: true }),
          navigate,
        });
        const platformIds = commands
          .map((c) => c.id)
          .filter(
            (id) =>
              id.startsWith('nav-') && !['nav-alerts', 'nav-patrol', 'nav-settings'].includes(id),
          );
        expect(platformIds).toStrictEqual(['nav-docker']);
      });

      it('emits BOTH nav-kubernetes and nav-kubernetes-workloads when only kubernetes is visible', () => {
        // The kubernetes block pushes two commands inside one `if`.
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({ kubernetes: true }),
          navigate,
        });
        const platformIds = commands
          .map((c) => c.id)
          .filter(
            (id) =>
              id.startsWith('nav-') && !['nav-alerts', 'nav-patrol', 'nav-settings'].includes(id),
          );
        expect(platformIds).toStrictEqual(['nav-kubernetes', 'nav-kubernetes-workloads']);
      });

      it('emits only nav-truenas when only truenas is visible', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({ truenas: true }),
          navigate,
        });
        const platformIds = commands
          .map((c) => c.id)
          .filter(
            (id) =>
              id.startsWith('nav-') && !['nav-alerts', 'nav-patrol', 'nav-settings'].includes(id),
          );
        expect(platformIds).toStrictEqual(['nav-truenas']);
      });

      it('emits BOTH nav-vmware and nav-vmware-networks when only vmware is visible', () => {
        // The vmware block pushes two commands inside one `if`.
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({ vmware: true }),
          navigate,
        });
        const platformIds = commands
          .map((c) => c.id)
          .filter(
            (id) =>
              id.startsWith('nav-') && !['nav-alerts', 'nav-patrol', 'nav-settings'].includes(id),
          );
        expect(platformIds).toStrictEqual(['nav-vmware', 'nav-vmware-networks']);
      });

      it('emits only nav-standalone when only standalone is visible', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({ standalone: true }),
          navigate,
        });
        const platformIds = commands
          .map((c) => c.id)
          .filter(
            (id) =>
              id.startsWith('nav-') && !['nav-alerts', 'nav-patrol', 'nav-settings'].includes(id),
          );
        expect(platformIds).toStrictEqual(['nav-standalone']);
      });
    });

    describe('all platforms visible at once', () => {
      it('emits all eight platform commands in source order, followed by the three tail commands', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({
            proxmox: true,
            docker: true,
            kubernetes: true,
            truenas: true,
            vmware: true,
            standalone: true,
          }),
          navigate,
        });
        expect(commands.map((c) => c.id)).toStrictEqual([
          'nav-proxmox',
          'nav-docker',
          'nav-kubernetes',
          'nav-kubernetes-workloads',
          'nav-truenas',
          'nav-vmware',
          'nav-vmware-networks',
          'nav-standalone',
          'nav-alerts',
          'nav-patrol',
          'nav-settings',
        ]);
        // 8 platform + 3 tail = 11.
        expect(commands).toHaveLength(11);
      });
    });

    describe('platform command `description` is sourced from `options.paths.*`', () => {
      it('flows each custom path string into the corresponding command description', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({
            proxmoxPath: '/custom/pve',
            dockerPath: '/custom/docker',
            kubernetesPath: '/custom/k8s',
            kubernetesWorkloadsPath: '/custom/k8s/wl',
            trueNasPath: '/custom/tn',
            vmwarePath: '/custom/vm',
            vmwareNetworksPath: '/custom/vm/net',
            standalonePath: '/custom/standalone',
          }),
          platformVisibility: makeVisibility({
            proxmox: true,
            docker: true,
            kubernetes: true,
            truenas: true,
            vmware: true,
            standalone: true,
          }),
          navigate,
        });
        const byId = new Map(commands.map((c) => [c.id, c]));
        expect(byId.get('nav-proxmox')?.description).toBe('/custom/pve');
        expect(byId.get('nav-docker')?.description).toBe('/custom/docker');
        expect(byId.get('nav-kubernetes')?.description).toBe('/custom/k8s');
        expect(byId.get('nav-kubernetes-workloads')?.description).toBe('/custom/k8s/wl');
        expect(byId.get('nav-truenas')?.description).toBe('/custom/tn');
        expect(byId.get('nav-vmware')?.description).toBe('/custom/vm');
        expect(byId.get('nav-vmware-networks')?.description).toBe('/custom/vm/net');
        expect(byId.get('nav-standalone')?.description).toBe('/custom/standalone');
      });

      it('uses the hardcoded descriptions for the always-pushed tail commands regardless of paths', () => {
        // alerts/patrol/settings descriptions are literals, not from options.paths.
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
        });
        const byId = new Map(commands.map((c) => [c.id, c]));
        expect(byId.get('nav-alerts')?.description).toBe('/alerts');
        expect(byId.get('nav-patrol')?.description).toBe('/patrol');
        expect(byId.get('nav-settings')?.description).toBe('/settings');
      });
    });

    describe('command shortcuts (only platform + tail commands carry them)', () => {
      it('assigns the documented shortcut to each platform/tail command and none to assistant commands', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({
            proxmox: true,
            docker: true,
            kubernetes: true,
            truenas: true,
            vmware: true,
            standalone: true,
          }),
          navigate,
          assistantActions: actions,
        });
        const byId = new Map(commands.map((c) => [c.id, c]));
        expect(byId.get('nav-proxmox')?.shortcut).toBe('g p');
        expect(byId.get('nav-docker')?.shortcut).toBe('g d');
        expect(byId.get('nav-kubernetes')?.shortcut).toBe('g k');
        expect(byId.get('nav-kubernetes-workloads')?.shortcut).toBeUndefined();
        expect(byId.get('nav-truenas')?.shortcut).toBe('g n');
        expect(byId.get('nav-vmware')?.shortcut).toBe('g v');
        expect(byId.get('nav-vmware-networks')?.shortcut).toBeUndefined();
        expect(byId.get('nav-standalone')?.shortcut).toBe('g s');
        expect(byId.get('nav-alerts')?.shortcut).toBe('g a');
        expect(byId.get('nav-patrol')?.shortcut).toBe('g r');
        expect(byId.get('nav-settings')?.shortcut).toBe('g t');
        // Assistant commands deliberately have no shortcut property.
        for (const id of [
          'assistant-open',
          'assistant-help',
          'assistant-new-session',
          'assistant-redo-last-turn',
        ]) {
          expect(byId.get(id)?.shortcut).toBeUndefined();
        }
      });
    });

    describe('action closure wiring — navigate is NOT called during build', () => {
      it('leaves the navigate spy untouched after building (actions are lazy)', () => {
        const { navigate, calls } = makeNavigateSpy();
        buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({
            proxmox: true,
            docker: true,
            kubernetes: true,
            truenas: true,
            vmware: true,
            standalone: true,
          }),
          navigate,
        });
        expect(calls).toStrictEqual([]);
      });
    });

    describe('action closure wiring — platform actions invoke navigate with the exact path', () => {
      it('fires navigate with the proxmox path when nav-proxmox action runs', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({ proxmoxPath: '/pve/path' }),
          platformVisibility: makeVisibility({ proxmox: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-proxmox')!.action();
        expect(calls).toStrictEqual(['/pve/path']);
      });

      it('fires navigate with the docker path when nav-docker action runs', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({ dockerPath: '/docker/path' }),
          platformVisibility: makeVisibility({ docker: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-docker')!.action();
        expect(calls).toStrictEqual(['/docker/path']);
      });

      it('fires navigate with the kubernetes path for nav-kubernetes', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({ kubernetesPath: '/k8s/path' }),
          platformVisibility: makeVisibility({ kubernetes: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-kubernetes')!.action();
        expect(calls).toStrictEqual(['/k8s/path']);
      });

      it('fires navigate with the workloads path for nav-kubernetes-workloads (distinct from kubernetes root)', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({
            kubernetesPath: '/k8s',
            kubernetesWorkloadsPath: '/k8s/workloads',
          }),
          platformVisibility: makeVisibility({ kubernetes: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-kubernetes-workloads')!.action();
        expect(calls).toStrictEqual(['/k8s/workloads']);
      });

      it('fires navigate with the truenas path when nav-truenas action runs', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({ trueNasPath: '/tn/path' }),
          platformVisibility: makeVisibility({ truenas: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-truenas')!.action();
        expect(calls).toStrictEqual(['/tn/path']);
      });

      it('fires navigate with the vmware path for nav-vmware', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({ vmwarePath: '/vm/path' }),
          platformVisibility: makeVisibility({ vmware: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-vmware')!.action();
        expect(calls).toStrictEqual(['/vm/path']);
      });

      it('fires navigate with the vmware networks path for nav-vmware-networks (distinct from vmware root)', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({ vmwarePath: '/vm', vmwareNetworksPath: '/vm/networks' }),
          platformVisibility: makeVisibility({ vmware: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-vmware-networks')!.action();
        expect(calls).toStrictEqual(['/vm/networks']);
      });

      it('fires navigate with the standalone path when nav-standalone action runs', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths({ standalonePath: '/standalone/path' }),
          platformVisibility: makeVisibility({ standalone: true }),
          navigate,
        });
        commands.find((c) => c.id === 'nav-standalone')!.action();
        expect(calls).toStrictEqual(['/standalone/path']);
      });
    });

    describe('action closure wiring — always-pushed tail actions', () => {
      it('fires navigate with /alerts, /patrol, /settings respectively', () => {
        const { navigate, calls } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
        });
        const byId = new Map(commands.map((c) => [c.id, c]));
        byId.get('nav-alerts')!.action();
        byId.get('nav-patrol')!.action();
        byId.get('nav-settings')!.action();
        expect(calls).toStrictEqual(['/alerts', '/patrol', '/settings']);
      });
    });

    describe('action closure wiring — assistant actions fire the correct method exactly once', () => {
      it('routes assistant-open -> assistantActions.open', () => {
        const { navigate } = makeNavigateSpy();
        const { actions, spies } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        commands.find((c) => c.id === 'assistant-open')!.action();
        expect(spies.open).toBe(1);
        // No other assistant method was invoked.
        expect(spies.help).toBe(0);
        expect(spies.newSession).toBe(0);
      });

      it('routes assistant-help -> assistantActions.help', () => {
        const { navigate } = makeNavigateSpy();
        const { actions, spies } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        commands.find((c) => c.id === 'assistant-help')!.action();
        expect(spies.help).toBe(1);
        expect(spies.open).toBe(0);
      });

      it('routes assistant-new-session -> newSession, assistant-switch-session -> sessions', () => {
        const { navigate } = makeNavigateSpy();
        const { actions, spies } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        commands.find((c) => c.id === 'assistant-new-session')!.action();
        commands.find((c) => c.id === 'assistant-switch-session')!.action();
        expect(spies.newSession).toBe(1);
        expect(spies.sessions).toBe(1);
      });

      it('routes assistant-switch-model -> models and assistant-provider-settings -> providers', () => {
        const { navigate } = makeNavigateSpy();
        const { actions, spies } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        commands.find((c) => c.id === 'assistant-switch-model')!.action();
        commands.find((c) => c.id === 'assistant-provider-settings')!.action();
        expect(spies.models).toBe(1);
        expect(spies.providers).toBe(1);
      });

      it('routes assistant-status -> status, assistant-undo-last-turn -> undo, assistant-redo-last-turn -> redo', () => {
        const { navigate } = makeNavigateSpy();
        const { actions, spies } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        commands.find((c) => c.id === 'assistant-status')!.action();
        commands.find((c) => c.id === 'assistant-undo-last-turn')!.action();
        commands.find((c) => c.id === 'assistant-redo-last-turn')!.action();
        expect(spies.status).toBe(1);
        expect(spies.undo).toBe(1);
        expect(spies.redo).toBe(1);
      });

      it('does NOT call navigate when an assistant action runs (assistant actions are independent of routing)', () => {
        const { navigate, calls } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        commands.find((c) => c.id === 'assistant-open')!.action();
        commands.find((c) => c.id === 'assistant-help')!.action();
        expect(calls).toStrictEqual([]);
      });
    });

    describe('keywords arrays are populated (spot-check representative commands)', () => {
      it('attaches the exact proxmox keyword set to nav-proxmox', () => {
        const { navigate } = makeNavigateSpy();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({ proxmox: true }),
          navigate,
        });
        expect(commands.find((c) => c.id === 'nav-proxmox')?.keywords).toStrictEqual([
          'proxmox',
          'pve',
          'pbs',
          'pmg',
          'mail',
          'backups',
          'ceph',
          'vm',
          'lxc',
        ]);
      });

      it('attaches the seven-term keyword set to assistant-open', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility(),
          navigate,
          assistantActions: actions,
        });
        expect(commands.find((c) => c.id === 'assistant-open')?.keywords).toStrictEqual([
          'assistant',
          'ai',
          'chat',
          'ask',
          'pulse',
          'view',
          'context',
        ]);
      });
    });

    describe('combined realistic build (assistant + all platforms visible)', () => {
      it('produces 9 assistant + 8 platform + 3 tail = 20 commands with unique ids', () => {
        const { navigate } = makeNavigateSpy();
        const { actions } = makeAssistantActions();
        const commands = buildCommandPaletteCommands({
          paths: makePaths(),
          platformVisibility: makeVisibility({
            proxmox: true,
            docker: true,
            kubernetes: true,
            truenas: true,
            vmware: true,
            standalone: true,
          }),
          navigate,
          assistantActions: actions,
          assistantOpenPresentation: { label: 'L', description: 'D' },
        });
        expect(commands).toHaveLength(20);
        const ids = commands.map((c) => c.id);
        expect(new Set(ids).size).toBe(ids.length);
        // First command is the customized assistant-open.
        expect(commands[0]).toMatchObject({ id: 'assistant-open', label: 'L', description: 'D' });
        // Last three are the always-pushed tail.
        expect(commands.slice(-3).map((c) => c.id)).toStrictEqual([
          'nav-alerts',
          'nav-patrol',
          'nav-settings',
        ]);
      });
    });
  });

  // ===========================================================================
  // 2. normalizeCommandPaletteQuery
  // ===========================================================================

  describe('normalizeCommandPaletteQuery', () => {
    describe('empty / whitespace-only inputs (every arm of \\s collapses to empty)', () => {
      it('returns "" for the empty string', () => {
        expect(normalizeCommandPaletteQuery('')).toBe('');
      });

      it('returns "" for a single space', () => {
        expect(normalizeCommandPaletteQuery(' ')).toBe('');
      });

      it('returns "" for many spaces', () => {
        expect(normalizeCommandPaletteQuery('     ')).toBe('');
      });

      it('returns "" for a single tab', () => {
        expect(normalizeCommandPaletteQuery('\t')).toBe('');
      });

      it('returns "" for a single newline', () => {
        expect(normalizeCommandPaletteQuery('\n')).toBe('');
      });

      it('returns "" for a carriage return', () => {
        expect(normalizeCommandPaletteQuery('\r')).toBe('');
      });

      it('returns "" for a form feed / vertical tab (exotic \\s members)', () => {
        expect(normalizeCommandPaletteQuery('\f')).toBe('');
        expect(normalizeCommandPaletteQuery('\v')).toBe('');
      });

      it('returns "" for a mix of all whitespace kinds surrounding real text', () => {
        // trim() removes outer; replace removes inner -> empty remains.
        expect(normalizeCommandPaletteQuery(' \t\n\r\f\v ')).toBe('');
      });
    });

    describe('toLowerCase arm', () => {
      it('leaves an already-lowercase, whitespace-free string unchanged', () => {
        expect(normalizeCommandPaletteQuery('proxmox')).toBe('proxmox');
      });

      it('lowercases an all-uppercase string', () => {
        expect(normalizeCommandPaletteQuery('PROXMOX')).toBe('proxmox');
      });

      it('lowercases a mixed-case string', () => {
        expect(normalizeCommandPaletteQuery('GoToSettings')).toBe('gotosettings');
      });

      it('preserves characters with no case (digits)', () => {
        expect(normalizeCommandPaletteQuery('ROUTE42')).toBe('route42');
      });

      it('lowercases non-ASCII latin-1 letters', () => {
        expect(normalizeCommandPaletteQuery('ÉÀÇ')).toBe('éàç');
      });
    });

    describe('trim arm (leading/trailing whitespace)', () => {
      it('removes leading whitespace', () => {
        expect(normalizeCommandPaletteQuery('  hello')).toBe('hello');
      });

      it('removes trailing whitespace', () => {
        expect(normalizeCommandPaletteQuery('hello  ')).toBe('hello');
      });

      it('removes leading and trailing whitespace, then the replace step also strips the inner space', () => {
        // '   go to   ' -> trim -> 'go to' -> replace \s+ -> 'goto' (inner space gone too).
        expect(normalizeCommandPaletteQuery('   go to   ')).toBe('goto');
      });

      it('removes leading/trailing tabs and newlines (not just spaces)', () => {
        expect(normalizeCommandPaletteQuery('\t\nhello\n\t')).toBe('hello');
      });
    });

    describe('replace(/\\s+/g, "") arm — internal whitespace removal', () => {
      it('removes a single internal space', () => {
        expect(normalizeCommandPaletteQuery('go to')).toBe('goto');
      });

      it('collapses a run of multiple internal spaces to nothing (not to a single space)', () => {
        // \s+ matches the whole run in one go; replacement is "" -> gone entirely.
        expect(normalizeCommandPaletteQuery('go   to')).toBe('goto');
      });

      it('removes an internal tab', () => {
        expect(normalizeCommandPaletteQuery('go\tto')).toBe('goto');
      });

      it('removes an internal newline', () => {
        expect(normalizeCommandPaletteQuery('go\nto')).toBe('goto');
      });

      it('removes a mixed-kind internal whitespace run in one match', () => {
        expect(normalizeCommandPaletteQuery('go \t\n to')).toBe('goto');
      });

      it('removes every space in an all-spaces word', () => {
        expect(normalizeCommandPaletteQuery('g o t o')).toBe('goto');
      });
    });

    describe('preservation of non-whitespace content', () => {
      it('preserves punctuation (dots, slashes, dashes)', () => {
        expect(normalizeCommandPaletteQuery('/proxmox/pve')).toBe('/proxmox/pve');
      });

      it('preserves leading slash used by Pulse command descriptions', () => {
        expect(normalizeCommandPaletteQuery('  /Help  ')).toBe('/help');
      });

      it('preserves digits adjacent to letters', () => {
        expect(normalizeCommandPaletteQuery('k8s')).toBe('k8s');
      });
    });

    describe('full-pipeline composition (all three steps in one input)', () => {
      it('applies trim, lowercase, and whitespace-removal together', () => {
        // '  /Go To  Settings  ' -> trim -> '/Go To  Settings' -> lowercase
        // -> '/go to  settings' -> replace \s+ -> '/gotosettings'
        expect(normalizeCommandPaletteQuery('  /Go To  Settings  ')).toBe('/gotosettings');
      });

      it('keeps a no-op pipeline stable (already-normalized input is returned verbatim)', () => {
        expect(normalizeCommandPaletteQuery('/alerts')).toBe('/alerts');
      });
    });
  });

  // ===========================================================================
  // 3. filterCommandPaletteCommands — COMPLEMENTARY input-class coverage only.
  //    The sibling suites (branchcov0712 / branchcov0712c) already cover every
  //    code branch (early return, every `??` arm, match/no-match, order,
  //    case-folding, haystack collapse). This block adds ONLY novel input
  //    classes that exercise those same branches through angles not yet
  //    asserted, to guard against regressions in `.includes` semantics.
  // ===========================================================================

  describe('filterCommandPaletteCommands — complementary input-class coverage', () => {
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

    describe('regex metacharacters in the query are treated as LITERAL substrings', () => {
      // filterCommandPaletteCommands uses String.prototype.includes (not a
      // RegExp), so metacharacters must never act as wildcards. Pin that.
      it('does NOT treat ".*" as a match-all wildcard', () => {
        const commands = [
          makeCommand({ id: 'a', label: 'Alpha' }),
          makeCommand({ id: 'b', label: 'Beta' }),
        ];
        // No command literally contains the substring '.*' -> empty result.
        expect(filterCommandPaletteCommands(commands, '.*')).toStrictEqual([]);
      });

      it('does NOT treat "(.*)" as a regex group', () => {
        const command = makeCommand({ id: 'a', label: 'Alpha' });
        expect(filterCommandPaletteCommands([command], '(.*)')).toStrictEqual([]);
      });

      it('matches a command that LITERALLY contains a metacharacter substring', () => {
        // label 'a.b' contains the literal '.' -> query 'a.b' matches, but a
        // query of 'aXb' would not (proves it is not regex-based).
        const command = makeCommand({ id: 'dot', label: 'a.b' });
        expect(filterCommandPaletteCommands([command], 'a.b')).toStrictEqual([command]);
        expect(filterCommandPaletteCommands([command], 'aXb')).toStrictEqual([]);
      });

      it('matches a literal bracket/plus/pipe substring', () => {
        const command = makeCommand({
          id: 'meta',
          label: 'regex',
          description: '[a]+|b',
          shortcut: 'g g',
          keywords: ['x'],
        });
        expect(filterCommandPaletteCommands([command], '[a]+|b')).toStrictEqual([command]);
      });
    });

    describe('single-character queries that match many descriptions (the "/" case)', () => {
      it('returns every command whose haystack contains "/" for the query "/"', () => {
        // Most Pulse descriptions start with '/'; label/keyword usually do not.
        const a = makeCommand({ id: 'a', label: 'Alerts', description: '/alerts' });
        const b = makeCommand({ id: 'b', label: 'Patrol', description: '/patrol' });
        const c = makeCommand({
          id: 'c',
          label: 'NoSlash',
          description: 'plain',
          shortcut: 'g g',
          keywords: ['plain'],
        });
        const result = filterCommandPaletteCommands([a, b, c], '/');
        // After haystack collapse only a and b contain '/'.
        expect(result).toStrictEqual([a, b]);
      });
    });

    describe('numeric-only queries', () => {
      it('matches a command whose keyword is purely numeric', () => {
        const command = makeCommand({
          id: 'num',
          label: 'Box',
          description: '/box',
          shortcut: 'g g',
          keywords: ['42', 'box'],
        });
        expect(filterCommandPaletteCommands([command], '42')).toStrictEqual([command]);
      });

      it('matches a command whose label embeds digits', () => {
        const command = makeCommand({ id: 'k8s', label: 'Go to Kubernetes 8' });
        expect(filterCommandPaletteCommands([command], '8')).toStrictEqual([command]);
      });

      it('does not match when no field contains the digit', () => {
        const command = makeCommand({ id: 'no-num', label: 'Alpha', keywords: ['beta'] });
        expect(filterCommandPaletteCommands([command], '9')).toStrictEqual([]);
      });
    });

    describe('exact full-haystack boundary match', () => {
      it('matches when the normalized query equals the ENTIRE normalized haystack', () => {
        // label 'Go' + description '/px' + shortcut '' + keywords [] -> join ' '
        // -> 'Go /px ' -> lowercase -> 'go /px ' -> strip ws -> 'go/px'.
        // Query 'go/px' is exactly the whole haystack.
        const command: CommandPaletteModalCommand = {
          id: 'exact',
          label: 'Go',
          description: '/px',
          action: () => {},
        };
        expect(filterCommandPaletteCommands([command], 'go/px')).toStrictEqual([command]);
      });

      it('also matches a leading substring of that full haystack (substring arm)', () => {
        const command: CommandPaletteModalCommand = {
          id: 'exact2',
          label: 'Go',
          description: '/px',
          action: () => {},
        };
        expect(filterCommandPaletteCommands([command], 'go')).toStrictEqual([command]);
      });
    });

    describe('query strictly longer than the haystack (no-match boundary)', () => {
      it('returns [] when the query contains the whole haystack plus extra characters', () => {
        // Full normalized haystack is 'go/px'; query 'go/px/extra' is a strict
        // superset and therefore cannot be a substring of the haystack.
        const command: CommandPaletteModalCommand = {
          id: 'short',
          label: 'Go',
          description: '/px',
          action: () => {},
        };
        expect(filterCommandPaletteCommands([command], 'go/px/extra')).toStrictEqual([]);
      });

      it('returns [] when the haystack is the empty string (label-only absent) and query is non-empty', () => {
        // Malformed-but-typed fixture: label is the only non-empty field but we
        // blank it too, so the collapsed haystack is ''.
        const command = {
          id: 'empty-hay',
          label: '',
          action: () => {},
        } as unknown as CommandPaletteModalCommand;
        expect(filterCommandPaletteCommands([command], 'x')).toStrictEqual([]);
      });
    });
  });
});
