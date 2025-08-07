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
  
  // Check current version
  const versionText = await page.locator('text=/Current Version/').first();
  if (await versionText.isVisible()) {
    console.log('Found:', await versionText.textContent());
  }
  
  // Test 1: Check with Stable channel
  console.log('\n=== Test 1: Checking for updates (Stable channel) ===');
  const stableDropdown = await page.locator('select').filter({ hasText: 'Stable' }).first();
  await stableDropdown.selectOption('stable');
  await page.waitForTimeout(500);
  
  const checkButton = await page.locator('button:has-text("Check for Updates")').first();
  await checkButton.click();
  await page.waitForTimeout(3000);
  
  // Check if update button appears
  const updateNowButton = await page.locator('button:has-text("Apply Update")').first();
  if (await updateNowButton.isVisible({ timeout: 1000 }).catch(() => false)) {
    console.log('✓ Apply Update button appeared!');
  } else {
    console.log('✗ Apply Update button NOT visible');
    // Check for any messages
    const alertText = await page.locator('.alert, text=/latest version/i').first();
    if (await alertText.isVisible({ timeout: 500 }).catch(() => false)) {
      console.log('Message:', await alertText.textContent());
    }
  }
  
  // Test 2: Check with RC channel
  console.log('\n=== Test 2: Switching to RC channel and checking ===');
  await stableDropdown.selectOption('rc');
  await page.waitForTimeout(500);
  
  await checkButton.click();
  await page.waitForTimeout(3000);
  
  const updateNowButton2 = await page.locator('button:has-text("Apply Update")').first();
  if (await updateNowButton2.isVisible({ timeout: 1000 }).catch(() => false)) {
    console.log('✓ Apply Update button appeared for RC!');
  } else {
    console.log('✗ Apply Update button NOT visible for RC');
  }
  
  console.log('\nTest complete! Browser will close in 5 seconds...');
  await page.waitForTimeout(5000);
  
  await browser.close();
})();