import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { RBACAPI } from '@/api/rbac';
import { SecurityAPI } from '@/api/security';
import { NodesAPI } from '@/api/nodes';
import { NotificationsAPI } from '@/api/notifications';
import { UpdatesAPI } from '@/api/updates';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';

describe('API URL encoding', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchMock.mockReset();
  });

  it('encodes RBAC and token path segments', async () => {
    apiFetchJSONMock.mockResolvedValue({} as never);

    await RBACAPI.getRole('role/root/admin');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/admin/roles/role%2Froot%2Fadmin');

    await RBACAPI.getUserAssignment('alice/dev');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/admin/users/alice%2Fdev/roles');

    await SecurityAPI.deleteToken('tok/123?x=1');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/security/tokens/tok%2F123%3Fx%3D1', {
      method: 'DELETE',
    });
  });

  it('encodes node and webhook ids', async () => {
    apiFetchJSONMock.mockResolvedValue({} as never);

    await NodesAPI.updateNode('node/1', {} as never);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/config/nodes/node%2F1', {
      method: 'PUT',
      body: JSON.stringify({}),
    });

    await NotificationsAPI.deleteWebhook('hook/a?b=1');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/webhooks/hook%2Fa%3Fb%3D1', {
      method: 'DELETE',
    });
  });

  it('builds update query strings safely', async () => {
    apiFetchJSONMock.mockResolvedValue({} as never);

    await UpdatesAPI.checkForUpdates('stable&next' as never);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/updates/check?channel=stable%26next');

    await UpdatesAPI.getUpdatePlan('v1.2.3&x=1', 'beta/dev' as never);
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/updates/plan?version=v1.2.3%26x%3D1&channel=beta%2Fdev',
    );
  });
});
