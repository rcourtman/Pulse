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
});
