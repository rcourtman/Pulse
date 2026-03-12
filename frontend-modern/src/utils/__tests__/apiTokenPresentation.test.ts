import { describe, expect, it } from 'vitest';
import {
  getAPITokenGenerateErrorMessage,
  getAPITokensLoadErrorMessage,
  getAPITokenRevokeErrorMessage,
} from '@/utils/apiTokenPresentation';

describe('apiTokenPresentation', () => {
  it('returns canonical API token error copy', () => {
    expect(getAPITokensLoadErrorMessage()).toBe('Failed to load API tokens');
    expect(getAPITokenGenerateErrorMessage()).toBe('Failed to generate API token');
    expect(getAPITokenRevokeErrorMessage()).toBe('Failed to revoke API token');
  });

  it('surfaces token scope denial copy for generate failures', () => {
    const error = Object.assign(
      new Error('Cannot grant scope "monitoring:read": your token does not have this scope'),
      { status: 403 },
    );

    expect(getAPITokenGenerateErrorMessage(error)).toBe(
      'Cannot grant scope "monitoring:read": your token does not have this scope',
    );
  });

  it('surfaces required scope when middleware returns missing_scope', () => {
    const error = Object.assign(new Error('missing_scope'), {
      status: 403,
      requiredScope: 'settings:write',
    });

    expect(getAPITokenGenerateErrorMessage(error)).toBe(
      'This token is missing the required scope: Settings (write) (settings:write).',
    );
  });
});
