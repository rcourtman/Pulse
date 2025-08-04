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

async function forceDarkMode(page) {
  // First, inject dark mode styles and classes before page load
  await page.addInitScript(() => {
    // Force dark mode via localStorage
    localStorage.setItem('theme', 'dark');
    localStorage.setItem('pulse-theme', 'dark');
    localStorage.setItem('color-theme', 'dark');
    
    // Add dark class to html element
    document.documentElement.classList.add('dark');
    document.documentElement.classList.remove('light');
    
    // Override any light mode styles
    const style = document.createElement('style');
    style.textContent = `
      html, body {
        background-color: #111827 !important; /* gray-900 */
        color: #f3f4f6 !important; /* gray-100 */
      }
      html.dark {
        color-scheme: dark !important;
      }
      /* Force dark backgrounds on main containers */
      .bg-white {
        background-color: #1f2937 !important; /* gray-800 */
      }
      .bg-gray-50 {
        background-color: #374151 !important; /* gray-700 */
      }
      /* Ensure the main app background is dark */
      #root > div:first-child {
        background-color: #111827 !important;
      }
    `;
    document.head.appendChild(style);
  });
}

async function ensureDarkThemeAfterLoad(page) {
  await page.evaluate(() => {
    // Double-check and force dark mode after page load
    document.documentElement.classList.add('dark');
    document.documentElement.classList.remove('light');
    
    // Force dark background on body
    document.body.style.backgroundColor = '#111827';
    
    // Find any light backgrounds and make them dark
    const lightElements = document.querySelectorAll('[class*="bg-white"], [class*="bg-gray-50"]');
    lightElements.forEach(el => {
      el.style.backgroundColor = '#1f2937';
    });
  });
}

async function main() {
  await ensureOutputDir();

  const browser = await chromium.launch({
    headless: true
  });

  // Desktop screenshots with dark mode forced
  const desktopContext = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    colorScheme: 'dark' // Force dark color scheme
  });

  const page = await desktopContext.newPage();
  
  // Force dark mode before navigation
  await forceDarkMode(page);

  try {
    console.log(`📸 Starting screenshot capture for ${PULSE_URL}`);
    
    // Navigate to Pulse
    await page.goto(PULSE_URL, { waitUntil: 'networkidle' });
    await wait(2000);
    
    // Ensure dark theme after page load
    await ensureDarkThemeAfterLoad(page);
    await wait(1000);

    // 1. Dashboard Screenshot
    console.log('📸 Capturing dashboard...');
    await takeScreenshot(page, '01-dashboard');

    // 2. Storage View
    console.log('📸 Capturing storage view...');
    const storageClicked = await clickAndWait(page, 'button:has-text("Storage"), a:has-text("Storage"), [role="tab"]:has-text("Storage")', 2000);
    if (storageClicked) {
      await ensureDarkThemeAfterLoad(page);
      await takeScreenshot(page, '02-storage');
    }

    // 3. Backups View
    console.log('📸 Capturing backups view...');
    const backupsClicked = await clickAndWait(page, 'button:has-text("Backups"), a:has-text("Backups"), [role="tab"]:has-text("Backups")', 2000);
    if (backupsClicked) {
      await ensureDarkThemeAfterLoad(page);
      await takeScreenshot(page, '03-backups');
    }

    // 4. Alerts View
    console.log('📸 Capturing alerts view...');
    const alertsClicked = await clickAndWait(page, 'button:has-text("Alerts"), a:has-text("Alerts"), [role="tab"]:has-text("Alerts")', 2000);
    if (alertsClicked) {
      await ensureDarkThemeAfterLoad(page);
      await takeScreenshot(page, '04-alerts');
      
      // 5. Alert History
      console.log('📸 Looking for alert history...');
      const historyClicked = await clickAndWait(page, 'button:has-text("History"), button:has-text("Alert History"), [role="tab"]:has-text("History")', 2000);
      if (historyClicked) {
        await ensureDarkThemeAfterLoad(page);
        await takeScreenshot(page, '05-alert-history');
      } else {
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
      await ensureDarkThemeAfterLoad(page);
      await takeScreenshot(page, '06-settings');
    } else {
      await clickAndWait(page, 'svg[class*="settings"], svg[class*="gear"], button:has(svg[class*="settings"])', 2000);
      await ensureDarkThemeAfterLoad(page);
      await takeScreenshot(page, '06-settings');
    }

    // 7. Dark Mode Screenshot
    console.log('📸 Capturing dark mode view...');
    await clickAndWait(page, 'button:has-text("Dashboard"), a:has-text("Dashboard"), [role="tab"]:has-text("Dashboard")', 2000);
    await ensureDarkThemeAfterLoad(page);
    await wait(1000);
    await takeScreenshot(page, '07-dark-mode');

    // Close desktop context
    await desktopContext.close();

    // 8. Mobile Screenshot with dark mode
    console.log('📸 Capturing mobile view...');
    const mobileContext = await browser.newContext({
      ...devices['iPhone 12'],
      viewport: { width: 390, height: 844 },
      colorScheme: 'dark'
    });
    
    const mobilePage = await mobileContext.newPage();
    await forceDarkMode(mobilePage);
    
    await mobilePage.goto(PULSE_URL, { waitUntil: 'networkidle' });
    await wait(2000);
    
    await ensureDarkThemeAfterLoad(mobilePage);
    await wait(1000);
    
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