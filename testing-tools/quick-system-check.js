const { chromium } = require('playwright');

async function quickCheck() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  
  try {
    console.log('Loading Pulse...');
    await page.goto('http://192.168.0.212:7655');
    await page.waitForTimeout(2000);
    
    console.log('Clicking Settings...');
    await page.click('text=Settings');
    await page.waitForTimeout(1000);
    
    console.log('Looking for System tab...');
    
    // Try different selectors for System tab
    const tabSelectors = [
      'button:has-text("System")',
      'div[role="tab"]:has-text("System")',
      '[data-tab="system"]',
      'text=System'
    ];
    
    let clicked = false;
    for (const selector of tabSelectors) {
      try {
        const element = page.locator(selector).first();
        if (await element.count() > 0) {
          console.log(`Found System tab with selector: ${selector}`);
          await element.click();
          clicked = true;
          break;
        }
      } catch (e) {
        // Continue trying
      }
    }
    
    if (!clicked) {
      console.log('Could not find System tab!');
      // Get all button texts
      const buttons = await page.locator('button').allTextContents();
      console.log('Available buttons:', buttons);
    }
    
    await page.waitForTimeout(2000);
    
    // Take screenshot
    await page.screenshot({ path: 'system-tab.png', fullPage: true });
    
    // Get all text
    const text = await page.textContent('body');
    
    // Check for expected content
    const checks = [
      'Performance',
      'Polling Interval',
      'Backend Port',
      'Updates',
      'Current Version',
      'Check for Updates'
    ];
    
    console.log('\nContent checks:');
    for (const check of checks) {
      if (text.includes(check)) {
        console.log(`✅ Found: "${check}"`);
      } else {
        console.log(`❌ Missing: "${check}"`);
      }
    }
    
    // Get all headings
    const headings = await page.locator('h3, h4').allTextContents();
    console.log('\nHeadings found:', headings);
    
  } finally {
    await browser.close();
  }
}

quickCheck().catch(console.error);