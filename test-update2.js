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
  page.on('console', msg => {
    if (!msg.text().includes('WebSocket')) {
      console.log('Browser console:', msg.text());
    }
  });
  page.on('pageerror', err => console.log('Page error:', err.message));
  
  console.log('Opening Pulse at http://192.168.0.212:7655');
  await page.goto('http://192.168.0.212:7655');
  
  // Wait for the page to load
  await page.waitForTimeout(3000);
  
  // Take initial screenshot
  await page.screenshot({ path: '/tmp/pulse-main.png', fullPage: true });
  console.log('Main page screenshot saved to /tmp/pulse-main.png');
  
  // Check for version in header
  console.log('Looking for version display...');
  const versionTexts = await page.locator('text=/4\\.0\\.8/').all();
  console.log(`Found ${versionTexts.length} elements with version 4.0.8`);
  
  // Look for update notification banner
  console.log('Looking for update notification...');
  const updateBanner = await page.locator('.update-banner, [class*="update"], [class*="notification"]').all();
  for (const banner of updateBanner) {
    if (await banner.isVisible()) {
      const text = await banner.textContent();
      console.log('Found banner:', text);
    }
  }
  
  // Try clicking on Settings
  console.log('Navigating to Settings...');
  const settingsButton = await page.locator('nav a:has-text("Settings"), button:has-text("Settings"), a[href*="settings"]').first();
  if (await settingsButton.isVisible()) {
    await settingsButton.click();
    await page.waitForTimeout(2000);
    
    await page.screenshot({ path: '/tmp/pulse-settings.png', fullPage: true });
    console.log('Settings screenshot saved to /tmp/pulse-settings.png');
    
    // Look for System tab
    const systemTab = await page.locator('button:has-text("System"), [role="tab"]:has-text("System")').first();
    if (await systemTab.isVisible()) {
      console.log('Clicking System tab...');
      await systemTab.click();
      await page.waitForTimeout(1000);
      
      // Look for update section
      const updateSection = await page.locator('text=/Updates|Version|Current Version/').all();
      for (const section of updateSection) {
        if (await section.isVisible()) {
          console.log('Found text:', await section.textContent());
        }
      }
      
      // Look for update button
      const updateButtons = await page.locator('button').all();
      for (const button of updateButtons) {
        const text = await button.textContent();
        if (text && (text.includes('Update') || text.includes('Check') || text.includes('Download'))) {
          console.log('Found button:', text);
          
          // Click update-related button
          if (text.includes('Check for Updates') || text.includes('Update Now')) {
            console.log('Clicking button:', text);
            await button.click();
            await page.waitForTimeout(3000);
            
            // Take screenshot after clicking
            await page.screenshot({ path: '/tmp/pulse-after-update-click.png', fullPage: true });
            console.log('Screenshot after update click saved');
            
            // Look for any modal or dialog
            const modals = await page.locator('[role="dialog"], .modal, [class*="modal"]').all();
            for (const modal of modals) {
              if (await modal.isVisible()) {
                const modalText = await modal.textContent();
                console.log('Modal content:', modalText);
              }
            }
          }
        }
      }
    }
  } else {
    console.log('Settings button not found');
  }
  
  await browser.close();
})();