const { chromium } = require('@playwright/test');
const sharp = require('sharp');
const fs = require('fs').promises;
const path = require('path');

const PULSE_URL = process.env.PULSE_URL || 'http://localhost:7655';
const OUTPUT_DIR = path.join(__dirname, '..', 'docs', 'images');
const NO_BROWSER_WINDOW = process.env.NO_BROWSER_WINDOW === 'true';

// Configurable shadow settings
const SHADOW_BLUR = parseInt(process.env.SHADOW_BLUR || '50');
const SHADOW_OPACITY = parseFloat(process.env.SHADOW_OPACITY || '0.2');
const SHADOW_OFFSET = parseInt(process.env.SHADOW_OFFSET || '20');

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

async function addBrowserWindow(screenshotBuffer, width, height) {
  if (NO_BROWSER_WINDOW) {
    return screenshotBuffer;
  }

  const browserHeight = 80;
  const totalHeight = height + browserHeight;
  const shadowPadding = 60;
  const totalWidth = width + shadowPadding * 2;
  const totalHeightWithShadow = totalHeight + shadowPadding * 2;

  // Create browser window top bar
  const topBar = Buffer.from(`
    <svg width="${width}" height="${browserHeight}" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient id="bg" x1="0%" y1="0%" x2="0%" y2="100%">
          <stop offset="0%" style="stop-color:#2d2d2d;stop-opacity:1" />
          <stop offset="100%" style="stop-color:#1a1a1a;stop-opacity:1" />
        </linearGradient>
      </defs>
      <rect width="${width}" height="${browserHeight}" fill="url(#bg)"/>
      
      <!-- Traffic lights -->
      <circle cx="30" cy="40" r="6" fill="#ff5f56"/>
      <circle cx="50" cy="40" r="6" fill="#ffbd2e"/>
      <circle cx="70" cy="40" r="6" fill="#27c93f"/>
      
      <!-- URL Bar -->
      <rect x="120" y="25" width="${width - 240}" height="30" rx="5" fill="#000000" opacity="0.3"/>
      <text x="${width / 2}" y="45" font-family="Arial, sans-serif" font-size="13" fill="#888888" text-anchor="middle">
        localhost:7655 — Pulse Monitor
      </text>
    </svg>
  `);

  const topBarImage = await sharp(topBar)
    .png()
    .toBuffer();

  // Create shadow
  const shadow = Buffer.from(`
    <svg width="${totalWidth}" height="${totalHeightWithShadow}" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <filter id="shadow">
          <feGaussianBlur in="SourceAlpha" stdDeviation="${SHADOW_BLUR}"/>
          <feOffset dx="0" dy="${SHADOW_OFFSET}" result="offsetblur"/>
          <feFlood flood-color="#000000" flood-opacity="${SHADOW_OPACITY}"/>
          <feComposite in2="offsetblur" operator="in"/>
          <feMerge>
            <feMergeNode/>
            <feMergeNode in="SourceGraphic"/>
          </feMerge>
        </filter>
      </defs>
      <rect x="${shadowPadding}" y="${shadowPadding}" width="${width}" height="${totalHeight}" fill="white" filter="url(#shadow)" rx="10"/>
    </svg>
  `);

  // Convert shadow to buffer first
  const shadowBuffer = await sharp(shadow)
    .resize(totalWidth, totalHeightWithShadow)
    .png()
    .toBuffer();

  // Composite everything together
  const result = await sharp({
    create: {
      width: totalWidth,
      height: totalHeightWithShadow,
      channels: 4,
      background: { r: 255, g: 255, b: 255, alpha: 0 }
    }
  })
    .composite([
      {
        input: shadowBuffer,
        top: 0,
        left: 0
      },
      {
        input: topBarImage,
        top: shadowPadding,
        left: shadowPadding
      },
      {
        input: screenshotBuffer,
        top: shadowPadding + browserHeight,
        left: shadowPadding
      }
    ])
    .png()
    .toBuffer();

  return result;
}

async function takeScreenshot(page, name, options = {}) {
  const { 
    selector = null, 
    fullPage = false,
    addWindow = true,
    padding = 0
  } = options;

  await wait(1500); // Wait for animations to complete

  let screenshotOptions = {
    type: 'png',
    scale: 'device' // Use device scale factor for better quality
  };

  if (selector) {
    const element = await page.locator(selector).first();
    await element.waitFor({ state: 'visible', timeout: 10000 });
    
    if (padding > 0) {
      screenshotOptions = {
        ...screenshotOptions,
        clip: await element.boundingBox().then(box => ({
          x: Math.max(0, box.x - padding),
          y: Math.max(0, box.y - padding),
          width: box.width + (padding * 2),
          height: box.height + (padding * 2)
        }))
      };
    } else {
      screenshotOptions = {
        ...screenshotOptions,
        clip: await element.boundingBox()
      };
    }
  } else if (fullPage) {
    screenshotOptions.fullPage = true;
  }

  const screenshot = await page.screenshot(screenshotOptions);
  
  let finalImage = screenshot;
  if (addWindow && !fullPage && !selector) {
    const viewport = page.viewportSize();
    finalImage = await addBrowserWindow(screenshot, viewport.width, viewport.height);
  }

  // Save as PNG
  const filename = `${name}.png`;
  await fs.writeFile(path.join(OUTPUT_DIR, filename), finalImage);
  console.log(`✅ Saved ${filename}`);
  
  return filename;
}

async function clickAndWait(page, selector, waitTime = 1000) {
  await page.click(selector);
  await wait(waitTime);
}

async function ensureDarkTheme(page) {
  // Check if theme toggle exists and if we're in light mode
  const themeToggle = page.locator('[aria-label="Toggle theme"]');
  if (await themeToggle.count() > 0) {
    // Check if we're in light mode by looking for the light theme class
    const isLightMode = await page.evaluate(() => {
      return document.documentElement.classList.contains('light') || 
             !document.documentElement.classList.contains('dark');
    });
    
    if (isLightMode) {
      await themeToggle.click();
      await wait(500); // Wait for theme transition
    }
  }
}

async function main() {
  await ensureOutputDir();

  const browser = await chromium.launch({
    headless: true,
    args: ['--force-device-scale-factor=2'] // Force 2x DPI for retina quality
  });

  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2
  });

  const page = await context.newPage();

  try {
    console.log(`📸 Starting screenshot capture for ${PULSE_URL}`);
    
    // Navigate to Pulse
    await page.goto(PULSE_URL, { waitUntil: 'networkidle' });
    await wait(3000); // Wait for initial load

    // Ensure dark theme is active
    await ensureDarkTheme(page);

    // 1. Dashboard Screenshot
    console.log('📸 Capturing dashboard...');
    await takeScreenshot(page, '01-dashboard');

    // 2. Storage View
    console.log('📸 Capturing storage view...');
    // Click on Storage tab
    await clickAndWait(page, 'button:has-text("Storage")', 2000);
    await takeScreenshot(page, '02-storage');

    // 3. Backups View
    console.log('📸 Capturing backups view...');
    // Click on Backups tab
    await clickAndWait(page, 'button:has-text("Backups")', 2000);
    await takeScreenshot(page, '03-backups');

    // 4. Charts/Metrics View
    console.log('📸 Capturing charts view...');
    // Go back to dashboard first
    await clickAndWait(page, 'button:has-text("Dashboard")', 2000);
    
    // Click on a node to see its charts (if available)
    const nodeCard = page.locator('.node-card').first();
    if (await nodeCard.count() > 0) {
      await nodeCard.click();
      await wait(2000);
      
      // Look for charts or metrics section
      const chartsSection = page.locator('[data-charts], .charts-container, .metrics-view').first();
      if (await chartsSection.count() > 0) {
        await takeScreenshot(page, '04-charts');
      }
    }

    // 5. Alerts View
    console.log('📸 Capturing alerts view...');
    // Click on Alerts tab if it exists
    const alertsTab = page.locator('button:has-text("Alerts")');
    if (await alertsTab.count() > 0) {
      await clickAndWait(alertsTab, 2000);
      await takeScreenshot(page, '05-alerts');
    }

    // 6. Settings View (optional)
    console.log('📸 Capturing settings view...');
    // Click on Settings tab/button
    const settingsButton = page.locator('button:has-text("Settings"), [aria-label="Settings"]').first();
    if (await settingsButton.count() > 0) {
      await clickAndWait(settingsButton, 2000);
      await takeScreenshot(page, '06-settings');
    }

    console.log('✨ Screenshot capture complete!');
    console.log(`📁 Screenshots saved to: ${OUTPUT_DIR}`);

  } catch (error) {
    console.error('❌ Error during screenshot capture:', error);
    
    // Take error screenshot for debugging
    await takeScreenshot(page, 'error-state');
  } finally {
    await browser.close();
  }
}

// Run the script
main().catch(console.error);