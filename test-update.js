const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const context = await browser.newContext({
    ignoreHTTPSErrors: true
  });
  
  const page = await context.newPage();
  
  // Enable console logging
  page.on('console', msg => console.log('Browser console:', msg.text()));
  page.on('pageerror', err => console.log('Page error:', err.message));
  
  console.log('Opening Pulse at http://192.168.0.212:7655');
  await page.goto('http://192.168.0.212:7655');
  
  // Wait for the page to load
  await page.waitForTimeout(2000);
  
  // Check current version
  const versionElement = await page.locator('text=/v4\\.0\\.\\d+/').first();
  if (versionElement) {
    const currentVersion = await versionElement.textContent();
    console.log('Current version displayed:', currentVersion);
  }
  
  // Look for update notification
  const updateNotification = await page.locator('text=/update available/i').first();
  if (await updateNotification.isVisible()) {
    console.log('Update notification is visible');
    
    // Click on the update notification or button
    const updateButton = await page.locator('button:has-text("Update")').first();
    if (await updateButton.isVisible()) {
      console.log('Found Update button, clicking...');
      await updateButton.click();
      
      // Wait for update modal or process
      await page.waitForTimeout(2000);
      
      // Check for any error messages
      const errorMessages = await page.locator('.error, [class*="error"], text=/error/i').all();
      for (const error of errorMessages) {
        if (await error.isVisible()) {
          console.log('Error found:', await error.textContent());
        }
      }
      
      // Check for confirmation dialog
      const confirmButton = await page.locator('button:has-text("Confirm"), button:has-text("Yes"), button:has-text("Download")').first();
      if (await confirmButton.isVisible()) {
        console.log('Found confirmation button, clicking...');
        await confirmButton.click();
        await page.waitForTimeout(3000);
      }
      
      // Check final status
      const successMessage = await page.locator('text=/success|complete|updated/i').first();
      if (await successMessage.isVisible()) {
        console.log('Success message:', await successMessage.textContent());
      }
    } else {
      console.log('No Update button found');
    }
  } else {
    console.log('No update notification visible');
    
    // Try to trigger update check manually
    console.log('Opening Settings page...');
    const settingsLink = await page.locator('a[href*="settings"], button:has-text("Settings")').first();
    if (await settingsLink.isVisible()) {
      await settingsLink.click();
      await page.waitForTimeout(2000);
      
      // Look for update section in settings
      const updateSection = await page.locator('text=/update|version/i').first();
      if (await updateSection.isVisible()) {
        console.log('Found update section in settings');
        
        // Look for check updates button
        const checkButton = await page.locator('button:has-text("Check"), button:has-text("Update")').first();
        if (await checkButton.isVisible()) {
          console.log('Found check updates button, clicking...');
          await checkButton.click();
          await page.waitForTimeout(3000);
          
          // Check for update modal or message
          const updateInfo = await page.locator('text=/4\\.0\\.9|new version|update available/i').first();
          if (await updateInfo.isVisible()) {
            console.log('Update info:', await updateInfo.textContent());
          }
        }
      }
    }
  }
  
  // Take a screenshot for debugging
  await page.screenshot({ path: '/tmp/pulse-update-test.png', fullPage: true });
  console.log('Screenshot saved to /tmp/pulse-update-test.png');
  
  await browser.close();
})();