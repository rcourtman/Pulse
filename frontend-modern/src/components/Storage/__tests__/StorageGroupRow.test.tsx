import { cleanup, render, screen } from '@solidjs/testing-library';
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
          <StorageGroupRow group={makeGroup()} groupBy="node" expanded={false} onToggle={onToggle} />
        </tbody>
      </table>
    ));

    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('2 pools')).toBeInTheDocument();
    expect(container.querySelector('.bg-green-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-yellow-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-red-500')).toBeInTheDocument();
    expect(container.querySelector('.bg-slate-400')).toBeInTheDocument();
    expect(container.querySelector('.bg-slate-300')).toBeInTheDocument();
  });
});
