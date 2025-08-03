const { chromium } = require('playwright');

const FRONTEND_URL = 'http://localhost:7655';

async function testFrontendPayload() {
  console.log('=== TESTING FRONTEND EMAIL PAYLOAD ===\n');
  
  const browser = await chromium.launch({ 
    headless: true
  });
  
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    // Intercept the API request to see what's being sent
    page.on('request', request => {
      if (request.url().includes('/api/notifications/test')) {
        console.log('Test email request intercepted:');
        console.log('  URL:', request.url());
        console.log('  Method:', request.method());
        console.log('  Headers:', request.headers());
        console.log('  Body:', request.postData());
        
        if (request.postData()) {
          try {
            const body = JSON.parse(request.postData());
            console.log('\nParsed body:');
            console.log(JSON.stringify(body, null, 2));
            
            if (body.config) {
              console.log('\nEmail config details:');
              console.log('  From:', body.config.from);
              console.log('  To:', body.config.to);
              console.log('  To length:', body.config.to ? body.config.to.length : 0);
              console.log('  Has password:', !!body.config.password);
            }
          } catch (e) {
            console.log('Could not parse body as JSON');
          }
        }
      }
    });
    
    // Monitor responses
    page.on('response', response => {
      if (response.url().includes('/api/notifications/test')) {
        console.log('\nTest email response:');
        console.log('  Status:', response.status());
        response.text().then(text => {
          console.log('  Body:', text);
        });
      }
    });
    
    // Navigate to alerts page
    console.log('1. Navigating to alerts page...');
    await page.goto(`${FRONTEND_URL}/alerts`);
    await page.waitForTimeout(2000);
    
    // Click on Notifications tab
    console.log('2. Clicking Notifications tab...');
    await page.click('button:has-text("Notifications")');
    await page.waitForTimeout(1000);
    
    // Click Send Test Email
    console.log('3. Clicking Send Test Email...\n');
    await page.click('button:has-text("Send Test Email")');
    
    // Wait for request/response
    await page.waitForTimeout(3000);
    
  } catch (error) {
    console.error('Error:', error.message);
  } finally {
    await browser.close();
  }
}

if (require.main === module) {
  testFrontendPayload().catch(console.error);
}

module.exports = { testFrontendPayload };