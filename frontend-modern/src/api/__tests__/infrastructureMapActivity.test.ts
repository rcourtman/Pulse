import { beforeEach, describe, expect, it, vi } from 'vitest';

import { apiFetchJSON } from '@/utils/apiClient';
import {
  recordInfrastructureMapExport,
  recordInfrastructureMapOpen,
  resetInfrastructureMapActivityForTest,
} from '../infrastructureMapActivity';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));

const sentBodies = (): string[] =>
  vi.mocked(apiFetchJSON).mock.calls.map(([, init]) => String(init?.body));

describe('infrastructure map activity', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.sessionStorage.clear();
    resetInfrastructureMapActivityForTest();
    vi.mocked(apiFetchJSON).mockResolvedValue(null);
  });

  it('counts an open once per browser session and sends only the milestone', () => {
    recordInfrastructureMapOpen();
    recordInfrastructureMapOpen();

    expect(apiFetchJSON).toHaveBeenCalledTimes(1);
    expect(apiFetchJSON).toHaveBeenCalledWith('/api/usage/infrastructure-map', {
      method: 'POST',
      body: '{"activity":"open"}',
    });
  });

  it('still counts once after a reload in the same session', () => {
    recordInfrastructureMapOpen();
    resetInfrastructureMapActivityForTest();
    recordInfrastructureMapOpen();

    expect(apiFetchJSON).toHaveBeenCalledTimes(1);
  });

  it('records a saved image and that names were hidden', () => {
    recordInfrastructureMapExport({ namesHidden: true });

    expect(sentBodies()).toEqual(['{"activity":"export"}', '{"activity":"export_names_hidden"}']);
  });

  it('adds only what a later save newly shows', () => {
    recordInfrastructureMapExport({ namesHidden: false });
    recordInfrastructureMapExport({ namesHidden: false });
    recordInfrastructureMapExport({ namesHidden: true });
    recordInfrastructureMapExport({ namesHidden: true });

    expect(sentBodies()).toEqual(['{"activity":"export"}', '{"activity":"export_names_hidden"}']);
  });

  it('allows a later retry when the local intake failed', async () => {
    vi.mocked(apiFetchJSON).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(null);

    recordInfrastructureMapOpen();
    await Promise.resolve();
    await Promise.resolve();
    recordInfrastructureMapOpen();

    expect(apiFetchJSON).toHaveBeenCalledTimes(2);
  });
});
