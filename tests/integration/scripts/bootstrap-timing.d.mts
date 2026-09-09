import type { Page } from '@playwright/test';
export type BootstrapTiming = {
  transport: string;
  path: string;
  started: number;
  status: number | null;
  duration: number | null;
};
export function observeBootstrapTiming(page: Page, now?: () => number): () => Promise<BootstrapTiming[]>;
