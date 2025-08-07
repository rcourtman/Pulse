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
  
  // Look for hamburger menu or settings icon
  console.log('Looking for menu or settings icon...');
  const iconSelectors = [
    '[class*="hamburger"]',
    '[class*="menu-icon"]',
    '[class*="settings-icon"]',
    'button[aria-label*="menu"]',
    'button[aria-label*="settings"]',
    'svg',  // Many icons are SVGs
    '[class*="gear"]',
    '[class*="cog"]'
  ];
  
  for (const selector of iconSelectors) {
    const elements = await page.locator(selector).all();
    if (elements.length > 0) {
      console.log(`Found ${elements.length} elements matching ${selector}`);
      for (const el of elements.slice(0, 3)) { // Check first 3
        try {
          const parent = await el.locator('..').first();
          if (await parent.evaluate(node => node.tagName) === 'BUTTON' || 
              await parent.evaluate(node => node.tagName) === 'A') {
            console.log(`Found clickable icon with parent ${await parent.evaluate(node => node.tagName)}`);
            await parent.click();
            await page.waitForTimeout(1000);
            
            // Check if menu appeared
            const menuItems = await page.locator('a:visible, button:visible').all();
            for (const item of menuItems) {
              const text = await item.textContent();
              if (text && text.includes('Settings')) {
                console.log('Found Settings in menu!');
                await item.click();
                await page.waitForTimeout(2000);
                break;
              }
            }
          }
        } catch (e) {
          // Continue
        }
      }
    }
  }
  
  // Check if we're on a settings page now
  const currentUrl = page.url();
  console.log('Current URL after navigation attempts:', currentUrl);
  
  // Force navigate to settings
  console.log('Force navigating to settings page...');
  await page.goto('http://192.168.0.212:7655/#/settings', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  
  // Now look for System tab
  const systemTab = await page.locator('text="System"').first();
  if (await systemTab.isVisible()) {
    console.log('Found System tab, clicking...');
    await systemTab.click();
    await page.waitForTimeout(1000);
  }
  
  // Look for update information
  const pageContent = await page.content();
  if (pageContent.includes('4.0.9')) {
    console.log('>>> Found reference to version 4.0.9!');
  }
  if (pageContent.includes('Update')) {
    console.log('>>> Found Update text on page');
  }
  
  // Find Check for Updates button
  const checkButton = await page.locator('button:has-text("Check for Updates")').first();
  if (await checkButton.isVisible()) {
    console.log('>>> Found "Check for Updates" button, clicking...');
    await checkButton.click();
    await page.waitForTimeout(3000);
    
    // Check what happened
    const updateInfo = await page.locator('text=/4\\.0\\.9|Update Available|New Version/i').first();
    if (await updateInfo.isVisible()) {
      console.log('Update info appeared:', await updateInfo.textContent());
      
      // Look for Update Now button
      const updateNowButton = await page.locator('button:has-text("Update Now"), button:has-text("Download")').first();
      if (await updateNowButton.isVisible()) {
        console.log('>>> Found Update Now button, clicking...');
        await updateNowButton.click();
        await page.waitForTimeout(5000);
        
        // Check result
        const result = await page.locator('.alert, .error, .success, [role="alert"]').first();
        if (await result.isVisible()) {
          console.log('Result:', await result.textContent());
        }
      }
    }
  } else {
    console.log('Check for Updates button not found');
    
    // List all visible buttons
    const allButtons = await page.locator('button:visible').all();
    console.log(`\nAll visible buttons (${allButtons.length}):`);
    for (const btn of allButtons) {
      const text = await btn.textContent();
      if (text) console.log(`- "${text}"`);
    }
  }
  
  // Take final screenshot
  await page.screenshot({ path: '/tmp/pulse-settings-final.png', fullPage: true });
  console.log('\nFinal screenshot saved to /tmp/pulse-settings-final.png');
  
  await browser.close();
})();