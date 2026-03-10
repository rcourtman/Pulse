import { fireEvent, render } from '@solidjs/testing-library';
import { createEffect, type Component } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { HistoryTimeRange } from '@/api/charts';
import { useDrawerHistoryRange } from '../useDrawerHistoryRange';

const TestHarness: Component<{ resourceKey: string }> = (props) => {
  const [historyRange, setHistoryRange] = useDrawerHistoryRange(props.resourceKey);

  createEffect(() => {
    historyRange();
  });

  return (
    <select
      aria-label="History range"
      value={historyRange()}
      onChange={(event) => setHistoryRange(event.currentTarget.value as HistoryTimeRange)}
    >
      <option value="1h">Last 1 hour</option>
      <option value="6h">Last 6 hours</option>
      <option value="12h">Last 12 hours</option>
      <option value="24h">Last 24 hours</option>
      <option value="7d">Last 7 days</option>
      <option value="30d">Last 30 days</option>
      <option value="90d">Last 90 days</option>
    </select>
  );
};

describe('useDrawerHistoryRange', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  it('preserves the selected range across remounts for the same resource', async () => {
    const first = render(() => <TestHarness resourceKey="vm:cluster-a:100" />);
    const select = first.getByLabelText('History range') as HTMLSelectElement;

    await fireEvent.change(select, { target: { value: '7d' } });
    expect(select.value).toBe('7d');

    first.unmount();

    const second = render(() => <TestHarness resourceKey="vm:cluster-a:100" />);
    expect((second.getByLabelText('History range') as HTMLSelectElement).value).toBe('7d');
  });

  it('falls back to 1h when storage contains an invalid range', () => {
    window.localStorage.setItem('pulse.drawerHistoryRange.host:host-1', 'bogus');

    const view = render(() => <TestHarness resourceKey="host:host-1" />);
    expect((view.getByLabelText('History range') as HTMLSelectElement).value).toBe('1h');
  });
});
