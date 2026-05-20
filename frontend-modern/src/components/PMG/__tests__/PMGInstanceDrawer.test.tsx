import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';

const apiFetchJSONMock = vi.fn();

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

import { PMGInstanceDrawer } from '@/components/PMG/PMGInstanceDrawer';

describe('PMGInstanceDrawer', () => {
  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('uses the shared platform table spacing and divider contract for detail tables', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      id: 'pmg-1',
      type: 'pmg',
      name: 'mail-gateway',
      pmg: {
        hostname: 'mail-gateway.local',
        version: '8.1.0',
        nodes: [
          {
            name: 'mail-node-a',
            role: 'master',
            status: 'online',
            queueStatus: { total: 12 },
          },
        ],
        relayDomains: [{ domain: 'example.com', comment: 'primary' }],
        domainStats: [
          {
            domain: 'example.com',
            mailCount: 128,
            spamCount: 4,
            virusCount: 1,
            bytes: 4096,
          },
        ],
      },
    });

    const { container } = render(() => (
      <PMGInstanceDrawer resourceId="pmg-1" resourceName="Mail Gateway" />
    ));

    await waitFor(() => {
      expect(screen.getByText('mail-node-a')).toBeInTheDocument();
      expect(screen.getAllByText('example.com').length).toBeGreaterThan(0);
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/resources/pmg-1', {
      cache: 'no-store',
    });

    const tables = Array.from(container.querySelectorAll('table'));
    expect(tables).toHaveLength(3);
    for (const table of tables) {
      expect(table).toHaveClass('table-fixed');
      expect(table.querySelector('thead tr')).toHaveClass('bg-surface-alt');
      expect(table.querySelector('tbody')).toHaveClass('divide-y', 'divide-border');
      expect(table.querySelector('tbody')).not.toHaveClass('divide-border-subtle');
      expect(table.querySelector('th')).toHaveClass('px-1.5', 'py-0.5');
    }
  });
});
