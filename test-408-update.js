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
  
  // Enable console logging
  page.on('console', msg => {
    const text = msg.text();
    if (!text.includes('WebSocket')) {
      console.log('Browser:', text);
    }
  });
  page.on('pageerror', err => console.log('Page error:', err.message));
  page.on('response', response => {
    if (response.url().includes('/api/updates')) {
      console.log(`API ${response.url()} -> ${response.status()}`);
    }
  });
  
  console.log('Opening Pulse v4.0.8 at http://192.168.0.212:7655');
  await page.goto('http://192.168.0.212:7655', { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(2000);
  
  // Click Settings
  console.log('\nNavigating to Settings...');
  await page.locator('text="Settings"').first().click();
  await page.waitForTimeout(1000);
  
  // Click System tab
  console.log('Clicking System tab...');
  await page.locator('button:has-text("System")').first().click();
  await page.waitForTimeout(1000);
  
  // Check current version display
  const versionText = await page.locator('text=/Current Version.*4\\.0\\.8/').first();
  if (await versionText.isVisible()) {
    console.log('Current version:', await versionText.textContent());
  }
  
  // Take screenshot before update check
  await page.screenshot({ path: '/tmp/before-check.png' });
  console.log('Screenshot saved: /tmp/before-check.png');
  
  // Click Check for Updates
  console.log('\nClicking "Check for Updates"...');
  const checkButton = await page.locator('button:has-text("Check for Updates")').first();
  await checkButton.click();
  
  // Wait for update check to complete
  await page.waitForTimeout(3000);
  
  // Take screenshot after check
  await page.screenshot({ path: '/tmp/after-check.png' });
  console.log('Screenshot saved: /tmp/after-check.png');
  
  // Check what happened
  console.log('\n=== Update Check Results ===');
  
  // Look for update available message
  const updateAvailable = await page.locator('text=/Update available|4\\.0\\.9|new version/i').all();
  for (const elem of updateAvailable) {
    if (await elem.isVisible()) {
      console.log('Found:', await elem.textContent());
    }
  }
  
  // Check if Update Now button appeared
  const updateNowButton = await page.locator('button:has-text("Update Now")').first();
  if (await updateNowButton.isVisible()) {
    console.log('\n"Update Now" button is visible!');
    console.log('Clicking "Update Now"...');
    
    await updateNowButton.click();
    await page.waitForTimeout(5000);
    
    // Take screenshot after update attempt
    await page.screenshot({ path: '/tmp/after-update.png' });
    console.log('Screenshot saved: /tmp/after-update.png');
    
    // Check for any error or success messages
    const messages = await page.locator('.alert, [role="alert"], text=/error|success|failed|complete/i').all();
    for (const msg of messages) {
      if (await msg.isVisible()) {
        console.log('Message:', await msg.textContent());
      }
    }
    
    // Check update status
    const statusResponse = await page.evaluate(() => 
      fetch('/api/updates/status').then(r => r.json())
    );
    console.log('\nUpdate status:', statusResponse);
  } else {
    console.log('\n"Update Now" button NOT visible');
    
    // List all visible buttons
    const buttons = await page.locator('button:visible').all();
    console.log(`\nAll visible buttons (${buttons.length}):`);
    for (const btn of buttons) {
      const text = await btn.textContent();
      if (text && text.trim()) console.log(`- "${text}"`);
    }
  }
  
  await browser.close();
})();