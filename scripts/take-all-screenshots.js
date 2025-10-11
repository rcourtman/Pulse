const { chromium, devices } = require('@playwright/test');
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
    const element = page.locator(selector).first();
    if (await element.count() > 0) {
      await element.click();
      await wait(waitTime);
      return true;
    }
  } catch (e) {
    console.log(`Could not click ${selector}: ${e.message}`);
  }
  return false;
}

async function ensureDarkTheme(page) {
  try {
    // Wait a bit for theme system to initialize
    await wait(1000);
    
    // First check if we're already in dark mode
    const isDarkMode = await page.evaluate(() => {
      return document.documentElement.classList.contains('dark');
    });
    
    console.log(`Current theme mode: ${isDarkMode ? 'dark' : 'light'}`);
    
    if (!isDarkMode) {
      // Try to find and click the theme toggle
      // Look for common theme toggle selectors
      const themeSelectors = [
        'button[aria-label="Toggle theme"]',
        'button[title*="theme" i]',
        'button[aria-label*="theme" i]',
        'button:has(svg[class*="moon"])',
        'button:has(svg[class*="sun"])',
        '.theme-toggle',
        'button:has-text("Theme")',
        '[data-theme-toggle]'
      ];
      
      for (const selector of themeSelectors) {
        try {
          const element = page.locator(selector).first();
          if (await element.count() > 0) {
            await element.click();
            await wait(1000); // Wait for theme transition
            
            // Verify dark mode is now active
            const nowDark = await page.evaluate(() => {
              return document.documentElement.classList.contains('dark');
            });
            
            if (nowDark) {
              console.log('✅ Successfully switched to dark theme');
              return;
            }
          }
        } catch (e) {
          // Continue trying other selectors
        }
      }
      
      // If no toggle found, try to force dark mode via localStorage or class
      await page.evaluate(() => {
        document.documentElement.classList.remove('light');
        document.documentElement.classList.add('dark');
        localStorage.setItem('theme', 'dark');
        localStorage.setItem('pulse-theme', 'dark');
      });
      console.log('⚠️  Forced dark mode via DOM manipulation');
    } else {
      console.log('✅ Already in dark mode');
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

  // Desktop screenshots
  const desktopContext = await browser.newContext({
    viewport: { width: 1440, height: 900 }
  });

  const page = await desktopContext.newPage();

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

    // 2. Storage View
    console.log('📸 Capturing storage view...');
    const storageClicked = await clickAndWait(page, 'button:has-text("Storage"), a:has-text("Storage"), [role="tab"]:has-text("Storage")', 2000);
    if (storageClicked) {
      await takeScreenshot(page, '02-storage');
    }

    // 3. Backups View
    console.log('📸 Capturing backups view...');
    const backupsClicked = await clickAndWait(page, 'button:has-text("Backups"), a:has-text("Backups"), [role="tab"]:has-text("Backups")', 2000);
    if (backupsClicked) {
      await takeScreenshot(page, '03-backups');
    }

    // 4. Alerts View
    console.log('📸 Capturing alerts view...');
    const alertsClicked = await clickAndWait(page, 'button:has-text("Alerts"), a:has-text("Alerts"), [role="tab"]:has-text("Alerts")', 2000);
    if (alertsClicked) {
      await takeScreenshot(page, '04-alerts');
      
      // 5. Try to capture Alert History if available
      console.log('📸 Looking for alert history...');
      const historyClicked = await clickAndWait(page, 'button:has-text("History"), button:has-text("Alert History"), [role="tab"]:has-text("History")', 2000);
      if (historyClicked) {
        await takeScreenshot(page, '05-alert-history');
      } else {
        // If no history tab, just use alerts view
        await fs.copyFile(
          path.join(OUTPUT_DIR, '04-alerts.png'),
          path.join(OUTPUT_DIR, '05-alert-history.png')
        );
        console.log('✅ Used alerts view for alert history');
      }
    }

    // 6. Settings View
    console.log('📸 Capturing settings view...');
    const settingsClicked = await clickAndWait(page, 'button:has-text("Settings"), a:has-text("Settings"), [aria-label="Settings"], .settings-icon', 2000);
    if (settingsClicked) {
      await takeScreenshot(page, '06-settings');
    } else {
      // Try clicking a gear icon
      await clickAndWait(page, 'svg[class*="settings"], svg[class*="gear"], button:has(svg[class*="settings"])', 2000);
      await takeScreenshot(page, '06-settings');
    }

    // 7. Dark Mode Screenshot (go back to dashboard for a clean dark mode shot)
    console.log('📸 Capturing dark mode view...');
    await clickAndWait(page, 'button:has-text("Dashboard"), a:has-text("Dashboard"), [role="tab"]:has-text("Dashboard")', 2000);
    await wait(1000);
    await takeScreenshot(page, '07-dark-mode');

    // Close desktop context
    await desktopContext.close();

    // 8. Mobile Screenshot
    console.log('📸 Capturing mobile view...');
    const mobileContext = await browser.newContext({
      ...devices['iPhone 12'],
      viewport: { width: 390, height: 844 }
    });
    
    const mobilePage = await mobileContext.newPage();
    await mobilePage.goto(PULSE_URL, { waitUntil: 'networkidle' });
    await wait(2000);
    
    // Ensure dark theme on mobile
    await ensureDarkTheme(mobilePage);
    
    await takeScreenshot(mobilePage, '08-mobile');
    
    await mobileContext.close();

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