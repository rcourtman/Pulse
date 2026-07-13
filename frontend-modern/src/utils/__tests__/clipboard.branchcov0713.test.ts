/**
 * Branch-coverage tests for clipboard.ts — second pass.
 *
 * Scope: ONLY `copyToClipboard`. The sibling clipboard.test.ts already drives:
 *   - clipboard API happy path (writeText resolves → true),
 *   - writeText rejects + execCommand returns true (fallback succeeds),
 *   - writeText rejects + execCommand returns false (both fail),
 *   - navigator.clipboard === undefined + document undefined → false.
 *
 * This file drives every remaining branch:
 *
 * - `typeof navigator !== 'undefined'` FALSE arm — navigator entirely absent
 *   (SSR-like) — reaching the DOM fallback rather than the document-undefined
 *   early return.
 * - `navigator.clipboard?.writeText` falsy arm reached two ways the sibling
 *   test does not exercise: `clipboard` itself undefined while document IS
 *   defined, and `writeText` undefined while `clipboard` exists. Both must
 *   fall through to the execCommand fallback (the sibling only reaches this
 *   point with `document` also undefined, hitting the early return).
 * - The fallback DOM setup: textarea.value, the three off-screen style
 *   assignments, focus(), and select() — none of which the sibling asserts
 *   because it stubs document wholesale.
 * - The execCommand THROWS arm (inner catch at the fallback try/catch) — the
 *   sibling only drives the true/false return-value arms.
 * - The `finally` block: removeChild runs even on the success path.
 * - The OUTER try/catch: a throw inside the fallback setup (appendChild) is
 *   swallowed and returns false.
 * - Empty-string input forwarded verbatim to the clipboard API.
 *
 * jsdom does not implement `document.execCommand`, so each fallback-path test
 * installs it via `Object.defineProperty` (configurable) and removes the stub
 * in `afterEach`. All other DOM interactions use the real jsdom document so
 * textarea side-effects (value, styles, focus, select) can be asserted
 * directly rather than against a wholesale mock.
 */
import { afterEach, describe, expect, it, vi } from 'vitest';

import { copyToClipboard } from '@/utils/clipboard';

/** Installs a configurable `execCommand` stub on document (jsdom omits it). */
const stubExecCommand = (impl: (commandId: string, value?: string) => boolean) => {
  const mock = vi.fn(impl);
  Object.defineProperty(document, 'execCommand', {
    value: mock,
    configurable: true,
    writable: true,
  });
  return mock;
};

describe('copyToClipboard (branch coverage)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    // Remove our execCommand stub if it is still installed on the instance.
    const desc = Object.getOwnPropertyDescriptor(document, 'execCommand');
    if (desc?.configurable) {
      delete (document as unknown as { execCommand?: unknown }).execCommand;
    }
    document.body.innerHTML = '';
  });

  it('falls through to the DOM fallback when navigator is entirely undefined (typeof navigator arm)', async () => {
    vi.stubGlobal('navigator', undefined);
    const execSpy = stubExecCommand(() => true);

    const result = await copyToClipboard('payload-without-navigator');

    expect(result).toBe(true);
    expect(execSpy).toHaveBeenCalledWith('copy');
  });

  it('falls through to the DOM fallback when navigator exists but navigator.clipboard is undefined', async () => {
    // The sibling test only reaches this guard with document ALSO undefined;
    // here document is defined, so the execCommand fallback must run.
    vi.stubGlobal('navigator', { clipboard: undefined });
    const execSpy = stubExecCommand(() => true);

    const result = await copyToClipboard('payload-no-clipboard');

    expect(result).toBe(true);
    expect(execSpy).toHaveBeenCalledWith('copy');
  });

  it('falls through to the DOM fallback when navigator.clipboard exists but writeText is missing', async () => {
    vi.stubGlobal('navigator', { clipboard: {} });
    const execSpy = stubExecCommand(() => true);

    const result = await copyToClipboard('payload-no-writeText');

    expect(result).toBe(true);
    expect(execSpy).toHaveBeenCalledWith('copy');
  });

  it('configures the textarea (value, off-screen styles, focus, select) and cleans up via finally before returning true', async () => {
    // Force the fallback by removing the clipboard API entirely.
    vi.stubGlobal('navigator', undefined);

    // Pre-build the textarea and instrument it so we can assert the module's
    // DOM side-effects directly. createElement returns our instrumented node.
    const textArea = document.createElement('textarea');
    const focusSpy = vi.spyOn(textArea, 'focus');
    const selectSpy = vi.spyOn(textArea, 'select');
    const createSpy = vi.spyOn(document, 'createElement').mockReturnValue(textArea);
    const appendSpy = vi.spyOn(document.body, 'appendChild').mockImplementation((node) => node);
    const removeSpy = vi.spyOn(document.body, 'removeChild').mockImplementation((node) => node);
    const execSpy = stubExecCommand(() => true);

    const payload = 'configure-the-textarea';
    const result = await copyToClipboard(payload);

    expect(result).toBe(true);
    expect(createSpy).toHaveBeenCalledWith('textarea');
    // value + off-screen positioning:
    expect(textArea.value).toBe(payload);
    expect(textArea.style.position).toBe('fixed');
    expect(textArea.style.left).toBe('-999999px');
    expect(textArea.style.top).toBe('-999999px');
    // focus + select invoked exactly once each:
    expect(focusSpy).toHaveBeenCalledTimes(1);
    expect(selectSpy).toHaveBeenCalledTimes(1);
    // appendChild then removeChild (finally) on the SAME node:
    expect(appendSpy).toHaveBeenCalledWith(textArea);
    expect(removeSpy).toHaveBeenCalledWith(textArea);
    expect(removeSpy).toHaveBeenCalledTimes(1);
    expect(execSpy).toHaveBeenCalledWith('copy');
  });

  it('returns false and still runs the finally cleanup when document.execCommand throws (inner catch arm)', async () => {
    vi.stubGlobal('navigator', undefined);
    const textArea = document.createElement('textarea');
    vi.spyOn(document, 'createElement').mockReturnValue(textArea);
    const appendSpy = vi.spyOn(document.body, 'appendChild').mockImplementation((node) => node);
    const removeSpy = vi.spyOn(document.body, 'removeChild').mockImplementation((node) => node);
    const execErr = new Error('execCommand unavailable');
    stubExecCommand(() => {
      throw execErr;
    });

    const result = await copyToClipboard('exec-throws');

    expect(result).toBe(false);
    // finally block still runs cleanup even after the throw:
    expect(appendSpy).toHaveBeenCalledWith(textArea);
    expect(removeSpy).toHaveBeenCalledWith(textArea);
  });

  it('returns false from the outer catch when the fallback setup itself throws before execCommand', async () => {
    // The inner try wraps only execCommand; a throw in appendChild (part of
    // the setup) propagates to the OUTER try/catch and must be swallowed.
    vi.stubGlobal('navigator', undefined);
    const textArea = document.createElement('textarea');
    vi.spyOn(document, 'createElement').mockReturnValue(textArea);
    const setupErr = new Error('appendChild denied');
    const appendSpy = vi.spyOn(document.body, 'appendChild').mockImplementation(() => {
      throw setupErr;
    });
    // execCommand must NOT be reached, proving we never entered the inner try:
    const execSpy = stubExecCommand(() => true);

    const result = await copyToClipboard('setup-throws');

    expect(result).toBe(false);
    expect(appendSpy).toHaveBeenCalledWith(textArea);
    expect(execSpy).not.toHaveBeenCalled();
  });

  it('forwards an empty string verbatim to the clipboard API on the happy path', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });

    const result = await copyToClipboard('');

    expect(result).toBe(true);
    expect(writeText).toHaveBeenCalledWith('');
    expect(writeText).toHaveBeenCalledTimes(1);
  });
});
