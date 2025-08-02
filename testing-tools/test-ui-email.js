const { chromium } = require('playwright');

const FRONTEND_URL = 'http://192.168.0.123:7655';

async function testUIEmail() {
  console.log('=== TESTING EMAIL VIA UI ===\n');
  
  const browser = await chromium.launch({ 
    headless: true
  });
  
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    // Navigate to alerts page
    console.log('1. Navigating to alerts page...');
    await page.goto(`${FRONTEND_URL}/alerts`);
    await page.waitForTimeout(2000);
    
    // Click on Notifications tab
    console.log('2. Clicking Notifications tab...');
    await page.click('button:has-text("Notifications")');
    await page.waitForTimeout(1000);
    
    // Click Send Test Email
    console.log('3. Clicking Send Test Email...');
    await page.click('button:has-text("Send Test Email")');
    
    // Wait for response
    await page.waitForTimeout(3000);
    
    // Check for success notification
    const toasts = await page.locator('.toast-notification, .notification, [role="alert"]').allTextContents();
    if (toasts.length > 0) {
      console.log('4. Notifications received:', toasts);
    }
    
    // Check console for errors
    const errors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });
    
    await page.waitForTimeout(1000);
    
    if (errors.length > 0) {
      console.log('\n❌ Console errors:', errors);
    } else {
      console.log('\n✅ No console errors detected');
    }
    
    console.log('\n✅ Test completed - check your email at courtmanr@gmail.com');
    
  } catch (error) {
    console.error('Error:', error.message);
  } finally {
    await browser.close();
  }
}

if (require.main === module) {
  testUIEmail().catch(console.error);
}

module.exports = { testUIEmail };