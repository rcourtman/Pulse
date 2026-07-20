import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import {
  acknowledgeFinding,
  snoozeFinding,
  dismissFinding,
  setFindingNote,
  getPatrolAutonomySettings,
  getInvestigation,
  getInvestigationMessages,
  reinvestigateFinding,
} from '@/api/patrol';
import { apiFetchJSON } from '@/utils/apiClient';

describe('patrol api — uncovered branch coverage', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchJSONMock.mockResolvedValue({ success: true, message: 'ok' } as any);
  });

  describe('acknowledgeFinding', () => {
    it('POSTs the canonical finding_id body to /api/ai/patrol/acknowledge', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        success: true,
        message: 'Finding acknowledged',
      } as any);

      const result = await acknowledgeFinding('finding-ack-1');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/acknowledge', {
        method: 'POST',
        body: JSON.stringify({ finding_id: 'finding-ack-1' }),
      });
      expect(result).toEqual({ success: true, message: 'Finding acknowledged' });
    });

    it('passes the finding id through verbatim (no trimming / encoding at the client)', async () => {
      await acknowledgeFinding('  raw-id-with-spaces  ');

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/acknowledge', {
        method: 'POST',
        body: JSON.stringify({ finding_id: '  raw-id-with-spaces  ' }),
      });
    });

    it('propagates transport rejections (non-ok response) to the caller', async () => {
      const transportError = new Error('Request failed with status 404');
      apiFetchJSONMock.mockRejectedValueOnce(transportError);

      await expect(acknowledgeFinding('missing')).rejects.toBe(transportError);
    });
  });

  describe('snoozeFinding', () => {
    it('POSTs finding_id + duration_hours to /api/ai/patrol/snooze', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        success: true,
        message: 'Snoozed for 24h',
      } as any);

      const result = await snoozeFinding('finding-snooze', 24);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/snooze', {
        method: 'POST',
        body: JSON.stringify({ finding_id: 'finding-snooze', duration_hours: 24 }),
      });
      expect(result).toEqual({ success: true, message: 'Snoozed for 24h' });
    });

    it('serializes the 7-day duration (168h) without unit conversion', async () => {
      await snoozeFinding('finding-7d', 168);

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/snooze', {
        method: 'POST',
        body: JSON.stringify({ finding_id: 'finding-7d', duration_hours: 168 }),
      });
    });

    it('passes fractional durations through untouched (no integer rounding at the client)', async () => {
      // The client does no validation/normalization on duration_hours — it is
      // serialized verbatim. Pinning this so a future "round to integer"
      // refactor surfaces here rather than silently changing wire shape.
      await snoozeFinding('finding-half', 0.5);

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/snooze', {
        method: 'POST',
        body: JSON.stringify({ finding_id: 'finding-half', duration_hours: 0.5 }),
      });
    });

    it('passes a zero duration through (the backend, not the client, owns validation)', async () => {
      await snoozeFinding('finding-zero', 0);

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/snooze', {
        method: 'POST',
        body: JSON.stringify({ finding_id: 'finding-zero', duration_hours: 0 }),
      });
    });
  });

  describe('dismissFinding', () => {
    it('omits the optional note field entirely when note is not supplied (JSON.stringify drops undefined)', async () => {
      // Body shape with note===undefined: { finding_id, reason, note: undefined }
      // JSON.stringify elides undefined-valued keys, so the wire body is just
      // { finding_id, reason }. This is the absent-note branch.
      apiFetchJSONMock.mockResolvedValueOnce({
        success: true,
        message: 'Dismissed',
      } as any);

      const result = await dismissFinding('finding-1', 'not_an_issue');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/dismiss', {
        method: 'POST',
        body: JSON.stringify({ finding_id: 'finding-1', reason: 'not_an_issue' }),
      });
      expect(result).toEqual({ success: true, message: 'Dismissed' });
    });

    it('includes note as a real string field when supplied (present-note branch)', async () => {
      await dismissFinding('finding-2', 'expected_behavior', 'planned maintenance window');

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/dismiss', {
        method: 'POST',
        body: JSON.stringify({
          finding_id: 'finding-2',
          reason: 'expected_behavior',
          note: 'planned maintenance window',
        }),
      });
    });

    it('keeps an empty-string note on the wire (distinct from absent note)', async () => {
      // Empty string is a value, not undefined — JSON.stringify keeps `"note":""`.
      // The two branches must produce distinct wire shapes.
      await dismissFinding('finding-3', 'will_fix_later', '');

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/dismiss', {
        method: 'POST',
        body: JSON.stringify({
          finding_id: 'finding-3',
          reason: 'will_fix_later',
          note: '',
        }),
      });
    });

    it('round-trips the will_fix_later reason verbatim (the only reason with a remind_at side-effect)', async () => {
      await dismissFinding('finding-wfl', 'will_fix_later', 'next change window');

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/dismiss', {
        method: 'POST',
        body: JSON.stringify({
          finding_id: 'finding-wfl',
          reason: 'will_fix_later',
          note: 'next change window',
        }),
      });
    });
  });

  describe('setFindingNote', () => {
    it('POSTs finding_id + note to /api/ai/patrol/findings/note', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        success: true,
        message: 'Note saved',
      } as any);

      const result = await setFindingNote('finding-note', 'manual triage: low priority');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/findings/note', {
        method: 'POST',
        body: JSON.stringify({
          finding_id: 'finding-note',
          note: 'manual triage: low priority',
        }),
      });
      expect(result).toEqual({ success: true, message: 'Note saved' });
    });

    it('sends an empty-string note to clear an existing note (the documented clear branch)', async () => {
      // Per the JSDoc: "empty string to clear". The client does NOT convert
      // "" into an absent field — it sends note:"" so the backend can
      // distinguish "clear my note" from "I never sent one".
      await setFindingNote('finding-clear', '');

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/findings/note', {
        method: 'POST',
        body: JSON.stringify({ finding_id: 'finding-clear', note: '' }),
      });
    });
  });

  describe('getPatrolAutonomySettings', () => {
    it('GETs /api/ai/patrol/autonomy and returns the parsed settings verbatim', async () => {
      const payload = {
        autonomy_level: 'approval',
        requested_autonomy_level: 'full',
        effective_autonomy_level: 'approval',
        full_mode_unlocked: false,
        autopilot_acknowledgement: {
          code: 'acknowledgement_required',
          active: false,
          currentVersion: 3,
          acceptedScope: [],
          acceptedLimits: {
            policyAllowlistRequired: true,
            emergencyStopHonored: true,
            approvalFloorsHonored: true,
            verificationReconciledWhenSupported: true,
            evidenceClassDisclosed: true,
            inconclusiveOutcomeAllowed: false,
            executionSuccessIsNotOutcomeTruth: true,
          },
        },
        investigation_budget: 15,
        investigation_timeout_sec: 300,
      };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await getPatrolAutonomySettings();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/autonomy');
      expect(result).toEqual(payload);
      // The numeric budget/timeout fields must round-trip unchanged so the
      // slider UI does not silently clamp on the way back from the server.
      expect(result.investigation_budget).toBe(15);
      expect(result.investigation_timeout_sec).toBe(300);
    });
  });

  describe('getInvestigation', () => {
    it('GETs /api/ai/findings/:id/investigation and returns the parsed record', async () => {
      const investigation = {
        id: 'inv-1',
        finding_id: 'finding-inv',
        session_id: 'sess-1',
        status: 'completed',
        started_at: '2026-07-18T00:00:00Z',
        completed_at: '2026-07-18T00:05:00Z',
        turn_count: 4,
        outcome: 'fix_verified',
      };
      apiFetchJSONMock.mockResolvedValueOnce(investigation as any);

      const result = await getInvestigation('finding-inv');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/findings/finding-inv/investigation');
      expect(result).toEqual(investigation);
      expect(result.outcome).toBe('fix_verified');
    });

    it('URL-encodes finding ids that contain path separators', async () => {
      // The finding id is interpolated into the path with encodeURIComponent.
      // A slash in the id MUST be percent-encoded, otherwise it would split
      // the path into a different route.
      await getInvestigation('tenant/find%');

      expect(apiFetchJSONMock).toHaveBeenLastCalledWith(
        '/api/ai/findings/tenant%2Ffind%25/investigation',
      );
    });
  });

  describe('getInvestigationMessages', () => {
    it('GETs /api/ai/findings/:id/investigation/messages and returns the parsed envelope', async () => {
      const envelope = {
        investigation_id: 'inv-1',
        session_id: 'sess-1',
        messages: [
          {
            id: 'msg-1',
            role: 'user',
            content: 'why is disk full?',
            timestamp: '2026-07-18T00:00:01Z',
          },
          {
            id: 'msg-2',
            role: 'assistant',
            content: 'logs in /var/log grew 40GB',
            reasoning_content: 'checked du output',
            timestamp: '2026-07-18T00:00:05Z',
          },
        ],
      };
      apiFetchJSONMock.mockResolvedValueOnce(envelope as any);

      const result = await getInvestigationMessages('finding-msg');

      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/ai/findings/finding-msg/investigation/messages',
      );
      expect(result).toEqual(envelope);
      expect(result.messages).toHaveLength(2);
      expect(result.messages[1]?.reasoning_content).toBe('checked du output');
    });

    it('URL-encodes the finding id segment (separate from the messages suffix)', async () => {
      await getInvestigationMessages('org/find#1');

      // '#' and '/' must both be encoded so neither splits the path nor
      // starts a fragment.
      expect(apiFetchJSONMock).toHaveBeenLastCalledWith(
        '/api/ai/findings/org%2Ffind%231/investigation/messages',
      );
    });
  });

  describe('reinvestigateFinding', () => {
    it('POSTs to /api/ai/findings/:id/reinvestigate with no body', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        success: true,
        message: 'Reinvestigation queued',
      } as any);

      const result = await reinvestigateFinding('finding-re');

      // Note: no body key is set — this is the no-body branch (unlike the
      // other finding mutations which always carry a JSON body).
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/findings/finding-re/reinvestigate', {
        method: 'POST',
      });
      expect(result).toEqual({ success: true, message: 'Reinvestigation queued' });
    });

    it('URL-encodes the finding id before interpolating into the path', async () => {
      await reinvestigateFinding('tenant/finding 1');

      // space -> %20, slash -> %2F
      expect(apiFetchJSONMock).toHaveBeenLastCalledWith(
        '/api/ai/findings/tenant%2Ffinding%201/reinvestigate',
        { method: 'POST' },
      );
    });

    it('propagates transport errors (e.g. 409 conflict from an in-flight reinvestigation)', async () => {
      const conflict = new Error('Request failed with status 409');
      apiFetchJSONMock.mockRejectedValueOnce(conflict);

      await expect(reinvestigateFinding('in-flight')).rejects.toBe(conflict);
    });
  });
});
