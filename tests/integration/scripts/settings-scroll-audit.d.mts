import type { Page } from '@playwright/test';

export type OverflowAudit = {
  viewportWidth: number;
  pageWidth: number;
  overflowPx: number;
  offenders: Array<{ tag: string; className: string; overflow: number }>;
};

export function scrollSettingsToBottom(page: Page): Promise<void>;
export function settingsScrollPosition(page: Page): Promise<{
  scrollTop: number;
  maxScrollTop: number;
}>;
export function auditHorizontalOverflow(page: Page): Promise<OverflowAudit>;
