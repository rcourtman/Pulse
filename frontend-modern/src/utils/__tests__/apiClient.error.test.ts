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
});
