import { cleanup, fireEvent, render, within } from '@solidjs/testing-library';
import { createSignal, Show } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { ResourceDetailDrawerDebugTab } from '../ResourceDetailDrawerDebugTab';
import type { PlatformData } from '../resourceDetailMappers';
import { useResourceDetailDrawerDerivedState } from '../useResourceDetailDrawerDerivedState';
import type { UseResourceDetailDrawerStateResult } from '../useResourceDetailDrawerState';

const start = Date.parse('2026-10-11T10:00:00Z');
const makeResource = (): Resource => ({
  id: 'fixture:pve-a:101',
  name: 'Backup guest',
  displayName: 'Backup guest',
  type: 'vm',
  platformId: '101',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: start - 60_000,
  platformData: {
    proxmox: { vmid: 101, node: 'pve-a', diskStatusReason: 'prev-agent-timeout' },
    agent: { id: 'fixture-agent' },
    sourceStatus: {
      proxmox: { status: 'deferred', lastSeen: start - 60_000, error: 'Guest read deferred' },
      agent: { status: 'stale', lastSeen: new Date(start - 120_000).toISOString() },
    },
  },
});

// Use the production derived state, including its real platformData/section
// replacement behaviour. Only the clipboard action is replaced here.
function Fixture(props: { resource: Resource; onCopy?: (json: string) => void }) {
  const derived = useResourceDetailDrawerDerivedState({
    get resource() {
      return props.resource;
    },
    debugEnabled: () => true,
    discoveryFeatureEnabled: () => false,
    resourceIntelligence: () => null,
  });
  const drawer = {
    ...derived,
    copied: () => false,
    handleCopyJson: () => props.onCopy?.(derived.debugJson()),
  } as unknown as UseResourceDetailDrawerStateResult;
  return (
    <>
      <output data-testid="drawer-last-seen">{derived.lastSeen()}</output>
      <ResourceDetailDrawerDebugTab resource={props.resource} drawer={drawer} />
    </>
  );
}

const hoursLater = (hours: number) => {
  vi.setSystemTime(Date.now() + hours * 3_600_000);
  vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('resource Debug source ages', () => {
  it('follows arriving status records while payload sections, disclosure and focus stay mounted', () => {
    vi.useFakeTimers({ now: start });
    const [resource, setResource] = createStore(
      makeResource() as Resource & { platformData: PlatformData },
    );
    const view = render(() => <Fixture resource={resource} />);
    const details = [...view.container.querySelectorAll('details')];
    const summaries = details.map((element) => element.querySelector('summary')!);
    const payloads = details.map((element) => element.querySelector('pre')!.textContent);
    details[0].open = true;
    summaries[0].focus();

    // Only source status is replaced, as in a nested canonical store update.
    // The real derived-state sourceSections memo keeps its original entries.
    setResource('platformData', 'sourceStatus', {
      proxmox: { status: 'available', lastSeen: start },
      agent: {
        status: 'error',
        lastSeen: new Date(start).toISOString(),
        error: 'Agent unavailable',
      },
    });
    expect(summaries[0]).toHaveTextContent('available • 0s ago');
    expect(summaries[1]).toHaveTextContent('error • 0s ago');
    expect(within(view.container).queryByText('Guest read deferred')).not.toBeInTheDocument();
    expect(within(view.container).getByText('Agent unavailable')).toBeInTheDocument();
    expect([...view.container.querySelectorAll('details')]).toEqual(details);
    expect(details[0].open).toBe(true);
    expect(summaries[0]).toHaveFocus();
    expect(details.map((element) => element.querySelector('pre')!.textContent)).toEqual(payloads);

    vi.advanceTimersByTime(60_000);
    expect(summaries[0]).toHaveTextContent('available • 1 min ago');
    expect(summaries[1]).toHaveTextContent('error • 1 min ago');
    setResource(
      'platformData',
      'sourceStatus',
      reconcile({
        proxmox: { status: 'deferred', lastSeen: 'invalid', error: 'No usable timestamp' },
      }),
    );
    expect(summaries[0].textContent).toBe('Proxmoxdeferred');
    expect(summaries[1].textContent).toBe('Agentunknown');
    expect(within(view.container).queryByText('Agent unavailable')).not.toBeInTheDocument();
    expect(within(view.container).getByText('No usable timestamp')).toBeInTheDocument();
  });

  it('ages held source timestamps with the drawer without receiving another snapshot', () => {
    vi.useFakeTimers({ now: start });
    const resource = makeResource();
    const view = render(() => <Fixture resource={resource} />);
    const summaries = () =>
      [...view.container.querySelectorAll('summary')].map((el) => el.textContent);

    expect(summaries()).toEqual(['Proxmoxdeferred • 1 min ago', 'Agentstale • 2 mins ago']);
    hoursLater(2);

    expect(within(view.container).getByTestId('drawer-last-seen')).toHaveTextContent('2 hours ago');
    expect(summaries()).toEqual(['Proxmoxdeferred • 2 hours ago', 'Agentstale • 2 hours ago']);
    expect((resource.platformData as PlatformData).sourceStatus?.proxmox.lastSeen).toBe(
      start - 60_000,
    );
    expect(within(view.container).getByText('Guest read deferred')).toBeInTheDocument();
  });

  it('keeps disclosure, focus, source payload and copied JSON unchanged across age ticks', () => {
    vi.useFakeTimers({ now: start });
    const copied = vi.fn();
    const view = render(() => <Fixture resource={makeResource()} onCopy={copied} />);
    const details = view.container.querySelector<HTMLDetailsElement>('details')!;
    const summary = details.querySelector('summary')!;
    const copy = within(view.container).getByRole('button', { name: 'Copy JSON' });
    fireEvent.click(copy);
    const original = copied.mock.calls[0][0];
    details.open = true;
    summary.focus();
    const payload = details.querySelector('pre')!.textContent;

    hoursLater(4);

    expect(summary).toHaveTextContent('deferred • 4 hours ago');
    expect(view.container.querySelector('details')).toBe(details);
    expect(details.open).toBe(true);
    expect(summary).toHaveFocus();
    expect(details.querySelector('pre')!.textContent).toBe(payload);
    fireEvent.click(copy);
    expect(copied.mock.calls[1][0]).toBe(original);
    const bundle = JSON.parse(original);
    expect(bundle.sources.proxmox.diskStatusReason).toBe('prev-agent-timeout');
    expect(bundle.sources.sourceStatus.proxmox.status).toBe('deferred');
    expect(bundle.sources.sourceStatus.proxmox.lastSeen).toBe(start - 60_000);
  });

  it('still follows ordinary platformData replacements rather than retaining old status or errors', () => {
    vi.useFakeTimers({ now: start });
    const [resource, setResource] = createSignal(makeResource());
    const view = render(() => <Fixture resource={resource()} />);
    setResource({
      ...resource(),
      platformData: {
        proxmox: { vmid: 101, node: 'pve-a', diskStatusReason: '' },
        sourceStatus: { proxmox: { status: 'available', lastSeen: start } },
      },
    });
    expect(view.container.querySelector('summary')).toHaveTextContent('available • 0s ago');
    expect(within(view.container).queryByText('Guest read deferred')).not.toBeInTheDocument();
    expect(view.container.querySelectorAll('details')).toHaveLength(1);

    vi.advanceTimersByTime(60_000);

    expect(view.container.querySelector('summary')).toHaveTextContent('available • 1 min ago');
  });

  it('does not invent ages or freshness for absent and invalid source timestamps', () => {
    vi.useFakeTimers({ now: start });
    const resource = makeResource();
    resource.platformData = {
      proxmox: { vmid: 101 },
      agent: { id: 'fixture-agent' },
      docker: { id: 'fixture-docker' },
      sourceStatus: {
        proxmox: { status: 'deferred' },
        agent: { status: 'stale', lastSeen: 'not-a-date', error: 'No usable timestamp' },
      },
    };
    const view = render(() => <Fixture resource={resource} />);
    hoursLater(2);
    expect([...view.container.querySelectorAll('summary')].map((el) => el.textContent)).toEqual([
      'Proxmoxdeferred',
      'Agentstale',
      'Dockerunknown',
    ]);
    expect(within(view.container).getByText('No usable timestamp')).toBeInTheDocument();
  });

  it('shares one clock across drawers and releases it when the last drawer unmounts', () => {
    vi.useFakeTimers({ now: start });
    const [first, setFirst] = createSignal(true);
    const [second, setSecond] = createSignal(true);
    render(() => (
      <>
        <Show when={first()}>
          <Fixture resource={makeResource()} />
        </Show>
        <Show when={second()}>
          <Fixture resource={makeResource()} />
        </Show>
      </>
    ));
    expect(vi.getTimerCount()).toBe(1);
    setFirst(false);
    expect(vi.getTimerCount()).toBe(1);
    setSecond(false);
    expect(vi.getTimerCount()).toBe(0);
  });
});
