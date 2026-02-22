import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { copyToClipboard } from '../clipboard';

describe('clipboard', () => {
  let originalNavigator: Navigator;
  let originalDocument: Document;

  beforeEach(() => {
    originalNavigator = globalThis.navigator;
    originalDocument = globalThis.document;

    vi.stubGlobal('navigator', {
      clipboard: {
        writeText: vi.fn(),
      },
    });

    vi.stubGlobal('document', {
      body: {
        appendChild: vi.fn(),
        removeChild: vi.fn(),
      },
      createElement: vi.fn().mockImplementation((tag: string) => ({
        tagName: tag.toUpperCase(),
        value: '',
        style: {},
        focus: vi.fn(),
        select: vi.fn(),
      })),
      execCommand: vi.fn().mockReturnValue(true),
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('copies text using clipboard API', async () => {
    vi.mocked(navigator.clipboard.writeText).mockResolvedValueOnce(undefined);

    const result = await copyToClipboard('test text');

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('test text');
    expect(result).toBe(true);
  });

  it('returns true when clipboard API throws but fallback succeeds', async () => {
    vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(new Error('Failed'));
    vi.mocked(document.execCommand).mockReturnValueOnce(true);

    const result = await copyToClipboard('test text');

    expect(result).toBe(true);
  });

  it('falls back to execCommand when clipboard API throws', async () => {
    vi.stubGlobal('navigator', {
      clipboard: {
        writeText: vi.fn().mockRejectedValueOnce(new Error('Clipboard API failed')),
      },
    });
    vi.mocked(document.execCommand).mockReturnValueOnce(true);

    const result = await copyToClipboard('test text');

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('test text');
    expect(document.execCommand).toHaveBeenCalledWith('copy');
    expect(result).toBe(true);
  });

  it('returns false when both clipboard API and fallback fail', async () => {
    vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(new Error('Failed'));
    vi.mocked(document.execCommand).mockReturnValueOnce(false);

    const result = await copyToClipboard('test text');

    expect(result).toBe(false);
  });

  it('returns false when document is not available', async () => {
    vi.stubGlobal('navigator', {
      clipboard: undefined,
    });
    vi.stubGlobal('document', undefined);

    const result = await copyToClipboard('test text');

    expect(result).toBe(false);
  });
});
