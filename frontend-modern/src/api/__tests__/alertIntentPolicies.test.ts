import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  AlertIntentPoliciesAPI,
  type AlertIntentPolicyDocument,
  type AlertIntentPolicyPreviewRequest,
} from '@/api/alertIntentPolicies';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

describe('AlertIntentPoliciesAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('reads, updates, and previews policies through the canonical routes', async () => {
    const document: AlertIntentPolicyDocument = {
      schemaVersion: 1,
      revision: 4,
      defaults: {
        'state.offline': {
          graceSeconds: 90,
          honorOperatorState: true,
        },
      },
    };
    const previewRequest: AlertIntentPolicyPreviewRequest = {
      resourceId: 'vm:pve-a:101',
      resourceType: 'vm',
      signal: 'state.offline',
      conditionActive: true,
      firstMatchedAt: '2026-07-20T12:00:00Z',
    };
    const preview = {
      resourceId: previewRequest.resourceId,
      resourceType: previewRequest.resourceType,
      signal: previewRequest.signal,
      status: 'pending_grace' as const,
      reason: 'grace period active',
      effective: {
        graceSeconds: 90,
        honorOperatorState: true,
        sources: { graceSeconds: 'defaults.state.offline' },
        explicit: true,
      },
      contexts: [],
      warnings: [],
    };

    mockedApiFetchJSON.mockResolvedValueOnce(document);
    await expect(AlertIntentPoliciesAPI.get()).resolves.toEqual(document);
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/alerts/intent-policies');

    mockedApiFetchJSON.mockResolvedValueOnce({ ...document, revision: 5 });
    await expect(AlertIntentPoliciesAPI.update(document)).resolves.toEqual({
      ...document,
      revision: 5,
    });
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/alerts/intent-policies', {
      method: 'PUT',
      body: JSON.stringify(document),
    });

    mockedApiFetchJSON.mockResolvedValueOnce(preview);
    await expect(AlertIntentPoliciesAPI.preview(previewRequest)).resolves.toEqual(preview);
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/alerts/intent-policies/preview', {
      method: 'POST',
      body: JSON.stringify(previewRequest),
    });
  });
});
