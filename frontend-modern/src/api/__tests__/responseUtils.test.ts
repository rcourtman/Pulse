import { describe, expect, it } from 'vitest';
import { apiErrorCode, apiErrorDetailField } from '@/api/responseUtils';

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
});
