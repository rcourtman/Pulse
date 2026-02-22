import { describe, expect, it, vi } from 'vitest';
import { readAPIErrorMessage, parseOptionalJSON } from '@/api/responseUtils';

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
    await expect(parseOptionalJSON(response, {}, 'Custom parse error')).rejects.toThrow('Custom parse error');
  });
});
