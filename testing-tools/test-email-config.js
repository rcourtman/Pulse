const { chromium } = require('playwright');

async function testEmailConfiguration() {
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const context = await browser.newContext();
  const page = await context.newPage();
  
  // Enable console logging
  page.on('console', msg => {
    if (msg.type() === 'log' || msg.type() === 'error') {
      console.log(`[Browser ${msg.type()}]:`, msg.text());
    }
  });
  
  try {
    // Navigate to Pulse
    console.log('1. Navigating to Pulse dashboard...');
    await page.goto('http://localhost:7655');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);
    
    // Navigate to Alerts
    console.log('2. Clicking Alerts tab...');
    await page.locator('div[role="tab"]:has-text("Alerts")').click();
    await page.waitForTimeout(1000);
    
    // Click on Notifications tab
    console.log('3. Clicking Notifications tab...');
    await page.locator('button:has-text("Notifications")').click();
    await page.waitForTimeout(1000);
    
    // Take screenshot
    await page.screenshot({ path: 'email-config-page.png' });
    console.log('   Screenshot saved: email-config-page.png');
    
    // Check if email is enabled
    const emailFormVisible = await page.locator('text=SMTP Server').isVisible().catch(() => false);
    console.log('4. Email form visible:', emailFormVisible);
    
    if (!emailFormVisible) {
      console.log('   Enabling email notifications...');
      const toggles = await page.locator('button[type="button"]').all();
      if (toggles.length > 0) {
        await toggles[toggles.length - 1].click();
        await page.waitForTimeout(1000);
      }
    }
    
    // Test email sending
    const testEmailButton = page.locator('button:has-text("Send Test Email")');
    if (await testEmailButton.isVisible()) {
      console.log('5. Clicking Send Test Email...');
      await testEmailButton.click();
      
      // Wait for response
      const testSent = await page.locator('text=/sent|success|check your email/i').waitFor({ timeout: 10000 }).then(() => true).catch(() => false);
      console.log('   Test email sent:', testSent);
    }
    
    // Check persistence
    console.log('\n6. Testing persistence...');
    await page.locator('div[role="tab"]:has-text("Main")').click();
    await page.waitForTimeout(1000);
    await page.locator('div[role="tab"]:has-text("Alerts")').click();
    await page.waitForTimeout(1000);
    await page.locator('button:has-text("Notifications")').click();
    await page.waitForTimeout(1000);
    
    const stillVisible = await page.locator('text=SMTP Server').isVisible().catch(() => false);
    console.log('   Email config persisted:', stillVisible);
    
    console.log('\n✅ Email configuration test complete!');
    
  } catch (error) {
    console.error('\nTest error:', error.message);
    await page.screenshot({ path: 'error-email-test.png' });
    throw error;
  } finally {
    await browser.close();
  }
}

// Run the test
if (require.main === module) {
  testEmailConfiguration().catch(err => {
    console.error('\nTest failed:', err);
    process.exit(1);
  });
}

module.exports = { testEmailConfiguration };