import { describe, expect, it } from 'vitest';
import {
  apiErrorCode,
  apiErrorDetailField,
  apiErrorMonitoredSystemPreview,
} from '@/api/responseUtils';

describe('responseUtils structured API errors', () => {
  it('reads canonical code and detail fields from shared API errors', () => {
    const error = {
      code: 'vmware_connection_failed',
      details: {
        category: 'unsupported_version',
        error: 'VMware vCenter 6.7 is below the supported VI JSON release floor',
      },
    };

    expect(apiErrorCode(error)).toBe('vmware_connection_failed');
    expect(apiErrorDetailField(error, 'category')).toBe('unsupported_version');
    expect(apiErrorDetailField(error, 'error')).toBe(
      'VMware vCenter 6.7 is below the supported VI JSON release floor',
    );
  });

  it('normalizes monitored-system preview payloads from shared API errors', () => {
    const error = {
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
    };

    expect(apiErrorMonitoredSystemPreview(error)).toMatchObject({
      current_count: 5,
      projected_count: 6,
      would_exceed_limit: true,
      projected_systems: [
        expect.objectContaining({
          name: 'backup',
          source: 'truenas',
        }),
      ],
    });
  });
});
