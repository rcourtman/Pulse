const { chromium } = require('playwright');

async function testUpdateButton() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  
  try {
    console.log('1. Loading Pulse...');
    await page.goto('http://192.168.0.212:7655');
    await page.waitForTimeout(1000);
    
    console.log('2. Navigating to Settings...');
    await page.click('text=Settings');
    await page.waitForTimeout(1000);
    
    console.log('3. Clicking System tab...');
    await page.click('button:has-text("System")');
    await page.waitForTimeout(2000);
    
    console.log('4. Looking for Check for Updates button...');
    const checkButton = page.locator('button:has-text("Check for Updates")');
    if (await checkButton.isVisible()) {
      console.log('✅ Found Check for Updates button');
      
      console.log('5. Clicking Check for Updates...');
      await checkButton.click();
      await page.waitForTimeout(3000);
      
      console.log('6. Looking for update notification...');
      const updateText = await page.textContent('body');
      if (updateText.includes('4.0.99')) {
        console.log('✅ Found update version 4.0.99');
      }
      
      console.log('7. Looking for Apply Update button...');
      const applyButton = page.locator('button:has-text("Apply Update")');
      if (await applyButton.count() > 0) {
        const isVisible = await applyButton.first().isVisible();
        console.log(`✅ Apply Update button found (visible: ${isVisible})`);
        
        if (isVisible) {
          console.log('🎯 SUCCESS: Update button is visible and clickable!');
          await page.screenshot({ path: 'update-button-success.png' });
        }
      } else {
        console.log('❌ No Apply Update button found');
      }
    } else {
      console.log('❌ Check for Updates button not visible');
    }
    
  } catch (error) {
    console.error('Test failed:', error.message);
  } finally {
    await browser.close();
  }
}

testUpdateButton();