const { chromium } = require('playwright');
const axios = require('axios');

const FRONTEND_URL = 'http://localhost:7655';

async function testUIIntegration() {
  console.log('=== UI INTEGRATION TEST ===\n');
  
  const browser = await chromium.launch({ 
    headless: true
  });
  
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    let capturedRequest = null;
    
    // Intercept the test email request
    await page.route('**/api/notifications/test', async (route, request) => {
      capturedRequest = {
        url: request.url(),
        method: request.method(),
        headers: request.headers(),
        body: request.postDataJSON()
      };
      
      // Let it continue to the real backend
      await route.continue();
    });
    
    // Navigate and click test email
    console.log('1. Navigating to alerts page...');
    await page.goto(`${FRONTEND_URL}/alerts`);
    await page.waitForSelector('button:has-text("Notifications")', { timeout: 5000 });
    
    console.log('2. Clicking Notifications tab...');
    await page.click('button:has-text("Notifications")');
    await page.waitForTimeout(1000);
    
    console.log('3. Clicking Send Test Email...');
    await page.click('button:has-text("Send Test Email")');
    await page.waitForTimeout(2000);
    
    // Check what was sent
    if (capturedRequest) {
      console.log('\n✅ Captured request:');
      console.log(JSON.stringify(capturedRequest, null, 2));
      
      // Verify field names
      if (capturedRequest.body?.config) {
        const config = capturedRequest.body.config;
        console.log('\nField validation:');
        console.log('  Has smtpHost?', 'smtpHost' in config);
        console.log('  Has server?', 'server' in config);
        console.log('  Has smtpPort?', 'smtpPort' in config);
        console.log('  Has port?', 'port' in config);
      }
    } else {
      console.log('\n❌ No request captured');
    }
    
    // Also test backend directly with same data
    if (capturedRequest?.body) {
      console.log('\n4. Testing same payload directly against backend...');
      try {
        const response = await axios.post('http://localhost:3000/api/notifications/test', capturedRequest.body);
        console.log('✅ Backend accepted the payload');
      } catch (error) {
        console.log('❌ Backend rejected the payload:', error.response?.data);
      }
    }
    
  } catch (error) {
    console.error('Error:', error.message);
  } finally {
    await browser.close();
  }
}

if (require.main === module) {
  testUIIntegration().catch(console.error);
}

module.exports = { testUIIntegration };