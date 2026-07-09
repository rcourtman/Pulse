import { describe, expect, it } from 'vitest';
import {
  apiErrorDetails,
  apiErrorStatus,
  apiResponseStatus,
  arrayOrEmpty,
  arrayOrUndefined,
  coerceTimestampMillis,
  finiteNumberOrUndefined,
  isAPIErrorStatus,
  isAPIResponseStatus,
  normalizeStructuredAPIError,
  objectArrayFieldOrEmpty,
  optionalTrimmedString,
  parseJSONTextSafe,
  promoteLegacyAlertIdentifier,
  strictBoolean,
  strictString,
  stringArray,
  stringRecordOrUndefined,
  trimmedString,
} from '@/api/responseUtils';

describe('responseUtils pure coercion and guards', () => {
  describe('apiErrorStatus', () => {
    it('returns the integer status within the 100-599 HTTP range inclusive', () => {
      expect(apiErrorStatus({ status: 404 })).toBe(404);
      expect(apiErrorStatus({ status: 100 })).toBe(100);
      expect(apiErrorStatus({ status: 599 })).toBe(599);
    });

    it('returns null below the 100 floor and above the 599 ceiling', () => {
      expect(apiErrorStatus({ status: 99 })).toBeNull();
      expect(apiErrorStatus({ status: 600 })).toBeNull();
      expect(apiErrorStatus({ status: 0 })).toBeNull();
    });

    it('rejects non-integer and numeric-string statuses', () => {
      expect(apiErrorStatus({ status: 404.5 })).toBeNull();
      expect(apiErrorStatus({ status: '404' })).toBeNull();
      expect(apiErrorStatus({ status: NaN })).toBeNull();
      expect(apiErrorStatus({ status: Infinity })).toBeNull();
      expect(apiErrorStatus({ status: -Infinity })).toBeNull();
    });

    it('returns null when status is missing and for non-object inputs', () => {
      expect(apiErrorStatus({})).toBeNull();
      expect(apiErrorStatus({ status: null })).toBeNull();
      expect(apiErrorStatus({ status: undefined })).toBeNull();
      expect(apiErrorStatus(null)).toBeNull();
      expect(apiErrorStatus(undefined)).toBeNull();
      expect(apiErrorStatus('error')).toBeNull();
      expect(apiErrorStatus(404)).toBeNull();
      expect(apiErrorStatus(true)).toBeNull();
    });
  });

  describe('apiErrorDetails', () => {
    it('returns the full details record when all entries are non-empty strings', () => {
      expect(apiErrorDetails({ details: { a: '1', b: '2' } })).toEqual({ a: '1', b: '2' });
    });

    it('trims both keys and values and keeps the normalized pair', () => {
      expect(apiErrorDetails({ details: { '  key  ': '  val  ' } })).toEqual({ key: 'val' });
    });

    it('drops entries whose key or value is empty/whitespace or non-string', () => {
      expect(apiErrorDetails({ details: { a: '1', b: '   ', c: 2, d: null } })).toEqual({ a: '1' });
      expect(apiErrorDetails({ details: { '   ': 'val' } })).toBeNull();
      expect(apiErrorDetails({ details: { a: 1, b: 2 } })).toBeNull();
    });

    it('returns null for empty details objects, missing details, and non-object inputs', () => {
      expect(apiErrorDetails({ details: {} })).toBeNull();
      expect(apiErrorDetails({})).toBeNull();
      expect(apiErrorDetails({ details: null })).toBeNull();
      expect(apiErrorDetails({ details: 'string' })).toBeNull();
      expect(apiErrorDetails({ details: [] })).toBeNull();
      expect(apiErrorDetails(null)).toBeNull();
      expect(apiErrorDetails('error')).toBeNull();
    });
  });

  describe('apiResponseStatus', () => {
    it('returns the integer status within the 100-599 range inclusive', () => {
      expect(apiResponseStatus({ status: 200 })).toBe(200);
      expect(apiResponseStatus({ status: 100 })).toBe(100);
      expect(apiResponseStatus({ status: 599 })).toBe(599);
    });

    it('returns null outside the 100-599 range', () => {
      expect(apiResponseStatus({ status: 99 })).toBeNull();
      expect(apiResponseStatus({ status: 600 })).toBeNull();
    });

    it('rejects non-integer and non-number statuses', () => {
      expect(apiResponseStatus({ status: 200.5 })).toBeNull();
      expect(apiResponseStatus({ status: '200' })).toBeNull();
      expect(apiResponseStatus({ status: NaN })).toBeNull();
      expect(apiResponseStatus({ status: true })).toBeNull();
    });

    it('returns null for missing status and non-object inputs', () => {
      expect(apiResponseStatus({})).toBeNull();
      expect(apiResponseStatus({ status: null })).toBeNull();
      expect(apiResponseStatus(null)).toBeNull();
      expect(apiResponseStatus(undefined)).toBeNull();
      expect(apiResponseStatus(200 as unknown as Parameters<typeof apiResponseStatus>[0])).toBeNull();
    });
  });

  describe('isAPIResponseStatus', () => {
    it('returns true only when the response status matches a valid expected status', () => {
      expect(isAPIResponseStatus({ status: 200 }, 200)).toBe(true);
      expect(isAPIResponseStatus({ status: 100 }, 100)).toBe(true);
      expect(isAPIResponseStatus({ status: 599 }, 599)).toBe(true);
    });

    it('returns false on status mismatch', () => {
      expect(isAPIResponseStatus({ status: 200 }, 404)).toBe(false);
    });

    it('returns false when the response status is outside the valid range', () => {
      expect(isAPIResponseStatus({ status: 99 }, 99)).toBe(false);
      expect(isAPIResponseStatus({ status: 600 }, 600)).toBe(false);
      expect(isAPIResponseStatus({ status: '200' }, 200)).toBe(false);
    });

    it('returns false for null/undefined responses', () => {
      expect(isAPIResponseStatus(null, 200)).toBe(false);
      expect(isAPIResponseStatus(undefined, 200)).toBe(false);
    });
  });

  describe('isAPIErrorStatus', () => {
    it('returns true when the error status matches a valid expected status', () => {
      expect(isAPIErrorStatus({ status: 404 }, 404)).toBe(true);
      expect(isAPIErrorStatus({ status: 503 }, 503)).toBe(true);
    });

    it('returns false on mismatch and on invalid statuses', () => {
      expect(isAPIErrorStatus({ status: 404 }, 500)).toBe(false);
      expect(isAPIErrorStatus({ status: '404' }, 404)).toBe(false);
      expect(isAPIErrorStatus({ status: 99 }, 99)).toBe(false);
    });

    it('returns false for non-object errors', () => {
      expect(isAPIErrorStatus(null, 404)).toBe(false);
      expect(isAPIErrorStatus('error', 404)).toBe(false);
    });
  });

  describe('trimmedString', () => {
    it('trims surrounding whitespace from string inputs', () => {
      expect(trimmedString('  hello  ')).toBe('hello');
      expect(trimmedString('hello')).toBe('hello');
      expect(trimmedString('\t\n x \t\n')).toBe('x');
    });

    it('returns an empty string for empty or whitespace-only strings', () => {
      expect(trimmedString('')).toBe('');
      expect(trimmedString('   ')).toBe('');
    });

    it('returns an empty string for null and undefined via loose equality', () => {
      expect(trimmedString(null)).toBe('');
      expect(trimmedString(undefined)).toBe('');
    });

    it('coerces non-string, non-nullish values via String() then trims', () => {
      expect(trimmedString(0)).toBe('0');
      expect(trimmedString(42)).toBe('42');
      expect(trimmedString(-1.5)).toBe('-1.5');
      expect(trimmedString(true)).toBe('true');
      expect(trimmedString(false)).toBe('false');
      expect(trimmedString(NaN)).toBe('NaN');
      expect(trimmedString([1, 2])).toBe('1,2');
      expect(trimmedString({ a: 1 })).toBe('[object Object]');
    });
  });

  describe('optionalTrimmedString', () => {
    it('returns the trimmed string when it is non-empty', () => {
      expect(optionalTrimmedString('  hello  ')).toBe('hello');
      expect(optionalTrimmedString('hello')).toBe('hello');
    });

    it('returns undefined for empty, whitespace-only, nullish, and zero-numeric inputs that coerce to empty', () => {
      expect(optionalTrimmedString('')).toBeUndefined();
      expect(optionalTrimmedString('   ')).toBeUndefined();
      expect(optionalTrimmedString(null)).toBeUndefined();
      expect(optionalTrimmedString(undefined)).toBeUndefined();
    });

    it('returns the coerced string for truthy non-string values', () => {
      expect(optionalTrimmedString(0)).toBe('0');
      expect(optionalTrimmedString(42)).toBe('42');
      expect(optionalTrimmedString(false)).toBe('false');
    });
  });

  describe('strictString', () => {
    it('returns the original string when value is a string (including empty string)', () => {
      expect(strictString('hello')).toBe('hello');
      expect(strictString('')).toBe('');
    });

    it('returns the default empty fallback for non-strings', () => {
      expect(strictString(42)).toBe('');
      expect(strictString(null)).toBe('');
      expect(strictString(undefined)).toBe('');
      expect(strictString(true)).toBe('');
    });

    it('returns the provided fallback for non-strings', () => {
      expect(strictString(42, 'fallback')).toBe('fallback');
      expect(strictString(null, 'fallback')).toBe('fallback');
    });
  });

  describe('strictBoolean', () => {
    it('returns the original boolean for true and false', () => {
      expect(strictBoolean(true)).toBe(true);
      expect(strictBoolean(false)).toBe(false);
    });

    it('returns the default false fallback for non-booleans', () => {
      expect(strictBoolean('true')).toBe(false);
      expect(strictBoolean(1)).toBe(false);
      expect(strictBoolean(0)).toBe(false);
      expect(strictBoolean(null)).toBe(false);
      expect(strictBoolean(undefined)).toBe(false);
    });

    it('returns the provided fallback for non-booleans', () => {
      expect(strictBoolean('true', true)).toBe(true);
      expect(strictBoolean(1, false)).toBe(false);
    });
  });

  describe('finiteNumberOrUndefined', () => {
    it('returns finite numbers including zero and negatives', () => {
      expect(finiteNumberOrUndefined(42)).toBe(42);
      expect(finiteNumberOrUndefined(0)).toBe(0);
      expect(finiteNumberOrUndefined(-1.5)).toBe(-1.5);
    });

    it('returns undefined for NaN and infinities', () => {
      expect(finiteNumberOrUndefined(NaN)).toBeUndefined();
      expect(finiteNumberOrUndefined(Infinity)).toBeUndefined();
      expect(finiteNumberOrUndefined(-Infinity)).toBeUndefined();
    });

    it('returns undefined for non-number inputs', () => {
      expect(finiteNumberOrUndefined('42')).toBeUndefined();
      expect(finiteNumberOrUndefined(null)).toBeUndefined();
      expect(finiteNumberOrUndefined(undefined)).toBeUndefined();
      expect(finiteNumberOrUndefined(true)).toBeUndefined();
    });
  });

  describe('coerceTimestampMillis', () => {
    it('returns finite numeric inputs directly', () => {
      expect(coerceTimestampMillis(1000, 0)).toBe(1000);
      expect(coerceTimestampMillis(0, 999)).toBe(0);
      expect(coerceTimestampMillis(-100, 999)).toBe(-100);
    });

    it('parses ISO 8601 strings into epoch milliseconds', () => {
      expect(coerceTimestampMillis('2024-01-01T00:00:00Z', 0)).toBe(1704067200000);
    });

    it('trims whitespace around parseable date strings', () => {
      expect(coerceTimestampMillis('  2024-01-01T00:00:00Z  ', 0)).toBe(1704067200000);
    });

    it('returns the fallback for NaN, Infinity, and non-numeric inputs', () => {
      const fallback = 12345;
      expect(coerceTimestampMillis(NaN, fallback)).toBe(fallback);
      expect(coerceTimestampMillis(Infinity, fallback)).toBe(fallback);
      expect(coerceTimestampMillis('not-a-date', fallback)).toBe(fallback);
      expect(coerceTimestampMillis('', fallback)).toBe(fallback);
      expect(coerceTimestampMillis('   ', fallback)).toBe(fallback);
      expect(coerceTimestampMillis(null, fallback)).toBe(fallback);
      expect(coerceTimestampMillis(undefined, fallback)).toBe(fallback);
      expect(coerceTimestampMillis(false, fallback)).toBe(fallback);
    });
  });

  describe('stringArray', () => {
    it('returns only the string elements preserving order', () => {
      expect(stringArray(['a', 1, 'b', true, null, 'c'])).toEqual(['a', 'b', 'c']);
      expect(stringArray(['a', 'b'])).toEqual(['a', 'b']);
    });

    it('keeps empty-string elements', () => {
      expect(stringArray(['', 'a', ''])).toEqual(['', 'a', '']);
    });

    it('returns an empty array for non-arrays', () => {
      expect(stringArray('abc')).toEqual([]);
      expect(stringArray(null)).toEqual([]);
      expect(stringArray(undefined)).toEqual([]);
      expect(stringArray({ a: 'b' })).toEqual([]);
      expect(stringArray(42)).toEqual([]);
    });
  });

  describe('stringRecordOrUndefined', () => {
    it('returns the record when at least one value is a string', () => {
      expect(stringRecordOrUndefined({ a: '1', b: '2' })).toEqual({ a: '1', b: '2' });
    });

    it('drops non-string values, keeping string values (including empty strings)', () => {
      expect(stringRecordOrUndefined({ a: '1', b: 2, c: null })).toEqual({ a: '1' });
      expect(stringRecordOrUndefined({ a: '', b: '2' })).toEqual({ a: '', b: '2' });
    });

    it('returns undefined when no string values remain', () => {
      expect(stringRecordOrUndefined({ a: 1, b: 2 })).toBeUndefined();
      expect(stringRecordOrUndefined({})).toBeUndefined();
    });

    it('returns undefined for non-object inputs', () => {
      expect(stringRecordOrUndefined(null)).toBeUndefined();
      expect(stringRecordOrUndefined(undefined)).toBeUndefined();
      expect(stringRecordOrUndefined('str')).toBeUndefined();
      expect(stringRecordOrUndefined(42)).toBeUndefined();
    });

    it('coerces arrays into their index-keyed string entries (current behavior)', () => {
      expect(stringRecordOrUndefined(['a', 'b'])).toEqual({ 0: 'a', 1: 'b' });
    });
  });

  describe('normalizeStructuredAPIError', () => {
    it('returns canonical code and message for a well-formed payload', () => {
      expect(normalizeStructuredAPIError({ code: 'not_found', message: 'gone' }, 404)).toEqual({
        code: 'not_found',
        message: 'gone',
        details: undefined,
      });
    });

    it('trims whitespace from code and message', () => {
      expect(normalizeStructuredAPIError({ code: '  x  ', message: '  y  ' }, 500)).toEqual({
        code: 'x',
        message: 'y',
        details: undefined,
      });
    });

    it('uses the provided fallbackCode and fallbackStatus-derived message', () => {
      expect(normalizeStructuredAPIError({}, 503, 'custom_code')).toEqual({
        code: 'custom_code',
        message: 'Request failed (503)',
        details: undefined,
      });
    });

    it('defaults fallbackCode to request_failed when omitted', () => {
      expect(normalizeStructuredAPIError(null, 500)).toEqual({
        code: 'request_failed',
        message: 'Request failed (500)',
        details: undefined,
      });
      expect(normalizeStructuredAPIError('str', 500)).toEqual({
        code: 'request_failed',
        message: 'Request failed (500)',
        details: undefined,
      });
    });

    it('falls back when code and message are empty or whitespace', () => {
      expect(normalizeStructuredAPIError({ code: '', message: '   ' }, 500, 'c')).toEqual({
        code: 'c',
        message: 'Request failed (500)',
        details: undefined,
      });
    });

    it('exposes a string details record when present and omits nothing otherwise', () => {
      const withDetails = normalizeStructuredAPIError(
        { code: 'x', message: 'y', details: { field: 'bad' } },
        400,
      );
      expect(withDetails.details).toEqual({ field: 'bad' });

      const withoutDetails = normalizeStructuredAPIError({ code: 'x', message: 'y' }, 400);
      expect(withoutDetails.details).toBeUndefined();
      expect('details' in withoutDetails).toBe(true);
    });
  });

  describe('promoteLegacyAlertIdentifier', () => {
    it('promotes snake_case alert_identifier to camelCase alertIdentifier', () => {
      const result = promoteLegacyAlertIdentifier({ alert_identifier: 'id-1', foo: 'bar' });
      expect(result).toEqual({ alertIdentifier: 'id-1', foo: 'bar' });
      expect('alert_identifier' in result).toBe(false);
    });

    it('keeps existing camelCase alertIdentifier and drops the snake_case key', () => {
      const result = promoteLegacyAlertIdentifier({
        alertIdentifier: 'camel',
        alert_identifier: 'snake',
        foo: 'bar',
      });
      expect(result).toEqual({ alertIdentifier: 'camel', foo: 'bar' });
      expect('alert_identifier' in result).toBe(false);
    });

    it('falls back to snake_case when camelCase is whitespace', () => {
      const result = promoteLegacyAlertIdentifier({
        alertIdentifier: '   ',
        alert_identifier: 'real-id',
      });
      expect(result).toEqual({ alertIdentifier: 'real-id' });
    });

    it('preserves camelCase and removes snake_case when snake is whitespace', () => {
      const result = promoteLegacyAlertIdentifier({
        alertIdentifier: 'real-id',
        alert_identifier: '   ',
      });
      expect(result).toEqual({ alertIdentifier: 'real-id' });
    });

    it('returns the record unchanged when no identifier fields are present', () => {
      const result = promoteLegacyAlertIdentifier({ foo: 'bar', count: 3 });
      expect(result).toEqual({ foo: 'bar', count: 3 });
    });

    it('removes snake_case key and leaves the existing whitespace camelCase as-is when both are blank', () => {
      const result = promoteLegacyAlertIdentifier({
        alertIdentifier: '   ',
        alert_identifier: '   ',
        foo: 'bar',
      });
      expect(result).toEqual({ alertIdentifier: '   ', foo: 'bar' });
      expect('alert_identifier' in result).toBe(false);
    });
  });

  describe('parseJSONTextSafe', () => {
    it('parses valid JSON objects and arrays', () => {
      expect(parseJSONTextSafe('{"a":1}')).toEqual({ a: 1 });
      expect(parseJSONTextSafe('[1,2,3]')).toEqual([1, 2, 3]);
    });

    it('parses JSON primitives including null, booleans, and numbers', () => {
      expect(parseJSONTextSafe('null')).toBeNull();
      expect(parseJSONTextSafe('true')).toBe(true);
      expect(parseJSONTextSafe('false')).toBe(false);
      expect(parseJSONTextSafe('42')).toBe(42);
    });

    it('parses JSON with surrounding whitespace', () => {
      expect(parseJSONTextSafe('  {"a":1}  ')).toEqual({ a: 1 });
    });

    it('returns null for empty and whitespace-only input', () => {
      expect(parseJSONTextSafe('')).toBeNull();
      expect(parseJSONTextSafe('   ')).toBeNull();
      expect(parseJSONTextSafe('\t\n')).toBeNull();
    });

    it('returns null for malformed JSON without throwing', () => {
      expect(parseJSONTextSafe('not json')).toBeNull();
      expect(parseJSONTextSafe('{bad}')).toBeNull();
      expect(parseJSONTextSafe('{')).toBeNull();
    });
  });

  describe('arrayOrEmpty', () => {
    it('returns arrays unchanged including empty arrays', () => {
      expect(arrayOrEmpty([1, 2, 3])).toEqual([1, 2, 3]);
      expect(arrayOrEmpty([])).toEqual([]);
    });

    it('returns a fresh empty array for non-arrays', () => {
      expect(arrayOrEmpty('str')).toEqual([]);
      expect(arrayOrEmpty(null)).toEqual([]);
      expect(arrayOrEmpty(undefined)).toEqual([]);
      expect(arrayOrEmpty({})).toEqual([]);
      expect(arrayOrEmpty(0)).toEqual([]);
    });

    it('returns a distinct empty array reference each time it coerces', () => {
      const a = arrayOrEmpty(null);
      const b = arrayOrEmpty(null);
      expect(a).not.toBe(b);
      expect(a).toEqual([]);
    });
  });

  describe('arrayOrUndefined', () => {
    it('returns arrays unchanged including empty arrays', () => {
      expect(arrayOrUndefined([1, 2])).toEqual([1, 2]);
      expect(arrayOrUndefined([])).toEqual([]);
    });

    it('returns undefined for non-arrays', () => {
      expect(arrayOrUndefined('str')).toBeUndefined();
      expect(arrayOrUndefined(null)).toBeUndefined();
      expect(arrayOrUndefined(undefined)).toBeUndefined();
      expect(arrayOrUndefined({})).toBeUndefined();
      expect(arrayOrUndefined(0)).toBeUndefined();
    });
  });

  describe('objectArrayFieldOrEmpty', () => {
    it('returns the array at the given field when present', () => {
      expect(objectArrayFieldOrEmpty({ items: [1, 2] }, 'items')).toEqual([1, 2]);
      expect(objectArrayFieldOrEmpty({ items: [] }, 'items')).toEqual([]);
    });

    it('returns an empty array when the field holds a non-array value', () => {
      expect(objectArrayFieldOrEmpty({ items: 'str' }, 'items')).toEqual([]);
      expect(objectArrayFieldOrEmpty({ items: {} }, 'items')).toEqual([]);
      expect(objectArrayFieldOrEmpty({ items: null }, 'items')).toEqual([]);
      expect(objectArrayFieldOrEmpty({ items: undefined }, 'items')).toEqual([]);
    });

    it('returns an empty array when the field is missing', () => {
      expect(objectArrayFieldOrEmpty({ other: 1 }, 'items')).toEqual([]);
      expect(objectArrayFieldOrEmpty({}, 'items')).toEqual([]);
    });

    it('returns an empty array for non-object roots', () => {
      expect(objectArrayFieldOrEmpty(null, 'items')).toEqual([]);
      expect(objectArrayFieldOrEmpty(undefined, 'items')).toEqual([]);
      expect(objectArrayFieldOrEmpty('str', 'items')).toEqual([]);
    });

    it('does not descend into nested objects to find arrays', () => {
      expect(objectArrayFieldOrEmpty({ nested: { inner: [1, 2] } }, 'nested')).toEqual([]);
    });
  });
});
