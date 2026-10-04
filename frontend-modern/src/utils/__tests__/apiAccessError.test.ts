import { describe, expect, it } from 'vitest';
import { getAPIReadAccessErrorMessage } from '../apiAccessError';

describe('bounded API read access messages', () => {
  it.each([
    [401, 'Sign in again or check your API token.'],
    [403, 'Access denied. Check your permissions and license plan.'],
  ])('recognises final HTTP status %s without exposing diagnostics', (status, message) => {
    expect(
      getAPIReadAccessErrorMessage(Object.assign(new Error('private detail'), { status })),
    ).toBe(message);
    expect(
      getAPIReadAccessErrorMessage({
        status,
        message: '<script>unsafe</script>',
        code: 'arbitrary',
      }),
    ).toBe(message);
  });
  it.each([
    null,
    undefined,
    '403 Forbidden',
    new Error('401 unauthorised'),
    {},
    { status: '403' },
    { status: 400 },
    { status: 402 },
    { status: 404 },
    { status: 429 },
    { status: 500 },
  ])('does not infer permission failure from %j', (error) => {
    expect(getAPIReadAccessErrorMessage(error)).toBeNull();
  });
});
