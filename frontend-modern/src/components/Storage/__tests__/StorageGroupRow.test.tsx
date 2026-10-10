import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { StorageGroupedRecords } from '../useStorageModel';
import { StorageGroupRow } from '../StorageGroupRow';

vi.mock('../EnhancedStorageBar', () => ({
  EnhancedStorageBar: () => <div data-testid="enhanced-storage-bar" />,
}));

const makeGroup = (): StorageGroupedRecords => ({
  key: 'tower',
  items: [{ id: 'pool-1' }, { id: 'pool-2' }] as any[],
  stats: {
    totalBytes: 1000,
    usedBytes: 400,
    usagePercent: 40,
    byHealth: {
      healthy: 1,
      warning: 1,
      critical: 1,
      offline: 1,
      unknown: 1,
    },
  },
});

afterEach(() => {
  cleanup();
});

describe('StorageGroupRow', () => {
  it('renders grouped health dots from the canonical storage health presentation', () => {
    const onToggle = vi.fn();
    const { container } = render(() => (
      <table>
        <tbody>
          <StorageGroupRow
            group={makeGroup()}
            summaryGroupId={null}
            expanded={false}
            onToggle={onToggle}
          />
        </tbody>
      </table>
    ));

    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('tower').closest('tr')).toHaveClass('grouped-table-row');
    expect(screen.getByText('2 storage items')).toBeInTheDocument();
    expect(screen.getByText('40%')).toBeInTheDocument();
    expect(container.querySelector('.bg-green-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-yellow-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-red-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-slate-400')).toBeInTheDocument();
    expect(container.querySelector('.bg-slate-300')).toBeInTheDocument();
  });

  it('lets the whole group row own disclosure and nothing else', () => {
    const onToggle = vi.fn();

    render(() => (
      <table>
        <tbody>
          <StorageGroupRow
            group={makeGroup()}
            summaryGroupId="storage:node:tower"
            expanded={false}
            onToggle={onToggle}
          />
        </tbody>
      </table>
    ));

    const row = screen.getByText('tower').closest('tr');
    expect(row).not.toBeNull();
    if (!row) {
      return;
    }
    expect(row).toHaveAttribute('data-summary-group-id', 'storage:node:tower');
    expect(row).not.toHaveAttribute('data-summary-row-active');
    expect(row).not.toHaveAttribute('data-summary-group-series-count');
    expect(
      screen.queryByRole('button', {
        name: 'Pin summary scope for tower',
      }),
    ).not.toBeInTheDocument();

    fireEvent.click(row);
    expect(onToggle).toHaveBeenCalledTimes(1);

    const toggleButton = screen.getByRole('button', { name: 'Expand tower' });
    expect(toggleButton).toHaveClass('sr-only');
    expect(toggleButton).toHaveClass('sm:not-sr-only');
    fireEvent.click(toggleButton);
    expect(onToggle).toHaveBeenCalledTimes(2);
  });
});
