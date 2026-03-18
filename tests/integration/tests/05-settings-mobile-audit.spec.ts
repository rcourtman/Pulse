import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
type WorkerFixtures = {
    authStorageStatePath: string;
};

const SETTINGS_ROUTES = [
    '/settings/infrastructure',
    '/settings/infrastructure/pve',
    '/settings/infrastructure/pbs',
    '/settings/infrastructure/pmg',
    '/settings/workloads',
    '/settings/workloads/docker',
    '/settings/organization',
    '/settings/organization/access',
    '/settings/organization/sharing',
    '/settings/organization/billing',
    '/settings/organization/billing-admin',
    '/settings/integrations/api',
    '/operations/diagnostics',
    '/operations/reporting',
    '/operations/logs',
    '/settings/system-general',
    '/settings/system-network',
    '/settings/system-updates',
    '/settings/system-recovery',
    '/settings/system-ai',
    '/settings/system-relay',
    '/settings/system-pro',
    '/settings/security-overview',
    '/settings/security-auth',
    '/settings/security-sso',
    '/settings/security-roles',
    '/settings/security-users',
    '/settings/security-audit',
    '/settings/security-webhooks',
] as const;

const test = base.extend<{}, WorkerFixtures>({
    storageState: async ({ authStorageStatePath }, use) => {
        await use(authStorageStatePath);
    },
    authStorageStatePath: [async ({ browser }, use, workerInfo) => {
        const storageStatePath = path.resolve(
            __dirname,
            '..',
            '..',
            'tmp',
            'playwright-auth',
            `settings-mobile-audit-${workerInfo.project.name}.json`,
        );
        fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
        await createAuthenticatedStorageState(browser, storageStatePath);
        try {
            await use(storageStatePath);
        } finally {
            fs.rmSync(storageStatePath, { force: true });
        }
    }, { scope: 'worker' }],
});

type OverflowAudit = {
    viewportWidth: number;
    pageWidth: number;
    overflowPx: number;
    offenders: Array<{ tag: string; className: string; overflow: number }>;
};

const auditHorizontalOverflow = async (
    page: import('@playwright/test').Page,
): Promise<OverflowAudit> =>
    page.evaluate(() => {
        const viewportWidth = Math.max(document.documentElement.clientWidth, window.innerWidth || 0);
        const pageWidth = Math.max(
            document.body.scrollWidth,
            document.documentElement.scrollWidth,
            document.body.offsetWidth,
            document.documentElement.offsetWidth,
        );

        const offenders = Array.from(document.querySelectorAll('body *'))
            .map((el) => {
                const rect = el.getBoundingClientRect();
                if (rect.width <= 0 || rect.height <= 0) return null;
                const style = window.getComputedStyle(el);
                if (style.position === 'fixed' || style.position === 'absolute') return null;
                const overflow = rect.right - viewportWidth;
                if (overflow <= 1) return null;
                return {
                    tag: el.tagName.toLowerCase(),
                    className: (el.getAttribute('class') || '').trim().slice(0, 120),
                    overflow: Number(overflow.toFixed(1)),
                };
            })
            .filter((entry): entry is { tag: string; className: string; overflow: number } => Boolean(entry))
            .slice(0, 8);

        return {
            viewportWidth,
            pageWidth,
            overflowPx: Number((pageWidth - viewportWidth).toFixed(1)),
            offenders,
        };
    });

const scrollToBottom = async (page: import('@playwright/test').Page): Promise<void> => {
    const viewportHeight = await page.evaluate(() => window.innerHeight || 800);
    const step = Math.max(240, Math.floor(viewportHeight * 0.75));
    let wheelSupported = true;
    for (let i = 0; i < 20; i += 1) {
        if (wheelSupported) {
            try {
                await page.mouse.wheel(0, step);
            } catch {
                wheelSupported = false;
                await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
            }
        } else {
            await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
        }
        await page.waitForTimeout(60);
    }
};

test.describe('Settings mobile optimization audit', () => {
    test.setTimeout(180_000);

    for (const route of SETTINGS_ROUTES) {
        test(`no horizontal overflow after full scroll on ${route}`, async ({ page }) => {
            await page.goto(route, { waitUntil: 'domcontentloaded' });
            await page.waitForURL(/\/(settings|operations)/, { timeout: 15000 });
            await expect(page.locator('#root')).toBeVisible();
            await page.waitForTimeout(600);

            await scrollToBottom(page);
            const atBottom = await page.evaluate(() => {
                const scrollTop = window.scrollY;
                const maxScrollTop = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
                return scrollTop >= maxScrollTop - 3;
            });
            expect(atBottom, `Expected to reach the bottom while auditing ${route}`).toBeTruthy();

            const audit = await auditHorizontalOverflow(page);
            expect(
                audit.pageWidth,
                `Mobile overflow on ${route} (viewport=${audit.viewportWidth}, page=${audit.pageWidth}, offenders=${JSON.stringify(audit.offenders)})`,
            ).toBeLessThanOrEqual(audit.viewportWidth + 1);
        });
    }
});
