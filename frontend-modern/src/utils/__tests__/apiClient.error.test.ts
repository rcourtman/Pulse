import { describe, expect, it } from 'vitest';
import { apiErrorFromResponse } from '@/utils/apiClient';

describe('apiClient structured error extraction', () => {
  it('prefers structured JSON error messages over fallback copy', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: 'Canonical API error' }), { status: 400 }),
      'Fallback message',
    );

    expect(error.message).toBe('Canonical API error');
    expect(error.status).toBe(400);
  });

  it('falls back to plain text when the response is not JSON', async () => {
    const error = await apiErrorFromResponse(
      new Response('temporary failure', { status: 500 }),
      'Fallback message',
    );

    expect(error.message).toBe('temporary failure');
    expect(error.status).toBe(500);
  });

  it('preserves structured code and details from canonical API errors', async () => {
    const error = await apiErrorFromResponse(
      new Response(
        JSON.stringify({
          error: 'Failed to connect to VMware vCenter',
          code: 'vmware_connection_failed',
          details: {
            category: 'unsupported_version',
            error: 'VMware vCenter 6.7 is below the supported VI JSON release floor',
          },
        }),
        {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
      'Fallback message',
    );

    expect(error).toMatchObject({
      message: 'Failed to connect to VMware vCenter',
      status: 400,
      code: 'vmware_connection_failed',
      details: {
        category: 'unsupported_version',
        error: 'VMware vCenter 6.7 is below the supported VI JSON release floor',
      },
    });
  });

  it('preserves monitored-system preview payloads from canonical 402 errors', async () => {
    const error = await apiErrorFromResponse(
      new Response(
        JSON.stringify({
          error: 'license_required',
          message: 'Monitored-system limit reached (6/5)',
          feature: 'max_monitored_systems',
          monitored_system_preview: {
            current_count: 5,
            projected_count: 6,
            additional_count: 1,
            limit: 5,
            would_exceed_limit: true,
            effect: 'creates_new',
            current_systems: [],
            projected_systems: [
              {
                name: 'backup',
                type: 'truenas-system',
                status: 'online',
                source: 'truenas',
              },
            ],
            current_system: null,
            projected_system: null,
          },
        }),
        {
          status: 402,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
      'Fallback message',
    );

    expect(error).toMatchObject({
      message: 'Monitored-system limit reached (6/5)',
      status: 402,
      feature: 'max_monitored_systems',
      monitored_system_preview: {
        current_count: 5,
        projected_count: 6,
        would_exceed_limit: true,
      },
    });
  });
});
