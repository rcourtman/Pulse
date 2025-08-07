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
  await page.waitForTimeout(2000);
  
  // Click on Settings in the navigation
  console.log('Looking for Settings link...');
  const settingsLink = await page.locator('nav a').filter({ hasText: 'Settings' }).first();
  
  if (await settingsLink.isVisible()) {
    console.log('Found Settings link, clicking...');
    await settingsLink.click();
    await page.waitForTimeout(2000);
    
    // Look for System tab
    const systemTab = await page.locator('button').filter({ hasText: 'System' }).first();
    if (await systemTab.isVisible()) {
      console.log('Found System tab, clicking...');
      await systemTab.click();
      await page.waitForTimeout(1000);
      
      // Look for update section
      const updateSection = await page.locator('text=/Current Version|Updates/').first();
      if (await updateSection.isVisible()) {
        console.log('Found update section');
        
        // Check for update button
        const checkButton = await page.locator('button').filter({ hasText: 'Check for Updates' }).first();
        if (await checkButton.isVisible()) {
          console.log('Found Check for Updates button');
          await checkButton.click();
          await page.waitForTimeout(3000);
          
          // Check what happened
          const updateStatus = await page.locator('text=/up to date|available|4\\.0\\.9/i').first();
          if (await updateStatus.isVisible()) {
            console.log('Update status:', await updateStatus.textContent());
          }
        } else {
          console.log('Check for Updates button not found');
        }
      } else {
        console.log('Update section not found');
      }
    } else {
      console.log('System tab not found');
    }
  } else {
    console.log('Settings link not visible');
    
    // List all nav links
    const navLinks = await page.locator('nav a').all();
    console.log(`Found ${navLinks.length} nav links:`);
    for (const link of navLinks) {
      console.log(`- ${await link.textContent()}`);
    }
  }
  
  // Take screenshot
  await page.screenshot({ path: '/tmp/settings-test.png', fullPage: true });
  console.log('Screenshot saved to /tmp/settings-test.png');
  
  await browser.close();
})();