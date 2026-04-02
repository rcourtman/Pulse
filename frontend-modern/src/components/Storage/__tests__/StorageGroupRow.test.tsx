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
            groupBy="node"
            expanded={false}
            onToggle={onToggle}
            summaryGroupScope={null}
            summaryActive={false}
            summaryFocused={false}
          />
        </tbody>
      </table>
    ));

    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('2 pools')).toBeInTheDocument();
    expect(screen.getByText('40%')).toBeInTheDocument();
    expect(container.querySelector('.bg-green-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-yellow-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-red-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-slate-400')).toBeInTheDocument();
    expect(container.querySelector('.bg-slate-300')).toBeInTheDocument();
  });

  it('separates summary focus from expand-collapse controls on group rows', () => {
    const onToggle = vi.fn();
    const onFocusChange = vi.fn();
    const onHoverChange = vi.fn();
    const scope = {
      id: 'storage:node:tower',
      label: 'tower (2 pools)',
      seriesIds: ['pool-1', 'pool-2'],
    };

    render(() => (
      <table>
        <tbody>
          <StorageGroupRow
            group={makeGroup()}
            groupBy="node"
            expanded={false}
            onToggle={onToggle}
            summaryGroupScope={scope}
            summaryActive={false}
            summaryFocused={false}
            onFocusChange={onFocusChange}
            onHoverChange={onHoverChange}
          />
        </tbody>
      </table>
    ));

    const row = screen.getByText('tower').closest('tr');
    expect(row).not.toBeNull();
    if (!row) {
      return;
    }

    fireEvent.pointerEnter(row, { pointerType: 'mouse' });
    expect(onHoverChange).toHaveBeenCalledWith(scope);

    const scopeButton = screen.getByRole('button', {
      name: 'Pin summary scope for tower',
    });
    fireEvent.focusIn(scopeButton);
    expect(onHoverChange).toHaveBeenLastCalledWith(scope);

    fireEvent.click(scopeButton);
    expect(onFocusChange).toHaveBeenCalledWith(scope);

    fireEvent.click(row);
    expect(onFocusChange).toHaveBeenCalledWith(scope);
    expect(onToggle).not.toHaveBeenCalled();

    const toggleButton = screen.getByRole('button', { name: 'Expand tower' });
    fireEvent.click(toggleButton);
    expect(onToggle).toHaveBeenCalledTimes(1);

    fireEvent.pointerLeave(row, { pointerType: 'mouse' });
    expect(onHoverChange).toHaveBeenLastCalledWith(null);
  });
});
