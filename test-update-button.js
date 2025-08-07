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
    if (!msg.text().includes('WebSocket')) {
      console.log('Browser console:', msg.text());
    }
  });
  page.on('pageerror', err => console.log('Page error:', err.message));
  
  console.log('Opening Pulse and navigating to Settings...');
  await page.goto('http://192.168.0.212:7655', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  
  // Click Settings
  await page.locator('text="Settings"').first().click();
  await page.waitForTimeout(1000);
  
  // Click System tab
  await page.locator('button:has-text("System")').first().click();
  await page.waitForTimeout(1000);
  
  console.log('\n=== Current Update Status ===');
  
  // Check current version display
  const versionText = await page.locator('text=/Current Version.*4\\.0\\.9/').first();
  if (await versionText.isVisible()) {
    console.log('Current version shown:', await versionText.textContent());
  }
  
  // Check for existing update status
  const statusText = await page.locator('text=/You are running the latest version|Update available/').first();
  if (await statusText.isVisible({ timeout: 1000 }).catch(() => false)) {
    console.log('Update status:', await statusText.textContent());
  }
  
  console.log('\n=== Clicking Check for Updates ===');
  
  // Click Check for Updates button
  const checkButton = await page.locator('button:has-text("Check for Updates")').first();
  await checkButton.click();
  
  // Wait for response
  await page.waitForTimeout(3000);
  
  // Check what happened
  console.log('\n=== After Update Check ===');
  
  // Look for any status messages
  const messages = [
    'text=/You are running the latest version/',
    'text=/Update available/',
    'text=/Checking for updates/',
    'text=/Failed to check/',
    'text=/Error/',
    '.alert',
    '[role="alert"]'
  ];
  
  for (const selector of messages) {
    const element = await page.locator(selector).first();
    if (await element.isVisible({ timeout: 500 }).catch(() => false)) {
      const text = await element.textContent();
      console.log(`Found: ${text}`);
    }
  }
  
  // Check if Update Now button appeared
  const updateNowButton = await page.locator('button:has-text("Update Now")').first();
  if (await updateNowButton.isVisible({ timeout: 500 }).catch(() => false)) {
    console.log('\n"Update Now" button appeared!');
    console.log('This means an update was detected but we are already on latest version');
  }
  
  // Check button state
  const isButtonDisabled = await checkButton.isDisabled();
  console.log(`\nCheck button is ${isButtonDisabled ? 'disabled' : 'enabled'}`);
  
  // Take screenshot
  await page.screenshot({ path: '/tmp/update-check-result.png', fullPage: true });
  console.log('\nScreenshot saved to /tmp/update-check-result.png');
  
  await browser.close();
})();