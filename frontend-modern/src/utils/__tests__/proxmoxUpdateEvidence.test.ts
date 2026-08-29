import { describe, expect, it } from 'vitest';

import {
  getProxmoxUpdateEvidencePresentation,
  hasCurrentProxmoxUpdateEvidence,
} from '@/utils/proxmoxUpdateEvidence';

const relative = () => '5 mins ago';

describe('Proxmox update evidence presentation', () => {
  it('makes a confirmed zero visible', () => {
    expect(
      getProxmoxUpdateEvidencePresentation(
        {
          pendingUpdates: 0,
          pendingUpdatesStatus: 'checked',
          pendingUpdatesCheckedAt: '2026-08-29T12:00:00Z',
        },
        relative,
      ),
    ).toEqual({
      value: 'No pending updates · checked 5 mins ago',
      title: 'Last successful update check 5 mins ago',
      tone: 'success',
      current: true,
    });
  });

  it('labels retained counts stale after a failed refresh', () => {
    expect(
      getProxmoxUpdateEvidencePresentation(
        {
          pendingUpdates: 12,
          pendingUpdatesStatus: 'stale',
          pendingUpdatesReason: 'source_unavailable',
          pendingUpdatesCheckedAt: '2026-08-29T12:00:00Z',
        },
        relative,
      ),
    ).toMatchObject({
      value: '12 pending · stale · checked 5 mins ago',
      title: 'Proxmox source unavailable · last success 5 mins ago',
      current: false,
    });
  });

  it('explains unavailable permission evidence without exposing provider errors', () => {
    expect(
      getProxmoxUpdateEvidencePresentation({
        pendingUpdatesStatus: 'unavailable',
        pendingUpdatesReason: 'permission_denied',
      }),
    ).toMatchObject({
      value: 'Unavailable · Sys.Audit permission required',
      current: false,
    });
  });

  it('only treats checked or legacy-positive evidence as current', () => {
    expect(hasCurrentProxmoxUpdateEvidence({ pendingUpdates: 3 })).toBe(true);
    expect(
      hasCurrentProxmoxUpdateEvidence({ pendingUpdates: 3, pendingUpdatesStatus: 'stale' }),
    ).toBe(false);
    expect(
      hasCurrentProxmoxUpdateEvidence({ pendingUpdates: 0, pendingUpdatesStatus: 'checked' }),
    ).toBe(true);
  });
});
