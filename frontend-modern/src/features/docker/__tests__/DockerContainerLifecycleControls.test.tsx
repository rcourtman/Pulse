import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { createTooltipSystem, hideTooltip } from '@/components/shared/Tooltip';
import type { Resource } from '@/types/resource';
import { DockerContainerLifecycleControls } from '../DockerContainerLifecycleControls';

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: { getAction: vi.fn(), planAction: vi.fn() },
}));

vi.mock('@/features/actions/ActionReviewDialog', () => ({
  ActionReviewDialog: () => null,
}));

const AGENT_REFUSAL = 'Docker / Podman command agent is not connected.';

const refusedContainer = (): Resource =>
  ({
    id: 'app-container:docker-host:web',
    name: 'web',
    displayName: 'web',
    platformId: 'docker-1',
    platformType: 'docker',
    sourceType: 'agent',
    sources: ['docker'],
    status: 'running',
    type: 'app-container',
    lastSeen: 1_700_000_000_000,
    docker: {
      runtime: 'docker',
      agentId: 'agent-1',
      containerId: 'abc123',
      containerState: 'running',
    },
    actionReadiness: [
      {
        name: 'stop',
        available: false,
        reasonCode: 'command_agent_disconnected',
        reason: AGENT_REFUSAL,
      },
      {
        name: 'restart',
        available: false,
        reasonCode: 'command_agent_disconnected',
        reason: AGENT_REFUSAL,
      },
    ],
  }) as Resource;

const useTouchScreen = () =>
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockReturnValue({ matches: false, media: '(hover: hover) and (pointer: fine)' }),
  );

const tooltipText = () => document.body.querySelector('[data-tooltip="true"]')?.textContent ?? null;

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
  hideTooltip();
});

describe('DockerContainerLifecycleControls without hover', () => {
  it('shows each reason once in the row menu', () => {
    render(() => <DockerContainerLifecycleControls resource={refusedContainer()} collapsed />);

    fireEvent.click(screen.getByRole('button', { name: 'Container actions for web' }));

    const menu = screen.getByRole('menu', { name: 'Container actions for web' });
    const reasons = menu.querySelector('[data-docker-lifecycle-disabled-reasons]') as HTMLElement;
    expect(Array.from(reasons.children).map((reason) => reason.textContent)).toEqual([
      'Container is already running.',
      AGENT_REFUSAL,
    ]);
    expect(menu).toHaveAttribute('aria-describedby', reasons.id);
    expect(
      within(menu).getByRole('menuitem', { name: `Stop unavailable: ${AGENT_REFUSAL}` }),
    ).toBeDisabled();
  });

  it('opens a menu with reasons upward when it does not fit below its row', async () => {
    vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(200);
    render(() => <DockerContainerLifecycleControls resource={refusedContainer()} collapsed />);

    const trigger = screen.getByRole('button', { name: 'Container actions for web' });
    const top = window.innerHeight - 48;
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue({
      right: 300,
      top,
      bottom: top + 28,
    } as DOMRect);
    fireEvent.click(trigger);

    await waitFor(() => expect(screen.getByRole('menu').style.top).toBe(`${top - 200 - 4}px`));
  });

  it('answers a tap on an unavailable action on the resource detail surface', () => {
    useTouchScreen();
    const TooltipRoot = createTooltipSystem();
    render(() => (
      <>
        <TooltipRoot />
        <DockerContainerLifecycleControls resource={refusedContainer()} surface="resource-detail" />
      </>
    ));

    const restart = screen.getByRole('button', { name: `Restart unavailable: ${AGENT_REFUSAL}` });
    expect(restart).toHaveAttribute('aria-disabled', 'true');
    expect(tooltipText()).toBeNull();

    fireEvent.click(restart);

    expect(tooltipText()).toBe(`Restart unavailable: ${AGENT_REFUSAL}`);
    expect(ResourceActionsAPI.planAction).not.toHaveBeenCalled();

    // A touch browser fires this as soon as the tooltip mounts.
    fireEvent.mouseLeave(restart);
    expect(tooltipText()).toBe(`Restart unavailable: ${AGENT_REFUSAL}`);

    fireEvent.pointerDown(document.body);
    expect(tooltipText()).toBeNull();
  });
});
