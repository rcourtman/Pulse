const puppeteer = require('puppeteer');
const fs = require('fs').promises;
const path = require('path');

const PULSE_URL = process.env.PULSE_URL || 'http://localhost:7655';
const OUTPUT_DIR = path.join(__dirname, '..', 'docs', 'images');

async function cleanExistingScreenshots() {
  try {
    const files = await fs.readdir(OUTPUT_DIR);
    const screenshotFiles = files.filter(f => 
      f.endsWith('.png') && 
      (f.match(/^\d{2}-/) || f.includes('error-state'))
    );
    
    console.log(`Removing ${screenshotFiles.length} existing screenshots...`);
    
    for (const file of screenshotFiles) {
      await fs.unlink(path.join(OUTPUT_DIR, file));
    }
  } catch (error) {
    console.error('Error cleaning screenshots:', error);
  }
}

async function wait(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function main() {
  await cleanExistingScreenshots();
  
  console.log('Starting simple Puppeteer screenshot capture...\n');
  
  // Launch browser with MacBook Air-like display settings
  const browser = await puppeteer.launch({
    headless: 'new',  // Use new headless mode
    defaultViewport: null,  // Don't override viewport
    args: [
      '--window-size=1280,800',  // MacBook Air effective resolution
      '--force-device-scale-factor=2',  // Retina display (2x)
      '--disable-dev-shm-usage',  // Better memory handling
      '--enable-font-antialiasing',  // Smooth fonts
      '--font-render-hinting=none'  // Disable hinting for sharper text
    ]
  });
  
  const page = await browser.newPage();
  
  // CRITICAL: Set viewport BEFORE navigating for sharp rendering!
  // This must happen before page.goto() or text will be blurry
  // MacBook Air 13" display dimensions
  await page.setViewport({
    width: 1280,
    height: 800,
    deviceScaleFactor: 2  // Retina display
  });
  
  try {
    console.log(`Navigating to ${PULSE_URL}...`);
    await page.goto(PULSE_URL, { 
      waitUntil: 'networkidle0',
      timeout: 30000 
    });
    
    // Wait for fonts to fully load for crisp text
    await page.evaluateHandle('document.fonts.ready');
    
    // Wait for content to load
    await wait(5000);
    
    // Set dark theme
    await page.evaluate(() => {
      localStorage.setItem('pulse-theme', 'dark');
      document.documentElement.classList.remove('light');
      document.documentElement.classList.add('dark');
    });
    
    // Force crisp font rendering
    await page.addStyleTag({
      content: `
        * {
          -webkit-font-smoothing: antialiased !important;
          -moz-osx-font-smoothing: grayscale !important;
          text-rendering: optimizeLegibility !important;
        }
      `
    });
    
    await wait(2000);
    
    // Take screenshots of each view
    const views = [
      { name: 'Dashboard', file: '01-dashboard' },
      { name: 'Storage', file: '02-storage' },
      { name: 'Backups', file: '03-backups' },
      { name: 'Alerts', file: '04-alerts' },
      { name: 'Settings', file: '06-settings' }
    ];
    
    for (const view of views) {
      console.log(`Capturing ${view.name}...`);
      
      if (view.name !== 'Dashboard') {
        // Try to click on the tab
        try {
          await page.evaluate((viewName) => {
            const buttons = Array.from(document.querySelectorAll('button, [role="tab"], a'));
            const button = buttons.find(b => b.textContent.includes(viewName));
            if (button) button.click();
          }, view.name);
          
          await wait(3000);
        } catch (e) {
          console.log(`Could not navigate to ${view.name}`);
        }
      }
      
      // Take screenshot - MacBook Air resolution should fit content perfectly
      await page.screenshot({
        path: path.join(OUTPUT_DIR, `${view.file}.png`),
        type: 'png'
      });
      
      console.log(`Saved ${view.file}.png`);
    }
    
    // Note: Active alerts are shown on the main Alerts tab (04-alerts.png)
    // The tab has Config and History sub-tabs, no separate Active tab
    
    // Take alert history screenshot - need to click on History sub-tab
    console.log('Capturing alert history...');
    
    // First make sure we're on the Alerts tab
    await page.evaluate(() => {
      const alertsTab = Array.from(document.querySelectorAll('button, [role="tab"], a'))
        .find(b => b.textContent.includes('Alerts'));
      if (alertsTab) alertsTab.click();
    });
    await wait(2000);
    
    // Now click on the History sub-tab
    await page.evaluate(() => {
      const historyTab = Array.from(document.querySelectorAll('button, [role="tab"], a'))
        .find(b => b.textContent.trim() === 'History' || b.textContent.includes('History'));
      if (historyTab) historyTab.click();
    });
    
    // Wait for alert history to load with retry logic
    console.log('Waiting for alert history to load...');
    let retries = 0;
    const maxRetries = 15; // 15 seconds max
    
    while (retries < maxRetries) {
      await wait(1000);
      
      // Check if data has loaded by looking for alert rows in the DOM
      const hasData = await page.evaluate(() => {
        // Look for alert history rows or the "No alerts" message
        const rows = document.querySelectorAll('tbody tr, [data-testid="alert-row"], .alert-history-row');
        const noDataMsg = Array.from(document.querySelectorAll('*')).find(
          el => el.textContent && el.textContent.includes('No alerts in selected time range')
        );
        return rows.length > 0 || noDataMsg;
      });
      
      if (hasData) {
        console.log('Alert history loaded after', retries + 1, 'seconds');
        break;
      }
      
      retries++;
    }
    
    // Extra wait for any animations
    await wait(1000);
    
    // Take the screenshot - MacBook Air resolution
    await page.screenshot({
      path: path.join(OUTPUT_DIR, '05-alert-history.png'),
      type: 'png'
    });
    console.log('Saved 05-alert-history.png');
    
    // Note: Dashboard is already in dark mode, no need for duplicate or light mode
    
    // Mobile view
    console.log('\nCapturing mobile view...');
    
    // Set mobile viewport BEFORE reload for sharp rendering
    await page.setViewport({
      width: 375,  // iPhone standard width
      height: 667,  // iPhone standard height
      isMobile: true,
      deviceScaleFactor: 2  // 2x for standard retina
    });
    
    // Now reload with the new viewport already set
    await page.reload({ waitUntil: 'networkidle0' });
    
    // Wait for fonts to load again after reload
    await page.evaluateHandle('document.fonts.ready');
    await wait(3000);
    
    // Set dark theme again
    await page.evaluate(() => {
      localStorage.setItem('pulse-theme', 'dark');
      document.documentElement.classList.remove('light');
      document.documentElement.classList.add('dark');
    });
    
    await wait(2000);
    
    await page.screenshot({
      path: path.join(OUTPUT_DIR, '08-mobile.png'),
      type: 'png'
    });
    
    console.log('Saved 08-mobile.png');
    
    console.log('\nScreenshot capture complete!');
    
  } catch (error) {
    console.error('Error during screenshot capture:', error);
  } finally {
    await browser.close();
  }
}

main().catch(console.error);
