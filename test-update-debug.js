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
  
  // Log API responses
  page.on('response', response => {
    if (response.url().includes('/api/updates/check')) {
      response.json().then(data => {
        console.log('API Response:', JSON.stringify(data, null, 2));
      }).catch(() => {});
    }
  });
  
  // Log console messages
  page.on('console', msg => {
    if (msg.type() === 'error') {
      console.log('Browser error:', msg.text());
    }
  });
  
  console.log('Opening Pulse UI at http://localhost:7655');
  await page.goto('http://localhost:7655', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  
  // Click Settings
  console.log('\nNavigating to Settings...');
  await page.locator('text="Settings"').first().click();
  await page.waitForTimeout(1000);
  
  // Click System tab
  console.log('Clicking System tab...');
  await page.locator('button:has-text("System")').first().click();
  await page.waitForTimeout(1000);
  
  // Click Check for Updates
  console.log('\nClicking Check for Updates...');
  const checkButton = await page.locator('button:has-text("Check for Updates")').first();
  await checkButton.click();
  await page.waitForTimeout(3000);
  
  // Look for all elements that might contain update info
  console.log('\n=== Looking for update-related elements ===');
  
  // Check for success message
  const successMsg = await page.locator('.alert, text=/You are running the latest version/i').all();
  for (const msg of successMsg) {
    if (await msg.isVisible()) {
      console.log('Found message:', await msg.textContent());
    }
  }
  
  // Check for update available message
  const updateMsg = await page.locator('text=/Update available/i').all();
  for (const msg of updateMsg) {
    if (await msg.isVisible()) {
      console.log('Found update message:', await msg.textContent());
    }
  }
  
  // Check for Apply Update button
  const applyButton = await page.locator('button:has-text("Apply Update")').first();
  if (await applyButton.isVisible({ timeout: 500 }).catch(() => false)) {
    console.log('✓ Apply Update button is visible!');
  } else {
    console.log('✗ Apply Update button NOT visible');
  }
  
  // Check all visible buttons
  const allButtons = await page.locator('button:visible').all();
  console.log(`\nAll visible buttons (${allButtons.length}):`);
  for (const btn of allButtons) {
    const text = await btn.textContent();
    if (text && text.trim()) {
      console.log(`- "${text.trim()}"`);
    }
  }
  
  // Take screenshot
  await page.screenshot({ path: '/tmp/update-debug.png', fullPage: true });
  console.log('\nScreenshot saved to /tmp/update-debug.png');
  
  await browser.close();
})();