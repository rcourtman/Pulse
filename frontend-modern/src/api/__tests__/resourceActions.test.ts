import { beforeEach, describe, expect, it, vi } from 'vitest';
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));
import { ResourceActionsAPI } from '@/api/resourceActions';
import { apiFetchJSON } from '@/utils/apiClient';

describe('ResourceActionsAPI durable inbox', () => {
  const fetchJSON = vi.mocked(apiFetchJSON);
  beforeEach(() => fetchJSON.mockReset());

  it('lists canonical pending and settled action views without a local projection', async () => {
    fetchJSON.mockResolvedValue({ view: 'pending', actions: [], count: 0 });
    await ResourceActionsAPI.listActions('pending', 75);
    expect(fetchJSON).toHaveBeenCalledWith('/api/actions?view=pending&limit=75');
  });

  it('loads durable action detail by encoded server action id', async () => {
    fetchJSON.mockResolvedValue({ audit: {}, events: [] });
    await ResourceActionsAPI.getAction('action/one');
    expect(fetchJSON).toHaveBeenCalledWith('/api/actions/action%2Fone');
  });
});
