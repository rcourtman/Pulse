const { chromium } = require('playwright');

const FRONTEND_URL = 'http://192.168.0.123:7655';

async function testUIManual() {
  console.log('=== MANUAL UI TEST ===\n');
  console.log('This test will open a browser window for you to manually test.\n');
  
  const browser = await chromium.launch({ 
    headless: false,  // Show browser
    slowMo: 100       // Slow down actions
  });
  
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    // Set up request interception
    page.on('request', request => {
      if (request.url().includes('/api/notifications/test')) {
        console.log('\n📤 Test Email Request:');
        console.log('URL:', request.url());
        console.log('Method:', request.method());
        const body = request.postDataJSON();
        console.log('Body:', JSON.stringify(body, null, 2));
        
        if (body?.config) {
          console.log('\n🔍 Config Analysis:');
          console.log('  Has smtpHost?', 'smtpHost' in body.config, body.config.smtpHost || '');
          console.log('  Has server?', 'server' in body.config);
          console.log('  Recipients:', body.config.to);
          console.log('  Has password?', !!body.config.password);
        }
      }
    });
    
    page.on('response', response => {
      if (response.url().includes('/api/notifications/test')) {
        console.log('\n📥 Test Email Response:');
        console.log('Status:', response.status());
        response.text().then(text => {
          console.log('Body:', text);
          if (response.status() === 200) {
            console.log('✅ SUCCESS: Email test request succeeded!');
          } else {
            console.log('❌ FAILED: Email test request failed!');
          }
        });
      }
    });
    
    // Navigate to alerts page
    console.log('1. Navigating to alerts page...');
    await page.goto(`${FRONTEND_URL}/alerts`);
    
    console.log('\n📋 MANUAL STEPS:');
    console.log('1. Click on the "Notifications" tab');
    console.log('2. Make sure email is enabled');
    console.log('3. Leave recipients empty');
    console.log('4. Click "Send Test Email"');
    console.log('5. Watch the console output here');
    console.log('\nThe browser will stay open for 60 seconds...\n');
    
    // Keep browser open
    await page.waitForTimeout(60000);
    
  } catch (error) {
    console.error('Error:', error.message);
  } finally {
    console.log('\nClosing browser...');
    await browser.close();
  }
}

if (require.main === module) {
  testUIManual().catch(console.error);
}

module.exports = { testUIManual };