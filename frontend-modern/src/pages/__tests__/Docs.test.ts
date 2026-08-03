import { describe, expect, it } from 'vitest';
import { isHtmlResponse } from '../Docs';

// A document that is not shipped does not 404. The Go static handler and the
// dev server both fall back to index.html for any unmatched non-API path,
// including one ending in .md, so response.ok stays true and the only signal
// that the asset is missing is the content type.
describe('isHtmlResponse', () => {
  const withContentType = (value: string | null) => ({
    headers: { get: () => value } as unknown as Headers,
  });

  it('detects the SPA fallback that stands in for a missing document', () => {
    expect(isHtmlResponse(withContentType('text/html'))).toBe(true);
    expect(isHtmlResponse(withContentType('text/html; charset=utf-8'))).toBe(true);
    expect(isHtmlResponse(withContentType('TEXT/HTML'))).toBe(true);
  });

  it('accepts the content types the shipped docs are actually served with', () => {
    // text/plain from the Go static handler, text/markdown from vite.
    expect(isHtmlResponse(withContentType('text/plain; charset=utf-8'))).toBe(false);
    expect(isHtmlResponse(withContentType('text/markdown'))).toBe(false);
    expect(isHtmlResponse(withContentType(null))).toBe(false);
  });
});
