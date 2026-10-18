import type { Page } from '@playwright/test';

export function captureAuthRenderer(page: Page): Promise<Record<string, unknown>>;
