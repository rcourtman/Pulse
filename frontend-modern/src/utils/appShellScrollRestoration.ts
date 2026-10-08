let pendingAppShellRestoreTop: number | null = null;

export const schedulePendingAppShellRestoreTop = (scrollTop: number): void => {
  pendingAppShellRestoreTop = Math.max(0, scrollTop);
};

export const readPendingAppShellRestoreTop = (): number | null => pendingAppShellRestoreTop;

export const clearPendingAppShellRestoreTop = (): void => {
  pendingAppShellRestoreTop = null;
};
