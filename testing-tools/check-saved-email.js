const axios = require('axios');
const fs = require('fs');
const { exec } = require('child_process');
const util = require('util');
const execPromise = util.promisify(exec);

async function checkSavedEmail() {
  console.log('=== CHECKING SAVED EMAIL CONFIGURATION ===\n');
  
  try {
    // 1. Get via API (decrypted)
    console.log('1. Getting email config via API...');
    const response = await axios.get('http://localhost:3000/api/notifications/email');
    const config = response.data;
    
    console.log('\nEmail Configuration:');
    console.log('  Enabled:', config.enabled);
    console.log('  Provider:', config.provider);
    console.log('  SMTP Host:', config.smtpHost);
    console.log('  SMTP Port:', config.smtpPort);
    console.log('  From:', config.from);
    console.log('  Username:', config.username);
    console.log('  Password:', config.password ? '[REDACTED]' : '(empty)');
    console.log('  Recipients:', config.to);
    console.log('  Recipients count:', config.to ? config.to.length : 0);
    
    if (config.to && config.to.length > 0) {
      console.log('\n  Recipients list:');
      config.to.forEach((recipient, i) => {
        console.log(`    ${i + 1}. ${recipient}`);
      });
    } else {
      console.log('\n  ❌ No recipients configured!');
    }
    
    // 2. Test what happens when we save with recipients
    console.log('\n2. Testing save with recipients...');
    
    const testConfig = {
      ...config,
      to: ['test1@example.com', 'test2@example.com']
    };
    
    console.log('   Saving config with 2 test recipients...');
    await axios.put('http://localhost:3000/api/notifications/email', testConfig);
    
    // 3. Read back
    console.log('   Reading back saved config...');
    const saved = await axios.get('http://localhost:3000/api/notifications/email');
    
    console.log('   Recipients after save:', saved.data.to);
    console.log('   Recipients count:', saved.data.to ? saved.data.to.length : 0);
    
    // 4. Restore original
    console.log('\n3. Restoring original config...');
    await axios.put('http://localhost:3000/api/notifications/email', config);
    console.log('   ✅ Original config restored');
    
  } catch (error) {
    console.error('Error:', error.response?.data || error.message);
  }
}

if (require.main === module) {
  checkSavedEmail().catch(console.error);
}

module.exports = { checkSavedEmail };