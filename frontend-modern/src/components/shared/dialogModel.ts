export type DialogLayout = 'modal' | 'drawer-right';

const FOCUSABLE_SELECTOR =
  'a[href],area[href],button:not([disabled]),input:not([disabled]):not([type="hidden"]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

export function getDialogLayout(layout?: DialogLayout): DialogLayout {
  return layout ?? 'modal';
}

export function getDialogFocusableElements(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (element) =>
      !element.hasAttribute('disabled') && element.getAttribute('aria-hidden') !== 'true',
  );
}

export function getDialogViewportClass(layout: DialogLayout): string {
  return `relative h-full overflow-y-auto pointer-events-none ${
    layout === 'drawer-right' ? 'p-0' : 'p-4 sm:p-6'
  }`;
}

export function getDialogAlignmentClass(layout: DialogLayout): string {
  return `flex min-h-full ${
    layout === 'drawer-right'
      ? 'items-stretch justify-end'
      : 'items-start justify-center sm:items-center'
  }`;
}

export function getDialogPanelClass(layout: DialogLayout, panelClass?: string): string {
  return `relative flex w-full flex-col overflow-hidden bg-surface border border-border outline-none pointer-events-auto ${
    layout === 'drawer-right'
      ? 'h-dvh max-w-[720px] rounded-none border-y-0 border-r-0 animate-slide-up sm:h-full sm:max-h-dvh sm:rounded-l-xl sm:border-y sm:border-r-0'
      : 'max-h-[calc(100dvh-2rem)] rounded-md animate-slide-up'
  } ${panelClass ?? (layout === 'drawer-right' ? '' : 'max-w-lg')}`.trim();
}
