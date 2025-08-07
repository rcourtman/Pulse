const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1280, height: 720 }
  });
  
  const page = await context.newPage();
  
  console.log('Opening Pulse at http://192.168.0.212:7655');
  await page.goto('http://192.168.0.212:7655', { waitUntil: 'networkidle' });
  await page.waitForTimeout(3000);
  
  // Debug page structure
  console.log('\n=== Page Title ===');
  console.log(await page.title());
  
  console.log('\n=== All Links ===');
  const allLinks = await page.locator('a').all();
  for (const link of allLinks) {
    const text = await link.textContent();
    const href = await link.getAttribute('href');
    if (text) console.log(`Link: "${text}" -> ${href}`);
  }
  
  console.log('\n=== All Buttons ===');
  const allButtons = await page.locator('button').all();
  for (let i = 0; i < Math.min(10, allButtons.length); i++) {
    const text = await allButtons[i].textContent();
    if (text) console.log(`Button: "${text}"`);
  }
  
  console.log('\n=== Navigation Elements ===');
  const navElements = await page.locator('nav, header, [class*="nav"], [class*="menu"]').all();
  console.log(`Found ${navElements.length} navigation-like elements`);
  
  console.log('\n=== Checking for Settings ===');
  // Try different ways to find Settings
  const settingsSelectors = [
    'text="Settings"',
    'a:has-text("Settings")',
    'button:has-text("Settings")',
    '[href*="settings"]',
    'svg[class*="settings"], svg[class*="gear"], svg[class*="cog"]'
  ];
  
  for (const selector of settingsSelectors) {
    const element = await page.locator(selector).first();
    if (await element.isVisible({ timeout: 500 }).catch(() => false)) {
      console.log(`Found with selector: ${selector}`);
      
      // Try to get parent if it's an icon
      if (selector.includes('svg')) {
        const parent = await element.locator('..').first();
        const parentTag = await parent.evaluate(el => el.tagName);
        console.log(`Parent element: ${parentTag}`);
      }
    }
  }
  
  // Take screenshot
  await page.screenshot({ path: '/tmp/debug-page.png', fullPage: true });
  console.log('\nScreenshot saved to /tmp/debug-page.png');
  
  await browser.close();
})();