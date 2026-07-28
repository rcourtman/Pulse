// Regression coverage for issue #1640: a reverse proxy that cuts a slow
// Patrol readiness request answers with a full HTML error page. That body
// must never become Error.message, because the message is rendered directly
// into the readiness result boxes in AI settings.
import { describe, expect, it } from 'vitest';
import { apiErrorFromResponse } from '@/utils/apiClient';

const NGINX_GATEWAY_TIMEOUT = [
  '<html>',
  '<head><title>504 Gateway Time-out</title></head>',
  '<body>',
  '<center><h1>504 Gateway Time-out</h1></center>',
  '<hr><center>nginx</center>',
  '</body>',
  '</html>',
].join('\r\n');

describe('apiErrorFromResponse never surfaces raw HTML (#1640)', () => {
  it('replaces an HTML proxy error page with a generic status message', async () => {
    const error = await apiErrorFromResponse(new Response(NGINX_GATEWAY_TIMEOUT, { status: 504 }));
    expect(error.message).toBe('Request failed with status 504');
    expect(error.message).not.toContain('<');
    expect(error.status).toBe(504);
  });

  it('replaces a long non-JSON body with a generic status message', async () => {
    const error = await apiErrorFromResponse(new Response('x'.repeat(500), { status: 502 }));
    expect(error.message).toBe('Request failed with status 502');
  });

  it('keeps a short plain-text body as the message', async () => {
    const error = await apiErrorFromResponse(new Response('upstream unavailable', { status: 503 }));
    expect(error.message).toBe('upstream unavailable');
  });

  it('extracts short plain text from a <pre> block but rejects markup inside it', async () => {
    const plain = await apiErrorFromResponse(
      new Response('<html><body><pre>proxy read timeout</pre></body></html>', { status: 504 }),
    );
    expect(plain.message).toBe('proxy read timeout');

    const markup = await apiErrorFromResponse(
      new Response('<html><body><pre><b>504</b> upstream timed out</pre></body></html>', {
        status: 504,
      }),
    );
    expect(markup.message).toBe('Request failed with status 504');
  });

  it('still prefers a JSON error payload over any fallback handling', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: 'Patrol model readiness failed' }), { status: 500 }),
    );
    expect(error.message).toBe('Patrol model readiness failed');
  });

  it('uses the caller fallback message for unusable bodies', async () => {
    const error = await apiErrorFromResponse(
      new Response(NGINX_GATEWAY_TIMEOUT, { status: 504 }),
      'Pulse could not run the Patrol model readiness evaluation.',
    );
    expect(error.message).toBe('Pulse could not run the Patrol model readiness evaluation.');
  });

  // Precedence, pinned end to end. A sane short plain-text body outranks the
  // caller fallback -- server-provided detail on retryable errors has always
  // won here and call sites rely on it. Markup and oversized bodies are
  // dropped (#1640) so the fallback covers them, and canonical JSON outranks
  // everything because that message came from Pulse itself.
  describe('message precedence', () => {
    const FALLBACK = 'Pulse could not run the Patrol model readiness evaluation.';

    it('prefers a short plain-text body over the caller fallback', async () => {
      const error = await apiErrorFromResponse(
        new Response('upstream unavailable', { status: 503 }),
        FALLBACK,
      );
      expect(error.message).toBe('upstream unavailable');
    });

    it('prefers text extracted from a <pre> block over the caller fallback', async () => {
      const error = await apiErrorFromResponse(
        new Response('<html><body><pre>proxy read timeout</pre></body></html>', { status: 504 }),
        FALLBACK,
      );
      expect(error.message).toBe('proxy read timeout');
    });

    it('prefers the caller fallback over the generic status message', async () => {
      const error = await apiErrorFromResponse(
        new Response('x'.repeat(500), { status: 502 }),
        FALLBACK,
      );
      expect(error.message).toBe(FALLBACK);
    });

    it('still lets a canonical JSON error outrank the caller fallback', async () => {
      const error = await apiErrorFromResponse(
        new Response(JSON.stringify({ error: 'Patrol model readiness failed' }), { status: 500 }),
        FALLBACK,
      );
      expect(error.message).toBe('Patrol model readiness failed');
    });

    it('surfaces the plain-text body when no fallback was supplied', async () => {
      const error = await apiErrorFromResponse(
        new Response('upstream unavailable', { status: 503 }),
      );
      expect(error.message).toBe('upstream unavailable');
    });
  });
});
