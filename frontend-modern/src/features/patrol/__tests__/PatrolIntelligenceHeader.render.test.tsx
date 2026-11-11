import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { PatrolAutonomyLevel } from '@/api/patrol';
import type { Resource } from '@/types/resource';
import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';
import {
  PATROL_CONTROL_ANCHOR,
  PATROL_CONTROL_PATH,
  PATROL_CONTROL_PATH_WITH_STARTER,
  PATROL_OPERATIONS_LOOP_ANCHOR,
} from '@/routing/resourceLinks';
import { PatrolIntelligenceHeader } from '../PatrolIntelligenceHeader';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

const resourcesMock = vi.hoisted(() => ({ value: [] as Resource[] }));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ resources: () => resourcesMock.value }),
}));

const agentHost = (name: string, commandsEnabled = false): Resource =>
  ({
    id: `agent:${name}`,
    type: 'agent',
    name,
    displayName: name,
    status: 'online',
    sources: ['agent'],
    agent: { agentId: name, stale: false, ...(commandsEnabled ? { commandsEnabled: true } : {}) },
  }) as Resource;

beforeEach(() => {
  resourcesMock.value = [];
});

const WATCH_ONLY_DETAIL =
  'Patrol checks infrastructure and reports issues only. It does not start fixes.';

interface HeaderStateOptions {
  autonomyLevel?: PatrolAutonomyLevel;
  autoFixLocked?: boolean | (() => boolean);
  setupOnly?: boolean;
  autopilotDialogOpen?: boolean;
  defaultModeApplied?: boolean;
}

const createState = (options: HeaderStateOptions = {}) => {
  const autonomyLevel = options.autonomyLevel ?? 'approval';
  const handleAutonomyChange = vi.fn();
  const state = {
    autopilotStatus: () => null,
    autonomyLevel: () => autonomyLevel,
    requestedAutonomyLevel: () => autonomyLevel,
    defaultModeApplied: () => options.defaultModeApplied ?? false,
    autoFixLocked: () => {
      const locked = options.autoFixLocked ?? false;
      return typeof locked === 'function' ? locked() : locked;
    },
    autoFixCapabilityBlock: () => undefined,
    licenseRuntimeIdentity: () => undefined,
    patrolRunHistory: { value: () => [] },
    patrolStatus: () => null,
    patrolReadiness: () => null,
    blockedCause: () => null,
    runtimeState: () => 'active',
    canTriggerPatrol: () => true,
    triggerPatrolDisabledReason: () => '',
    isTriggeringPatrol: () => false,
    manualRunRequested: () => false,
    patrolStream: { isStreaming: () => false },
    patrolEnabledLocal: () => true,
    isTogglingPatrol: () => false,
    isUpdatingAutonomy: () => false,
    shouldShowPatrolSetupOnly: () => options.setupOnly ?? false,
    autopilotDialogOpen: () => options.autopilotDialogOpen ?? false,
    setAutopilotDialogOpen: vi.fn(),
    acknowledgeAndActivateAutopilot: vi.fn(),
    revokeAutopilot: vi.fn(),
    handleRunPatrol: vi.fn(),
    handleTogglePatrol: vi.fn(),
    handleAutonomyChange,
  } as unknown as PatrolIntelligenceState;
  return { state, handleAutonomyChange };
};

const renderHeader = (state: PatrolIntelligenceState) =>
  render(() => (
    <Router>
      <Route path="*" component={() => <PatrolIntelligenceHeader state={state} />} />
    </Router>
  ));

const modeDisclosure = () => {
  const summary = screen.getByText('Mode and automation');
  const details = summary.closest('details');
  expect(details).not.toBeNull();
  expect(summary.tagName).toBe('SUMMARY');
  return details!;
};

// These pin the Patrol header facts that the api-contracts, frontend-primitives,
// and patrol-intelligence contracts state about the Patrol mode selector.
describe('PatrolIntelligenceHeader Patrol mode placement', () => {
  afterEach(cleanup);

  it('keeps the Patrol mode selector inside the collapsed Mode and automation disclosure', () => {
    const { state, handleAutonomyChange } = createState();
    renderHeader(state);

    const details = modeDisclosure();
    expect(details.open).toBe(false);
    expect(details.hasAttribute('open')).toBe(false);

    const selector = screen.getByRole('group', { name: 'Patrol mode' });
    expect(details.contains(selector)).toBe(true);
    expect(selector.closest('#patrol-control')).not.toBeNull();
    expect(
      within(selector)
        .getAllByRole('button')
        .map((button) => button.getAttribute('aria-label')),
    ).toEqual(['Watch only', 'Ask first', 'Safe auto-fix', 'Autopilot']);

    fireEvent.click(within(selector).getByRole('button', { name: 'Safe auto-fix' }));
    expect(handleAutonomyChange).toHaveBeenCalledWith('assisted');
  });

  it('offers one Patrol mode selector and routes configuration to Patrol settings', () => {
    const { state } = createState();
    renderHeader(state);

    expect(screen.getAllByRole('group', { name: 'Patrol mode' })).toHaveLength(1);
    expect(screen.getByRole('link', { name: 'Open Patrol settings' })).toHaveAttribute(
      'href',
      settingsTabPath('system-ai-patrol'),
    );
  });

  it('shows plan-locked installs one inline Patrol mode line instead of a selector', () => {
    const { state } = createState({ autonomyLevel: 'approval', autoFixLocked: true });
    renderHeader(state);

    expect(screen.queryByText('Mode and automation')).toBeNull();
    expect(screen.queryByRole('group', { name: 'Patrol mode' })).toBeNull();
    expect(document.querySelector('details')).toBeNull();

    const label = screen.getByText('Patrol mode');
    const line = label.parentElement!;
    expect(line.textContent).toContain(WATCH_ONLY_DETAIL);
    expect(line.closest('#patrol-control')).not.toBeNull();
  });

  it('keeps the Mode and automation disclosure while setup hides the run and settings controls', () => {
    const { state } = createState({ setupOnly: true });
    renderHeader(state);

    expect(screen.queryByText('Check now')).toBeNull();
    expect(screen.queryByRole('link', { name: 'Open Patrol settings' })).toBeNull();
    const details = modeDisclosure();
    expect(details.contains(screen.getByRole('group', { name: 'Patrol mode' }))).toBe(true);
  });

  it('opens no Patrol control dialog; the Autopilot acknowledgement is its only dialog', () => {
    const closed = createState();
    renderHeader(closed.state);
    expect(screen.queryAllByRole('dialog')).toHaveLength(0);
    cleanup();

    const opened = createState({ autopilotDialogOpen: true });
    renderHeader(opened.state);
    const dialogs = screen.getAllByRole('dialog');
    expect(dialogs).toHaveLength(1);
    expect(dialogs[0]).toHaveAccessibleName('Activate Autopilot');
  });
});

const renderHeaderAt = (url: string, state: PatrolIntelligenceState) => {
  window.history.replaceState(null, '', url);
  return renderHeader(state);
};

const navigateTo = (url: string) => {
  window.history.pushState(null, '', url);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

const nextFrame = () => new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

// Settings and licence Patrol mode actions link to #patrol-control, and the
// api-contracts "Patrol control route target" clause requires that anchor to
// land on a visible Patrol mode selector, not on its collapsed disclosure.
describe('PatrolIntelligenceHeader Patrol mode anchors', () => {
  let scrolled: Element[];

  beforeEach(() => {
    scrolled = [];
    HTMLElement.prototype.scrollIntoView = vi.fn(function (this: HTMLElement) {
      scrolled.push(this);
    });
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState(null, '', '/');
  });

  it.each([
    ['the Patrol mode entry points', PATROL_CONTROL_PATH],
    ['the licence starter handoff', PATROL_CONTROL_PATH_WITH_STARTER],
    ['the operations-loop compatibility anchor', `/patrol#${PATROL_OPERATIONS_LOOP_ANCHOR}`],
  ])('opens the Patrol mode selector and scrolls to it for %s', async (_entry, url) => {
    renderHeaderAt(url, createState().state);

    const details = modeDisclosure();
    expect(details.open).toBe(true);
    expect(details.contains(screen.getByRole('group', { name: 'Patrol mode' }))).toBe(true);
    await vi.waitFor(() => expect(scrolled).toEqual([details]));
    expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalledWith({ block: 'center' });
  });

  it('keeps the disclosure collapsed on a plain Patrol visit', async () => {
    renderHeaderAt('/patrol', createState().state);

    expect(modeDisclosure().open).toBe(false);
    await nextFrame();
    expect(scrolled).toEqual([]);
  });

  it('opens the disclosure when the selector mounts after the licence loads', async () => {
    const [locked, setLocked] = createSignal(true);
    renderHeaderAt(PATROL_CONTROL_PATH, createState({ autoFixLocked: locked }).state);
    expect(screen.queryByText('Mode and automation')).toBeNull();

    setLocked(false);

    await vi.waitFor(() => expect(modeDisclosure().open).toBe(true));
    await vi.waitFor(() => expect(scrolled).toEqual([modeDisclosure()]));
  });

  it('opens the disclosure when navigation moves onto the anchor', async () => {
    renderHeaderAt('/patrol', createState().state);
    expect(modeDisclosure().open).toBe(false);

    navigateTo(PATROL_CONTROL_PATH);

    await vi.waitFor(() => expect(modeDisclosure().open).toBe(true));
  });

  it('leaves a disclosure the user collapsed alone while the anchor stays', async () => {
    renderHeaderAt(PATROL_CONTROL_PATH, createState().state);
    const details = modeDisclosure();
    await vi.waitFor(() => expect(scrolled).toHaveLength(1));

    details.open = false;
    navigateTo(`/patrol?attention=finding-1#${PATROL_CONTROL_ANCHOR}`);
    await nextFrame();

    expect(details.open).toBe(false);
    expect(scrolled).toHaveLength(1);
  });

  it('needs no disclosure for plan-locked installs', async () => {
    renderHeaderAt(PATROL_CONTROL_PATH, createState({ autoFixLocked: true }).state);

    expect(document.querySelector('details')).toBeNull();
    expect(screen.getByText('Patrol mode').closest('#patrol-control')).not.toBeNull();
    await nextFrame();
    expect(scrolled).toEqual([]);
  });
});

describe('PatrolIntelligenceHeader Pro default mode', () => {
  afterEach(cleanup);

  it('tells a Pro install it investigates by default and lets it keep or leave Ask first', () => {
    const { state, handleAutonomyChange } = createState({ defaultModeApplied: true });
    renderHeader(state);

    const notice = screen.getByTestId('patrol-default-mode-notice');
    expect(notice).toHaveTextContent(
      'Pro investigates new warning and critical findings by default.',
    );
    expect(notice).toHaveTextContent('Nothing changes without your approval.');
    expect(notice).toHaveTextContent('count toward your 30-day budget');

    fireEvent.click(within(notice).getByRole('button', { name: 'Keep Ask first' }));
    expect(handleAutonomyChange).toHaveBeenLastCalledWith('approval');
    fireEvent.click(within(notice).getByRole('button', { name: 'Switch to Watch only' }));
    expect(handleAutonomyChange).toHaveBeenLastCalledWith('monitor');
  });

  it('shows no default notice once a mode was chosen or while Pro features are locked', () => {
    const chosen = createState({ defaultModeApplied: false });
    renderHeader(chosen.state);
    expect(screen.queryByTestId('patrol-default-mode-notice')).toBeNull();
    cleanup();

    const locked = createState({ defaultModeApplied: true, autoFixLocked: true });
    renderHeader(locked.state);
    expect(screen.queryByTestId('patrol-default-mode-notice')).toBeNull();
  });

  it('does not claim Patrol is watching while it still needs setup', () => {
    const { state } = createState({ setupOnly: true });
    renderHeader(state);
    expect(screen.getByText('Patrol is not running yet')).toBeInTheDocument();
    expect(screen.queryByText('Watching your infrastructure')).toBeNull();
  });
});

describe('PatrolIntelligenceHeader command access', () => {
  afterEach(cleanup);

  it('names the hosts Patrol can only watch and links to the one step', () => {
    resourcesMock.value = [
      agentHost('tower'),
      agentHost('docker01'),
      agentHost('pve1', true),
      agentHost('nas01'),
      agentHost('minipc'),
    ];
    const { state } = createState({ autonomyLevel: 'approval' });
    renderHeader(state);

    const strip = screen.getByTestId('patrol-command-access');
    expect(strip.closest('#patrol-control')).not.toBeNull();
    expect(strip).toHaveTextContent("Can't read logs or run fixes");
    expect(strip).toHaveTextContent('4 of 5 hosts');
    // Collapsed, the row previews the first names and counts the rest.
    const summary = strip.querySelector('summary')!;
    expect(summary).toHaveTextContent('docker01');
    expect(summary).toHaveTextContent('minipc');
    expect(summary).toHaveTextContent('nas01');
    expect(summary).toHaveTextContent('+1 more');
    expect(summary).not.toHaveTextContent('pve1');
    const all = within(strip).getByRole('list', { name: 'Hosts with commands off' });
    expect(
      within(all)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual(['docker01', 'minipc', 'nas01', 'tower']);
    expect(strip).toHaveTextContent('"Allow Pulse to run commands on this host" ticked');
    expect(within(strip).getByRole('link', { name: 'Get install command' })).toHaveAttribute(
      'href',
      '/settings/infrastructure?add=agent&commands=1',
    );
  });

  it('names the Proxmox nodes where Pulse cannot start a stopped guest, with no button', () => {
    const proxmoxGuest = (
      name: string,
      node: string,
      refused: boolean,
      reasonCode = 'guest_start_agent_not_connected',
    ): Resource =>
      ({
        id: `lxc:${name}`,
        type: 'system-container',
        name,
        displayName: name,
        status: 'online',
        sources: ['proxmox'],
        proxmox: { nodeName: node, instance: 'home' },
        ...(refused
          ? {
              actionReadiness: [{ name: 'start', available: false, reasonCode }],
            }
          : {}),
      }) as Resource;
    // Commands are on everywhere, so the commands row has nothing to name.
    resourcesMock.value = [
      agentHost('pve1', true),
      agentHost('pve2', true),
      proxmoxGuest('gitea', 'pve1', true),
      proxmoxGuest('pihole', 'pve1', true),
      // pve2's agent starts guests. Its running guest's stop still needs the
      // runner, and that is not what this row is about.
      {
        ...proxmoxGuest('nextcloud', 'pve2', false),
        actionReadiness: [
          { name: 'stop', available: false, reasonCode: 'action_runner_unavailable' },
        ],
      } as Resource,
    ];
    renderHeader(createState({ autonomyLevel: 'approval' }).state);

    expect(screen.queryByTestId('patrol-command-access')).toBeNull();
    const row = screen.getByTestId('patrol-guest-action-access');
    expect(row.closest('#patrol-control')).not.toBeNull();
    expect(row).toHaveTextContent("Can't start guests");
    expect(row).toHaveTextContent('1 Proxmox node');
    expect(row).not.toHaveTextContent('1 Proxmox nodes');
    const all = within(row).getByRole('list', {
      name: 'Proxmox nodes where Pulse cannot start guests',
    });
    expect(
      within(all)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual(['pve1']);
    expect(row).toHaveTextContent('you start it in Proxmox');
    expect(row).toHaveTextContent(
      'the node needs an up-to-date Pulse agent, installed with "Allow Pulse to run commands on this host" ticked, that Pulse can match to the node by address',
    );
    expect(row).not.toHaveTextContent('does not cover');
    // The causes differ per node, so the row carries no single button.
    expect(within(row).queryByRole('link')).toBeNull();
    expect(within(row).queryByRole('button')).toBeNull();
    cleanup();

    renderHeader(createState({ autonomyLevel: 'monitor' }).state);
    expect(screen.queryByTestId('patrol-guest-action-access')).toBeNull();
    cleanup();

    resourcesMock.value = [agentHost('pve2', true), proxmoxGuest('nextcloud', 'pve2', false)];
    renderHeader(createState({ autonomyLevel: 'full' }).state);
    expect(screen.queryByTestId('patrol-guest-action-access')).toBeNull();
  });

  it('stays quiet in Watch only, when every host is ready, and before setup', () => {
    resourcesMock.value = [agentHost('tower'), agentHost('docker01')];
    renderHeader(createState({ autonomyLevel: 'monitor' }).state);
    expect(screen.queryByTestId('patrol-command-access')).toBeNull();
    cleanup();

    // A locked plan runs as Watch only whatever mode was saved.
    renderHeader(createState({ autonomyLevel: 'approval', autoFixLocked: true }).state);
    expect(screen.queryByTestId('patrol-command-access')).toBeNull();
    cleanup();

    renderHeader(createState({ autonomyLevel: 'approval', setupOnly: true }).state);
    expect(screen.queryByTestId('patrol-command-access')).toBeNull();
    cleanup();

    resourcesMock.value = [agentHost('tower', true), agentHost('docker01', true)];
    renderHeader(createState({ autonomyLevel: 'full' }).state);
    expect(screen.queryByTestId('patrol-command-access')).toBeNull();
  });
});
