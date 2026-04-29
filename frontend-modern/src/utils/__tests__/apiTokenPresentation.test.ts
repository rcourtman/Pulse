import { describe, expect, it } from 'vitest';
import {
  getAPITokenGenerateErrorMessage,
  getAPITokenManagementLocationMessage,
  getAPITokenRevealSettingsNote,
  getAPITokensLoadErrorMessage,
  getAPITokenRevokeErrorMessage,
} from '@/utils/apiTokenPresentation';

describe('apiTokenPresentation', () => {
  it('returns canonical API token error copy', () => {
    expect(getAPITokensLoadErrorMessage()).toBe('Unable to load API tokens.');
    expect(getAPITokenGenerateErrorMessage()).toBe('Unable to generate the API token.');
    expect(getAPITokenRevokeErrorMessage()).toBe('Unable to revoke the API token.');
  });

  it('returns canonical API token settings location copy', () => {
    expect(getAPITokenManagementLocationMessage()).toBe(
      'Create or rotate API tokens in Settings → API Access.',
    );
    expect(getAPITokenRevealSettingsNote()).toBe(
      'Copy this token now. You can reopen this dialog from Settings → API Access while this page stays open.',
    );
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
