const { chromium } = require('@playwright/test');
const fs = require('fs').promises;
const path = require('path');

const PULSE_URL = process.env.PULSE_URL || 'http://localhost:7655';
const OUTPUT_DIR = path.join(__dirname, '..', 'docs', 'images');

async function ensureOutputDir() {
  try {
    await fs.mkdir(OUTPUT_DIR, { recursive: true });
  } catch (error) {
    console.error('Error creating output directory:', error);
  }
}

async function wait(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function takeScreenshot(page, name, options = {}) {
  const { selector = null, fullPage = false } = options;

  await wait(1500); // Wait for animations

  let screenshotOptions = {
    type: 'png',
    path: path.join(OUTPUT_DIR, `${name}.png`)
  };

  if (selector) {
    const element = await page.locator(selector).first();
    await element.waitFor({ state: 'visible', timeout: 10000 });
    screenshotOptions = {
      ...screenshotOptions,
      clip: await element.boundingBox()
    };
  } else if (fullPage) {
    screenshotOptions.fullPage = true;
  }

  await page.screenshot(screenshotOptions);
  console.log(`✅ Saved ${name}.png`);
}

async function clickAndWait(page, selector, waitTime = 1000) {
  try {
    await page.click(selector);
    await wait(waitTime);
  } catch (e) {
    console.log(`Could not click ${selector}: ${e.message}`);
  }
}

async function ensureDarkTheme(page) {
  try {
    // Check if theme toggle exists
    const themeToggle = page.locator('[aria-label="Toggle theme"], button:has-text("Theme")').first();
    if (await themeToggle.count() > 0) {
      // Check if we're in light mode
      const isLightMode = await page.evaluate(() => {
        return document.documentElement.classList.contains('light') || 
               !document.documentElement.classList.contains('dark');
      });
      
      if (isLightMode) {
        await themeToggle.click();
        await wait(500);
      }
    }
  } catch (e) {
    console.log('Could not set dark theme:', e.message);
  }
}

async function main() {
  await ensureOutputDir();

  const browser = await chromium.launch({
    headless: true
  });

  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 }
  });

  const page = await context.newPage();

  try {
    console.log(`📸 Starting screenshot capture for ${PULSE_URL}`);
    
    // Navigate to Pulse
    await page.goto(PULSE_URL, { waitUntil: 'networkidle' });
    await wait(3000);

    // Ensure dark theme
    await ensureDarkTheme(page);

    // 1. Dashboard Screenshot
    console.log('📸 Capturing dashboard...');
    await takeScreenshot(page, '01-dashboard');

    // 2. Try to navigate to different views
    // Look for navigation tabs or buttons
    const navSelectors = [
      'button:has-text("Storage")',
      'a:has-text("Storage")',
      '[role="tab"]:has-text("Storage")',
      '.nav-link:has-text("Storage")'
    ];

    for (const selector of navSelectors) {
      if (await page.locator(selector).count() > 0) {
        console.log('📸 Capturing storage view...');
        await clickAndWait(page, selector, 2000);
        await takeScreenshot(page, '02-storage');
        break;
      }
    }

    // 3. Try Backups
    const backupSelectors = [
      'button:has-text("Backups")',
      'a:has-text("Backups")',
      '[role="tab"]:has-text("Backups")',
      '.nav-link:has-text("Backups")'
    ];

    for (const selector of backupSelectors) {
      if (await page.locator(selector).count() > 0) {
        console.log('📸 Capturing backups view...');
        await clickAndWait(page, selector, 2000);
        await takeScreenshot(page, '03-backups');
        break;
      }
    }

    // 4. Try Alerts
    const alertSelectors = [
      'button:has-text("Alerts")',
      'a:has-text("Alerts")',
      '[role="tab"]:has-text("Alerts")',
      '.nav-link:has-text("Alerts")'
    ];

    for (const selector of alertSelectors) {
      if (await page.locator(selector).count() > 0) {
        console.log('📸 Capturing alerts view...');
        await clickAndWait(page, selector, 2000);
        await takeScreenshot(page, '04-alerts');
        break;
      }
    }

    // 5. Try Settings
    const settingsSelectors = [
      'button:has-text("Settings")',
      'a:has-text("Settings")',
      '[aria-label="Settings"]',
      '.settings-icon'
    ];

    for (const selector of settingsSelectors) {
      if (await page.locator(selector).count() > 0) {
        console.log('📸 Capturing settings view...');
        await clickAndWait(page, selector, 2000);
        await takeScreenshot(page, '05-settings');
        break;
      }
    }

    console.log('✨ Screenshot capture complete!');
    console.log(`📁 Screenshots saved to: ${OUTPUT_DIR}`);

  } catch (error) {
    console.error('❌ Error during screenshot capture:', error);
    
    // Take error screenshot
    await takeScreenshot(page, 'error-state');
  } finally {
    await browser.close();
  }
}

// Run the script
main().catch(console.error);