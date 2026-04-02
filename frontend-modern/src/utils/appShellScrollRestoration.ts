let pendingAppShellRestoreTop: number | null = null;
const APP_SCROLL_SHELL_SELECTOR = '.app-scroll-shell';

export const schedulePendingAppShellRestoreTop = (scrollTop: number): void => {
  pendingAppShellRestoreTop = Math.max(0, scrollTop);
};

export const capturePendingAppShellRestoreTop = (): void => {
  if (typeof document === 'undefined') {
    return;
  }
  const shell = document.querySelector<HTMLElement>(APP_SCROLL_SHELL_SELECTOR);
  if (!shell || shell.scrollTop <= 0) {
    return;
  }
  schedulePendingAppShellRestoreTop(shell.scrollTop);
};

export const readPendingAppShellRestoreTop = (): number | null => pendingAppShellRestoreTop;

export const clearPendingAppShellRestoreTop = (): void => {
  pendingAppShellRestoreTop = null;
};
