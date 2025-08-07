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
  
  // Wait for content to load
  await page.waitForTimeout(3000);
  
  console.log('Page loaded, checking for Settings link...');
  
  // Debug: List all links and buttons
  const links = await page.locator('a').all();
  console.log(`Found ${links.length} links`);
  for (const link of links) {
    const text = await link.textContent();
    const href = await link.getAttribute('href');
    if (text) console.log(`Link: "${text}" -> ${href}`);
  }
  
  // Try different selectors for Settings
  const settingsSelectors = [
    'a:text("Settings")',
    'a[href="/settings"]',
    'a[href="#/settings"]',
    'nav a:nth-child(4)',  // Often settings is the 4th nav item
    '[data-testid="settings-link"]',
    '.nav-link:has-text("Settings")'
  ];
  
  let settingsClicked = false;
  for (const selector of settingsSelectors) {
    try {
      const element = await page.locator(selector).first();
      if (await element.isVisible({ timeout: 1000 })) {
        console.log(`Found Settings with selector: ${selector}`);
        await element.click();
        settingsClicked = true;
        break;
      }
    } catch (e) {
      // Continue to next selector
    }
  }
  
  if (!settingsClicked) {
    // Try clicking by coordinates if we can see it
    console.log('Could not find Settings link, trying navigation by URL');
    await page.goto('http://192.168.0.212:7655/settings', { waitUntil: 'networkidle' });
  }
  
  await page.waitForTimeout(2000);
  
  // Check if we're on Settings page
  const pageTitle = await page.title();
  const pageUrl = page.url();
  console.log(`Current page title: ${pageTitle}`);
  console.log(`Current URL: ${pageUrl}`);
  
  // Look for System tab
  console.log('Looking for System tab...');
  const tabs = await page.locator('button[role="tab"], .tab-button, button.tab').all();
  console.log(`Found ${tabs.length} tabs`);
  for (const tab of tabs) {
    const text = await tab.textContent();
    console.log(`Tab: "${text}"`);
    if (text && text.includes('System')) {
      console.log('Clicking System tab...');
      await tab.click();
      await page.waitForTimeout(1000);
      break;
    }
  }
  
  // Look for version info and update button
  console.log('Looking for version and update info...');
  const versionInfo = await page.locator('text=/Current Version:|Version:/').first();
  if (await versionInfo.isVisible()) {
    const versionText = await versionInfo.textContent();
    console.log('Version info:', versionText);
  }
  
  // Find all buttons and look for update-related ones
  const buttons = await page.locator('button').all();
  console.log(`Found ${buttons.length} buttons`);
  for (const button of buttons) {
    const text = await button.textContent();
    if (text) {
      console.log(`Button: "${text}"`);
      if (text.includes('Update') || text.includes('Check')) {
        console.log(`>>> Found update button: "${text}"`);
        
        // Click it
        await button.click();
        await page.waitForTimeout(3000);
        
        // Check for response
        const alerts = await page.locator('.alert, [role="alert"], .error, .success').all();
        for (const alert of alerts) {
          if (await alert.isVisible()) {
            console.log('Alert:', await alert.textContent());
          }
        }
        
        // Check for modal
        const modal = await page.locator('[role="dialog"], .modal').first();
        if (await modal.isVisible()) {
          console.log('Modal appeared:', await modal.textContent());
          
          // Look for download/confirm button in modal
          const modalButtons = await modal.locator('button').all();
          for (const modalBtn of modalButtons) {
            const btnText = await modalBtn.textContent();
            console.log(`Modal button: "${btnText}"`);
            if (btnText && (btnText.includes('Download') || btnText.includes('Update') || btnText.includes('Yes'))) {
              console.log('Clicking modal button:', btnText);
              await modalBtn.click();
              await page.waitForTimeout(5000);
              break;
            }
          }
        }
        
        break;
      }
    }
  }
  
  // Take final screenshot
  await page.screenshot({ path: '/tmp/pulse-final.png', fullPage: true });
  console.log('Final screenshot saved to /tmp/pulse-final.png');
  
  await browser.close();
})();