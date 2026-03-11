import { describe, expect, it, vi } from 'vitest';
import {
  assertAPIResponseOK,
  arrayOrUndefined,
  arrayOrEmpty,
  apiErrorStatus,
  apiResponseStatus,
  coerceTimestampMillis,
  finiteNumberOrUndefined,
  isAPIErrorStatus,
  isAPIResponseStatus,
  normalizeStructuredAPIError,
  objectArrayFieldOrEmpty,
  parseJSONSafe,
  parseJSONTextSafe,
  parseOptionalAPIResponse,
  parseOptionalAPIResponseOrNull,
  parseOptionalJSON,
  parseRequiredAPIResponse,
  parseRequiredAPIResponseOrNull,
  parseRequiredJSON,
  promoteLegacyAlertIdentifier,
  readAPIErrorMessage,
  optionalTrimmedString,
  strictBoolean,
  strictString,
  stringRecordOrUndefined,
  stringArray,
  trimmedString,
} from '@/api/responseUtils';

describe('readAPIErrorMessage', () => {
  it('returns fallback for empty response', async () => {
    const response = new Response('', { status: 500 });
    const result = await readAPIErrorMessage(response, 'Fallback error');
    expect(result).toBe('Fallback error');
  });

  it('returns fallback when response throws', async () => {
    const response = {
      text: vi.fn().mockRejectedValue(new Error('Network error')),
    } as unknown as Response;
    const result = await readAPIErrorMessage(response, 'Fallback error');
    expect(result).toBe('Fallback error');
  });

  it('extracts error field from JSON', async () => {
    const response = new Response(JSON.stringify({ error: 'Something went wrong' }));
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('Something went wrong');
  });

  it('extracts nested error.message from JSON', async () => {
    const response = new Response(JSON.stringify({ error: { message: 'Nested error' } }));
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('Nested error');
  });

  it('extracts message field from JSON', async () => {
    const response = new Response(JSON.stringify({ message: 'API message' }));
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('API message');
  });

  it('prefers error over message field', async () => {
    const response = new Response(JSON.stringify({ error: 'Error text', message: 'Message text' }));
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('Error text');
  });

  it('returns raw text when not valid JSON', async () => {
    const response = new Response('Not a JSON string');
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('Not a JSON string');
  });

  it('trims whitespace from error message', async () => {
    const response = new Response(JSON.stringify({ error: '  Trimmed error  ' }));
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('Trimmed error');
  });

  it('returns raw JSON when error is not a string', async () => {
    const response = new Response(JSON.stringify({ error: { code: 500 } }));
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('{"error":{"code":500}}');
  });

  it('returns raw JSON when message is not a string', async () => {
    const response = new Response(JSON.stringify({ message: { text: 'Nested' } }));
    const result = await readAPIErrorMessage(response, 'Fallback');
    expect(result).toBe('{"message":{"text":"Nested"}}');
  });
});

describe('assertAPIResponseOK', () => {
  it('does nothing for ok responses', async () => {
    await expect(assertAPIResponseOK(new Response('', { status: 200 }), 'Fallback')).resolves.toBe(
      undefined,
    );
  });

  it('throws the canonical parsed error message for non-ok responses', async () => {
    await expect(
      assertAPIResponseOK(new Response(JSON.stringify({ error: 'Bad request' }), { status: 400 }), 'Fallback'),
    ).rejects.toThrow('Bad request');
  });
});

describe('parseRequiredAPIResponse', () => {
  it('asserts ok and parses valid JSON payloads', async () => {
    const response = new Response(JSON.stringify({ ok: true }), { status: 200 });
    await expect(
      parseRequiredAPIResponse<{ ok: boolean }>(response, 'Request failed', 'Parse failed'),
    ).resolves.toEqual({ ok: true });
  });

  it('throws the request error when the response is not ok', async () => {
    const response = new Response(JSON.stringify({ error: 'Request failed' }), { status: 500 });
    await expect(
      parseRequiredAPIResponse(response, 'Request failed', 'Parse failed'),
    ).rejects.toThrow('Request failed');
  });
});

describe('parseOptionalAPIResponse', () => {
  it('asserts ok and returns emptyValue for empty payloads', async () => {
    const response = new Response('', { status: 200 });
    await expect(
      parseOptionalAPIResponse(response, [], 'Request failed', 'Parse failed'),
    ).resolves.toEqual([]);
  });

  it('throws the request error when the response is not ok', async () => {
    const response = new Response(JSON.stringify({ error: 'Request failed' }), { status: 500 });
    await expect(
      parseOptionalAPIResponse(response, [], 'Request failed', 'Parse failed'),
    ).rejects.toThrow('Request failed');
  });
});

describe('parseRequiredAPIResponseOrNull', () => {
  it('returns null when the response matches the null status', async () => {
    const response = new Response('', { status: 404 });
    await expect(
      parseRequiredAPIResponseOrNull(response, 404, 'Request failed', 'Parse failed'),
    ).resolves.toBeNull();
  });

  it('parses valid JSON payloads for non-null statuses', async () => {
    const response = new Response(JSON.stringify({ ok: true }), { status: 200 });
    await expect(
      parseRequiredAPIResponseOrNull<{ ok: boolean }>(
        response,
        404,
        'Request failed',
        'Parse failed',
      ),
    ).resolves.toEqual({ ok: true });
  });
});

describe('parseOptionalAPIResponseOrNull', () => {
  it('returns null when the response matches the null status', async () => {
    const response = new Response('', { status: 404 });
    await expect(
      parseOptionalAPIResponseOrNull(response, 404, 'Request failed', 'Parse failed'),
    ).resolves.toBeNull();
  });

  it('parses valid JSON payloads for non-null statuses', async () => {
    const response = new Response(JSON.stringify({ ok: true }), { status: 200 });
    await expect(
      parseOptionalAPIResponseOrNull<{ ok: boolean }>(
        response,
        404,
        'Request failed',
        'Parse failed',
      ),
    ).resolves.toEqual({ ok: true });
  });
});

describe('parseOptionalJSON', () => {
  it('returns emptyValue for empty response', async () => {
    const response = new Response('');
    const result = await parseOptionalJSON(response, [], 'Parse error');
    expect(result).toEqual([]);
  });

  it('returns emptyValue for whitespace-only response', async () => {
    const response = new Response('   ');
    const result = await parseOptionalJSON(response, null, 'Parse error');
    expect(result).toBeNull();
  });

  it('parses valid JSON', async () => {
    const response = new Response(JSON.stringify({ name: 'test', count: 5 }));
    const result = await parseOptionalJSON(response, {}, 'Parse error');
    expect(result).toEqual({ name: 'test', count: 5 });
  });

  it('parses JSON array', async () => {
    const response = new Response(JSON.stringify([1, 2, 3]));
    const result = await parseOptionalJSON(response, [], 'Parse error');
    expect(result).toEqual([1, 2, 3]);
  });

  it('throws error for invalid JSON', async () => {
    const response = new Response('not valid json');
    await expect(parseOptionalJSON(response, {}, 'Custom parse error')).rejects.toThrow(
      'Custom parse error',
    );
  });
});

describe('parseRequiredJSON', () => {
  it('parses valid JSON payloads', async () => {
    const response = new Response(JSON.stringify({ name: 'test', count: 5 }));
    const result = await parseRequiredJSON(response, 'Parse error');
    expect(result).toEqual({ name: 'test', count: 5 });
  });

  it('throws for empty response bodies', async () => {
    const response = new Response('');
    await expect(parseRequiredJSON(response, 'Required parse error')).rejects.toThrow(
      'Required parse error',
    );
  });

  it('throws for invalid JSON payloads', async () => {
    const response = new Response('not valid json');
    await expect(parseRequiredJSON(response, 'Required parse error')).rejects.toThrow(
      'Required parse error',
    );
  });
});

describe('parseJSONSafe', () => {
  it('parses valid JSON payloads', async () => {
    const response = new Response(JSON.stringify({ ok: true }));
    const result = await parseJSONSafe<{ ok: boolean }>(response);
    expect(result).toEqual({ ok: true });
  });

  it('returns null for empty payloads', async () => {
    const response = new Response('');
    const result = await parseJSONSafe(response);
    expect(result).toBeNull();
  });

  it('returns null for invalid JSON payloads', async () => {
    const response = new Response('not valid json');
    const result = await parseJSONSafe(response);
    expect(result).toBeNull();
  });
});

describe('parseJSONTextSafe', () => {
  it('parses valid JSON text', () => {
    expect(parseJSONTextSafe<{ ok: boolean }>('{"ok":true}')).toEqual({ ok: true });
  });

  it('returns null for empty text', () => {
    expect(parseJSONTextSafe('   ')).toBeNull();
  });

  it('returns null for invalid JSON text', () => {
    expect(parseJSONTextSafe('not valid json')).toBeNull();
  });
});

describe('arrayOrEmpty', () => {
  it('returns arrays unchanged', () => {
    expect(arrayOrEmpty<string>(['a', 'b'])).toEqual(['a', 'b']);
  });

  it('returns empty array for non-array values', () => {
    expect(arrayOrEmpty<string>(null)).toEqual([]);
    expect(arrayOrEmpty<string>({ items: ['a'] })).toEqual([]);
  });
});

describe('arrayOrUndefined', () => {
  it('returns arrays unchanged', () => {
    expect(arrayOrUndefined<string>(['a', 'b'])).toEqual(['a', 'b']);
  });

  it('returns undefined for non-array values', () => {
    expect(arrayOrUndefined<string>(null)).toBeUndefined();
    expect(arrayOrUndefined<string>({ items: ['a'] })).toBeUndefined();
  });
});

describe('objectArrayFieldOrEmpty', () => {
  it('returns object array fields unchanged', () => {
    expect(objectArrayFieldOrEmpty<string>({ items: ['a', 'b'] }, 'items')).toEqual(['a', 'b']);
  });

  it('returns empty array for missing or invalid object fields', () => {
    expect(objectArrayFieldOrEmpty<string>(null, 'items')).toEqual([]);
    expect(objectArrayFieldOrEmpty<string>({ items: 'bad' }, 'items')).toEqual([]);
  });
});

describe('trimmedString', () => {
  it('trims string input and coerces non-null values', () => {
    expect(trimmedString('  value  ')).toBe('value');
    expect(trimmedString(42)).toBe('42');
    expect(trimmedString(null)).toBe('');
  });
});

describe('optionalTrimmedString', () => {
  it('returns undefined for empty normalized strings', () => {
    expect(optionalTrimmedString('   ')).toBeUndefined();
    expect(optionalTrimmedString(null)).toBeUndefined();
  });

  it('returns normalized string when present', () => {
    expect(optionalTrimmedString(' value ')).toBe('value');
  });
});

describe('strictString', () => {
  it('returns strings unchanged and falls back for non-strings', () => {
    expect(strictString('value')).toBe('value');
    expect(strictString(42)).toBe('');
    expect(strictString(42, 'fallback')).toBe('fallback');
  });
});

describe('strictBoolean', () => {
  it('returns booleans unchanged and falls back for non-booleans', () => {
    expect(strictBoolean(true)).toBe(true);
    expect(strictBoolean(false)).toBe(false);
    expect(strictBoolean('true')).toBe(false);
    expect(strictBoolean('true', true)).toBe(true);
  });
});

describe('finiteNumberOrUndefined', () => {
  it('returns finite numbers and rejects invalid values', () => {
    expect(finiteNumberOrUndefined(0)).toBe(0);
    expect(finiteNumberOrUndefined(1.5)).toBe(1.5);
    expect(finiteNumberOrUndefined('1')).toBeUndefined();
    expect(finiteNumberOrUndefined(Number.NaN)).toBeUndefined();
  });
});

describe('coerceTimestampMillis', () => {
  it('keeps finite numbers and parses valid timestamp strings', () => {
    expect(coerceTimestampMillis(1234, 9999)).toBe(1234);
    expect(coerceTimestampMillis(' 2026-01-01T00:00:00Z ', 9999)).toBe(
      Date.parse('2026-01-01T00:00:00Z'),
    );
  });

  it('falls back for invalid timestamp values', () => {
    expect(coerceTimestampMillis('not-a-date', 9999)).toBe(9999);
    expect(coerceTimestampMillis(null, 9999)).toBe(9999);
  });
});

describe('stringArray', () => {
  it('returns only string entries from array values', () => {
    expect(stringArray(['a', 1, 'b', null])).toEqual(['a', 'b']);
  });

  it('returns empty array for non-array values', () => {
    expect(stringArray('a')).toEqual([]);
  });
});

describe('stringRecordOrUndefined', () => {
  it('returns only string fields from object values', () => {
    expect(stringRecordOrUndefined({ a: 'one', b: 2, c: 'three' })).toEqual({
      a: 'one',
      c: 'three',
    });
  });

  it('returns undefined for invalid or empty records', () => {
    expect(stringRecordOrUndefined(null)).toBeUndefined();
    expect(stringRecordOrUndefined({ a: 1, b: false })).toBeUndefined();
  });
});

describe('normalizeStructuredAPIError', () => {
  it('normalizes code, message, and string details through shared helpers', () => {
    expect(
      normalizeStructuredAPIError(
        {
          code: ' invalid_email ',
          message: ' Invalid email format ',
          details: {
            field: 'email',
            ignored: 42,
          },
        },
        400,
      ),
    ).toEqual({
      code: 'invalid_email',
      message: 'Invalid email format',
      details: {
        field: 'email',
      },
    });
  });

  it('falls back when payload fields are missing or invalid', () => {
    expect(normalizeStructuredAPIError(null, 502)).toEqual({
      code: 'request_failed',
      message: 'Request failed (502)',
    });
    expect(normalizeStructuredAPIError({ code: '   ', message: '' }, 503)).toEqual({
      code: 'request_failed',
      message: 'Request failed (503)',
    });
  });
});

describe('promoteLegacyAlertIdentifier', () => {
  it('promotes legacy alert_identifier into canonical alertIdentifier', () => {
    expect(
      promoteLegacyAlertIdentifier({
        id: 'finding-1',
        alert_identifier: ' legacy-alert ',
      }),
    ).toEqual({
      id: 'finding-1',
      alertIdentifier: 'legacy-alert',
    });
  });

  it('prefers canonical alertIdentifier when already present', () => {
    expect(
      promoteLegacyAlertIdentifier({
        id: 'finding-2',
        alertIdentifier: 'canonical-alert',
        alert_identifier: ' legacy-alert ',
      }),
    ).toEqual({
      id: 'finding-2',
      alertIdentifier: 'canonical-alert',
    });
  });
});

describe('apiErrorStatus', () => {
  it('returns null for non-errors and missing statuses', () => {
    expect(apiErrorStatus(null)).toBeNull();
    expect(apiErrorStatus(new Error('boom'))).toBeNull();
  });

  it('returns the canonical numeric status from API errors', () => {
    const error = Object.assign(new Error('Payment Required'), { status: 402 });
    expect(apiErrorStatus(error)).toBe(402);
  });

  it('rejects non-http status values', () => {
    expect(apiErrorStatus({ status: '402' })).toBeNull();
    expect(apiErrorStatus({ status: 99 })).toBeNull();
    expect(apiErrorStatus({ status: 600 })).toBeNull();
  });
});

describe('isAPIErrorStatus', () => {
  it('matches canonical status-bearing API errors', () => {
    const error = Object.assign(new Error('Not Found'), { status: 404 });
    expect(isAPIErrorStatus(error, 404)).toBe(true);
    expect(isAPIErrorStatus(error, 402)).toBe(false);
  });
});

describe('apiResponseStatus', () => {
  it('returns null for missing or invalid response statuses', () => {
    expect(apiResponseStatus(null)).toBeNull();
    expect(apiResponseStatus({})).toBeNull();
    expect(apiResponseStatus({ status: '404' })).toBeNull();
    expect(apiResponseStatus({ status: 99 })).toBeNull();
  });

  it('returns canonical numeric response status', () => {
    expect(apiResponseStatus(new Response('', { status: 404 }))).toBe(404);
  });
});

describe('isAPIResponseStatus', () => {
  it('matches canonical status-bearing responses', () => {
    const response = new Response(null, { status: 204 });
    expect(isAPIResponseStatus(response, 204)).toBe(true);
    expect(isAPIResponseStatus(response, 404)).toBe(false);
  });
});
