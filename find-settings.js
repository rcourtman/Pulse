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
  
  // Find Settings element
  const settingsElement = await page.locator('text="Settings"').first();
  
  if (await settingsElement.isVisible()) {
    console.log('Settings element found!');
    
    // Get bounding box
    const box = await settingsElement.boundingBox();
    console.log('Position:', box);
    
    // Get parent elements
    let current = settingsElement;
    for (let i = 0; i < 5; i++) {
      current = await current.locator('..').first();
      const tag = await current.evaluate(el => el.tagName);
      const className = await current.evaluate(el => el.className);
      const id = await current.evaluate(el => el.id);
      console.log(`Parent ${i+1}: ${tag} class="${className}" id="${id}"`);
    }
    
    // Check if it's clickable
    const isClickable = await settingsElement.evaluate(el => {
      const tag = el.tagName.toLowerCase();
      return tag === 'a' || tag === 'button' || el.onclick !== null || el.style.cursor === 'pointer';
    });
    console.log('Is clickable element:', isClickable);
    
    // Try to click it
    console.log('Attempting to click Settings...');
    try {
      await settingsElement.click();
      await page.waitForTimeout(2000);
      
      // Check if URL changed
      console.log('New URL:', page.url());
      
      // Check if we're on Settings page
      const systemTab = await page.locator('button:has-text("System")').first();
      if (await systemTab.isVisible()) {
        console.log('SUCCESS! Settings page loaded, System tab visible');
        
        // Click System tab
        await systemTab.click();
        await page.waitForTimeout(1000);
        
        // Look for update elements
        const updateElements = await page.locator('text=/Update|Version/').all();
        console.log(`\nFound ${updateElements.length} update-related elements:`);
        for (const el of updateElements) {
          console.log(`- ${await el.textContent()}`);
        }
        
        // Look for Check for Updates button
        const checkButton = await page.locator('button:has-text("Check for Updates")').first();
        if (await checkButton.isVisible()) {
          console.log('\nFound "Check for Updates" button!');
        } else {
          console.log('\n"Check for Updates" button not found');
          
          // List all buttons in System tab
          const buttons = await page.locator('button').all();
          console.log(`\nAll buttons (${buttons.length}):`);
          for (const btn of buttons) {
            const text = await btn.textContent();
            if (text && text.trim()) console.log(`- "${text}"`);
          }
        }
      }
    } catch (error) {
      console.log('Click failed:', error.message);
    }
  } else {
    console.log('Settings element not visible');
  }
  
  // Take screenshot
  await page.screenshot({ path: '/tmp/settings-page.png', fullPage: true });
  console.log('\nScreenshot saved to /tmp/settings-page.png');
  
  await browser.close();
})();