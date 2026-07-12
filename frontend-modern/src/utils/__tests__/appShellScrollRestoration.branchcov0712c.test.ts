import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  capturePendingAppShellRestoreTop,
  clearPendingAppShellRestoreTop,
  readPendingAppShellRestoreTop,
} from '@/utils/appShellScrollRestoration';

describe('capturePendingAppShellRestoreTop – branch coverage', () => {
  beforeEach(() => {
    clearPendingAppShellRestoreTop();
    document.body.innerHTML = '';
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.innerHTML = '';
  });

  it('returns early without capturing when document is undefined (SSR guard arm)', () => {
    vi.stubGlobal('document', undefined);

    capturePendingAppShellRestoreTop();

    expect(readPendingAppShellRestoreTop()).toBeNull();
  });

  it('does not capture when no .app-scroll-shell element exists (!shell short-circuit arm)', () => {
    capturePendingAppShellRestoreTop();

    expect(readPendingAppShellRestoreTop()).toBeNull();
  });

  it('does not capture when the shell is at the top (scrollTop === 0 hits the <= 0 boundary arm)', () => {
    const shell = document.createElement('div');
    shell.className = 'app-scroll-shell';
    shell.scrollTop = 0;
    document.body.appendChild(shell);

    capturePendingAppShellRestoreTop();

    expect(readPendingAppShellRestoreTop()).toBeNull();
  });

  it('does not capture when the shell reports a negative scrollTop (defensive <= 0 arm)', () => {
    // Real browsers never report a negative scrollTop, so inject a stub
    // element via querySelector to force the <= 0 guard's negative side and
    // confirm the exact selector contract.
    const stubShell = { scrollTop: -10 } as unknown as HTMLElement;
    const querySpy = vi.spyOn(document, 'querySelector').mockReturnValue(stubShell);

    capturePendingAppShellRestoreTop();

    expect(readPendingAppShellRestoreTop()).toBeNull();
    expect(querySpy).toHaveBeenCalledWith('.app-scroll-shell');
    querySpy.mockRestore();
  });

  it('captures the exact scrollTop when the shell is scrolled past the top (happy path)', () => {
    const shell = document.createElement('div');
    shell.className = 'app-scroll-shell';
    shell.scrollTop = 327;
    document.body.appendChild(shell);

    capturePendingAppShellRestoreTop();

    expect(readPendingAppShellRestoreTop()).toBe(327);
  });

  it('preserves a previously captured value when a later capture hits a guard arm (no-overwrite)', () => {
    const scrolled = document.createElement('div');
    scrolled.className = 'app-scroll-shell';
    scrolled.scrollTop = 150;
    document.body.appendChild(scrolled);

    capturePendingAppShellRestoreTop();
    expect(readPendingAppShellRestoreTop()).toBe(150);

    scrolled.remove();
    capturePendingAppShellRestoreTop();

    expect(readPendingAppShellRestoreTop()).toBe(150);
  });
});
