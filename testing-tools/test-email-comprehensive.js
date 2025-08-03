const { chromium } = require('playwright');
const axios = require('axios');
const { spawn } = require('child_process');

const FRONTEND_URL = 'http://localhost:7655';
const BACKEND_URL = 'http://localhost:3000';

async function runComprehensiveEmailTest() {
  console.log('=== COMPREHENSIVE EMAIL TEST ===\n');
  
  const results = {
    passed: [],
    failed: []
  };
  
  // Test 1: Backend API - Empty recipients
  console.log('TEST 1: Backend API with empty recipients');
  try {
    const response = await axios.post(`${BACKEND_URL}/api/notifications/test`, {
      method: 'email',
      config: {
        enabled: true,
        smtpHost: 'smtp.gmail.com',
        smtpPort: 587,
        username: 'test@example.com',
        password: 'zlff ruyk bxxf cxch',
        from: 'test@example.com',
        to: [],
        tls: true
      }
    });
    
    if (response.data.status === 'success') {
      results.passed.push('Backend accepts empty recipients');
      console.log('✅ PASS: Backend accepts empty recipients');
    } else {
      results.failed.push('Backend response not success');
      console.log('❌ FAIL: Backend response not success');
    }
  } catch (error) {
    results.failed.push(`Backend API error: ${error.response?.data || error.message}`);
    console.log('❌ FAIL:', error.response?.data || error.message);
  }
  
  // Test 2: Check saved configuration
  console.log('\nTEST 2: Check saved email configuration');
  try {
    const response = await axios.get(`${BACKEND_URL}/api/notifications/email`);
    const config = response.data;
    
    if (config.from === 'test@example.com') {
      results.passed.push('Email configuration properly saved');
      console.log('✅ PASS: Email configuration found');
      console.log('  From:', config.from);
      console.log('  Recipients:', config.to);
    } else {
      results.failed.push('Email configuration not found');
      console.log('❌ FAIL: Email configuration not found');
    }
  } catch (error) {
    results.failed.push(`Config load error: ${error.message}`);
    console.log('❌ FAIL:', error.message);
  }
  
  // Test 3: UI Integration Test
  console.log('\nTEST 3: UI Integration - Send Test Email');
  const browser = await chromium.launch({ 
    headless: true,
    timeout: 30000
  });
  
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    let requestCaptured = false;
    let requestPayload = null;
    let responseStatus = null;
    
    // Intercept the API request
    await page.route('**/api/notifications/test', async (route, request) => {
      requestCaptured = true;
      requestPayload = request.postDataJSON();
      
      const response = await route.fetch();
      responseStatus = response.status();
      await route.fulfill({ response });
    });
    
    // Navigate to alerts page
    await page.goto(`${FRONTEND_URL}/alerts`);
    await page.waitForLoadState('networkidle');
    
    // Click Notifications tab
    const notifTab = await page.waitForSelector('button:has-text("Notifications")', { timeout: 5000 });
    await notifTab.click();
    await page.waitForTimeout(1000);
    
    // Click Send Test Email
    const testButton = await page.waitForSelector('button:has-text("Send Test Email")', { timeout: 5000 });
    await testButton.click();
    
    // Wait for request to complete
    await page.waitForTimeout(3000);
    
    if (requestCaptured) {
      console.log('✅ PASS: Request captured');
      results.passed.push('UI sends test email request');
      
      // Verify request payload
      if (requestPayload?.config) {
        const config = requestPayload.config;
        console.log('\nRequest payload validation:');
        
        // Check field names
        if ('smtpHost' in config && 'smtpPort' in config) {
          console.log('✅ PASS: Correct field names (smtpHost, smtpPort)');
          results.passed.push('UI uses correct field names');
        } else if ('server' in config || 'port' in config) {
          console.log('❌ FAIL: Wrong field names (server/port instead of smtpHost/smtpPort)');
          results.failed.push('UI uses wrong field names');
        }
        
        // Check recipients
        if (Array.isArray(config.to) && config.to.length === 0) {
          console.log('✅ PASS: Recipients array is empty');
          results.passed.push('UI sends empty recipients array');
        } else {
          console.log('❌ FAIL: Recipients not empty:', config.to);
          results.failed.push('UI adds recipients when should be empty');
        }
        
        // Check response
        if (responseStatus === 200) {
          console.log('✅ PASS: Backend accepted request (200 OK)');
          results.passed.push('Backend accepts UI request');
        } else {
          console.log('❌ FAIL: Backend rejected request:', responseStatus);
          results.failed.push(`Backend rejected UI request: ${responseStatus}`);
        }
      }
    } else {
      console.log('❌ FAIL: No request captured');
      results.failed.push('UI did not send test email request');
    }
    
    // Check for UI alerts/notifications
    const alerts = await page.locator('[role="alert"], .alert, .notification').allTextContents();
    if (alerts.length > 0) {
      console.log('\nUI Notifications:', alerts);
      if (alerts.some(a => a.toLowerCase().includes('success'))) {
        results.passed.push('UI shows success message');
      } else if (alerts.some(a => a.toLowerCase().includes('fail') || a.toLowerCase().includes('error'))) {
        results.failed.push(`UI shows error: ${alerts.join(', ')}`);
      }
    }
    
  } catch (error) {
    results.failed.push(`UI test error: ${error.message}`);
    console.log('❌ FAIL:', error.message);
  } finally {
    await browser.close();
  }
  
  // Test 4: Backend Log Verification
  console.log('\nTEST 4: Backend Log Verification');
  const logProcess = spawn('sudo', ['tail', '-100', '/opt/pulse/pulse.log']);
  
  let logs = '';
  logProcess.stdout.on('data', (data) => {
    logs += data.toString();
  });
  
  await new Promise((resolve) => {
    logProcess.on('close', resolve);
    setTimeout(() => logProcess.kill(), 2000);
  });
  
  // Check logs for email sending
  if (logs.includes('Using From address as recipient since To is empty')) {
    console.log('✅ PASS: Backend uses From address for empty recipients');
    results.passed.push('Backend handles empty recipients correctly');
  }
  
  if (logs.includes('Email notification sent successfully')) {
    console.log('✅ PASS: Email sent successfully');
    results.passed.push('Email actually sent');
  } else if (logs.includes('Failed to send email')) {
    console.log('❌ FAIL: Email send failed');
    results.failed.push('Email send failed');
  }
  
  // Summary
  console.log('\n=== TEST SUMMARY ===');
  console.log(`Total tests: ${results.passed.length + results.failed.length}`);
  console.log(`Passed: ${results.passed.length}`);
  console.log(`Failed: ${results.failed.length}`);
  
  if (results.passed.length > 0) {
    console.log('\nPASSED TESTS:');
    results.passed.forEach(test => console.log('  ✅', test));
  }
  
  if (results.failed.length > 0) {
    console.log('\nFAILED TESTS:');
    results.failed.forEach(test => console.log('  ❌', test));
  }
  
  return results.failed.length === 0;
}

// Run the test
runComprehensiveEmailTest()
  .then(success => {
    console.log(success ? '\n✅ ALL TESTS PASSED!' : '\n❌ SOME TESTS FAILED!');
    process.exit(success ? 0 : 1);
  })
  .catch(error => {
    console.error('\nTest suite error:', error);
    process.exit(1);
  });