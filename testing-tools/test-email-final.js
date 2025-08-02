const axios = require('axios');
const { spawn } = require('child_process');

const BACKEND_URL = 'http://localhost:3000';

async function testEmailFunctionality() {
  console.log('=== FINAL EMAIL FUNCTIONALITY TEST ===\n');
  
  let allPassed = true;
  
  // Test 1: Backend API directly
  console.log('TEST 1: Direct Backend API Test');
  try {
    // First, ensure we have the saved config with password
    console.log('  Setting up email config with password...');
    await axios.put(`${BACKEND_URL}/api/notifications/email`, {
      enabled: true,
      smtpHost: 'smtp.gmail.com',
      smtpPort: 587,
      username: 'courtmanr@gmail.com',
      password: 'zlff ruyk bxxf cxch',
      from: 'courtmanr@gmail.com',
      to: [],  // Empty recipients
      tls: true
    });
    
    // Test sending without config (uses saved config)
    console.log('  Testing with saved config...');
    const response = await axios.post(`${BACKEND_URL}/api/notifications/test`, {
      method: 'email'
    });
    
    if (response.data.status === 'success') {
      console.log('  ✅ PASS: Email sent using saved config');
    } else {
      console.log('  ❌ FAIL: Unexpected response:', response.data);
      allPassed = false;
    }
  } catch (error) {
    console.log('  ❌ FAIL:', error.response?.data || error.message);
    allPassed = false;
  }
  
  // Test 2: Monitor backend logs for actual email send
  console.log('\nTEST 2: Backend Email Sending Verification');
  
  // Start log monitor
  const logMonitor = spawn('sudo', ['tail', '-f', '/opt/pulse/pulse.log']);
  let logOutput = '';
  
  logMonitor.stdout.on('data', (data) => {
    logOutput += data.toString();
  });
  
  // Send test email
  try {
    console.log('  Sending test email and monitoring logs...');
    await axios.post(`${BACKEND_URL}/api/notifications/test`, {
      method: 'email'
    });
    
    // Wait for logs
    await new Promise(resolve => setTimeout(resolve, 3000));
    logMonitor.kill();
    
    // Check log results
    const checks = [
      {
        pattern: 'Using From address as recipient since To is empty',
        description: 'Backend uses From address for empty recipients'
      },
      {
        pattern: 'Attempting to send email via SMTP',
        description: 'SMTP connection attempt'
      },
      {
        pattern: 'Email notification sent successfully',
        description: 'Email sent successfully'
      },
      {
        pattern: 'courtmanr@gmail.com',
        description: 'Correct recipient (From address)'
      }
    ];
    
    checks.forEach(check => {
      if (logOutput.includes(check.pattern)) {
        console.log(`  ✅ PASS: ${check.description}`);
      } else {
        console.log(`  ❌ FAIL: ${check.description}`);
        allPassed = false;
      }
    });
    
    // Check for errors
    if (logOutput.includes('Failed to send email')) {
      console.log('  ❌ FAIL: Email send failed in logs');
      allPassed = false;
    }
    
  } catch (error) {
    console.log('  ❌ FAIL:', error.response?.data || error.message);
    allPassed = false;
  }
  
  // Test 3: Frontend payload format
  console.log('\nTEST 3: Frontend Payload Format Test');
  console.log('  Testing the exact payload format the UI sends...');
  
  try {
    // This simulates what the fixed frontend sends
    const frontendPayload = {
      method: 'email',
      config: {
        enabled: true,
        smtpHost: 'smtp.gmail.com',
        smtpPort: 587,
        username: 'courtmanr@gmail.com',
        password: 'zlff ruyk bxxf cxch',
        from: 'courtmanr@gmail.com',
        to: [],  // Empty as fixed
        tls: true
      }
    };
    
    const response = await axios.post(`${BACKEND_URL}/api/notifications/test`, frontendPayload);
    
    if (response.status === 200) {
      console.log('  ✅ PASS: Backend accepts frontend payload format');
    } else {
      console.log('  ❌ FAIL: Backend rejected frontend payload');
      allPassed = false;
    }
  } catch (error) {
    console.log('  ❌ FAIL:', error.response?.data || error.message);
    allPassed = false;
  }
  
  // Summary
  console.log('\n=== TEST SUMMARY ===');
  if (allPassed) {
    console.log('✅ ALL TESTS PASSED!');
    console.log('\nEmail functionality is working correctly:');
    console.log('- Backend accepts empty recipients and uses From address');
    console.log('- SMTP connection is established with proper auth');
    console.log('- Frontend sends correct field names (smtpHost, not server)');
    console.log('- Emails are sent to courtmanr@gmail.com');
  } else {
    console.log('❌ SOME TESTS FAILED!');
    console.log('\nPlease check the failed tests above.');
  }
  
  return allPassed;
}

// Run the test
testEmailFunctionality()
  .then(success => process.exit(success ? 0 : 1))
  .catch(error => {
    console.error('Test error:', error);
    process.exit(1);
  });