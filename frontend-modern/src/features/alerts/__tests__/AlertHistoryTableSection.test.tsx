import { render } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import * as observedWidth from '@/hooks/useObservedElementWidth';
import * as breakpoint from '@/hooks/useBreakpoint';
import { AlertHistoryTableSection } from '../AlertHistoryTableSection';
import type { AlertHistoryState } from '../useAlertHistoryState';

vi.mock('@/hooks/useObservedElementWidth', async () => {
  const { createSignal } = await import('solid-js');
  const [width, setWidth] = createSignal<number | null>(null);
  return {
    useObservedElementWidth: () => ({ setElement: () => undefined, width }),
    setObservedWidth: setWidth,
  };
});
vi.mock('@/hooks/useBreakpoint', async () => {
  const { createSignal } = await import('solid-js');
  const [isMobile, setIsMobile] = createSignal(false);
  return { useBreakpoint: () => ({ isMobile }), setIsMobile };
});
vi.mock('../AlertHistoryMobileList', () => ({
  AlertHistoryMobileList: (props: { active: () => boolean }) => (
    <div data-testid="history-cards" data-active={String(props.active())} />
  ),
}));
vi.mock('@/features/platformPage/PlatformWindowedRows', () => ({
  PlatformWindowedRows: () => null,
}));

const setObservedWidth = (
  observedWidth as unknown as { setObservedWidth: (w: number | null) => void }
).setObservedWidth;
const setIsMobile = (breakpoint as unknown as { setIsMobile: (value: boolean) => void })
  .setIsMobile;

const state = {
  loading: () => false,
  alertData: () => [{ id: 'a-1' }],
  historyLoadError: () => null,
  groupedAlerts: () => [],
} as unknown as AlertHistoryState;

// A laptop window with the Alerts rail open gave the table a card under
// 800px: names broke mid-word and the three row actions stacked, while the
// cards stayed hidden because they only showed below a 768px viewport.
describe('AlertHistoryTableSection layout', () => {
  it('shows cards whenever the card is too narrow for the table', () => {
    const { container, getByTestId } = render(() => <AlertHistoryTableSection state={state} />);
    const layout = () => container.querySelector('[data-alert-history-layout]');
    const tableHidden = () => container.querySelector('table')?.closest('.hidden') !== null;

    // Before the card is measured, the phone breakpoint decides.
    expect(layout()).toHaveAttribute('data-alert-history-layout', 'table');
    expect(getByTestId('history-cards')).toHaveAttribute('data-active', 'false');
    expect(tableHidden()).toBe(false);

    setObservedWidth(640);
    expect(layout()).toHaveAttribute('data-alert-history-layout', 'cards');
    expect(getByTestId('history-cards')).toHaveAttribute('data-active', 'true');
    expect(tableHidden()).toBe(true);

    setObservedWidth(799);
    expect(layout()).toHaveAttribute('data-alert-history-layout', 'cards');

    setObservedWidth(800);
    expect(layout()).toHaveAttribute('data-alert-history-layout', 'table');
    expect(getByTestId('history-cards')).toHaveAttribute('data-active', 'false');
    expect(tableHidden()).toBe(false);

    setObservedWidth(null);
    setIsMobile(true);
    expect(layout()).toHaveAttribute('data-alert-history-layout', 'cards');
  });
});
