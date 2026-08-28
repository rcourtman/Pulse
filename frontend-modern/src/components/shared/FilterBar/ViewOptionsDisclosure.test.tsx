import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import {
  ViewOptionsDisclosurePanel,
  ViewOptionsDisclosureTrigger,
  createViewOptionsDisclosureState,
} from './ViewOptionsDisclosure';

afterEach(cleanup);

const TestDisclosure = () => {
  const state = createViewOptionsDisclosureState();
  return (
    <>
      <ViewOptionsDisclosureTrigger state={state} />
      <ViewOptionsDisclosurePanel state={state}>
        <button type="button">Grouped</button>
      </ViewOptionsDisclosurePanel>
    </>
  );
};

describe('ViewOptionsDisclosure', () => {
  it('reveals persistent presentation controls as an inline region', () => {
    render(() => <TestDisclosure />);

    const trigger = screen.getByRole('button', { name: 'View' });
    expect(screen.queryByRole('region', { name: 'View preferences' })).not.toBeInTheDocument();
    expect(trigger).not.toHaveAttribute('aria-haspopup');

    fireEvent.click(trigger);

    const region = screen.getByRole('region', { name: 'View preferences' });
    expect(region).toHaveClass('border', 'bg-surface-alt/40');
    expect(region).not.toHaveClass('absolute', 'fixed', 'shadow-lg');
    expect(region.firstElementChild?.nextElementSibling).toHaveClass('view-options-grid', 'grid');
    expect(screen.getByRole('button', { name: 'Grouped' })).toBeInTheDocument();
    expect(trigger).toHaveAttribute('aria-expanded', 'true');

    fireEvent.click(trigger);
    expect(screen.queryByRole('region', { name: 'View preferences' })).not.toBeInTheDocument();
  });

  it('collapses on Escape and restores focus to the trigger', async () => {
    render(() => <TestDisclosure />);
    const trigger = screen.getByRole('button', { name: 'View' });

    fireEvent.click(trigger);
    const grouped = screen.getByRole('button', { name: 'Grouped' });
    grouped.focus();
    fireEvent.keyDown(grouped, { key: 'Escape' });
    await Promise.resolve();

    expect(screen.queryByRole('region', { name: 'View preferences' })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });
});
