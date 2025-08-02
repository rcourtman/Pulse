const axios = require('axios');

async function testSaveWithPassword() {
  console.log('=== TESTING EMAIL SAVE WITH PASSWORD ===\n');
  
  try {
    // 1. Get current config
    console.log('1. Getting current email config...');
    const response = await axios.get('http://localhost:3000/api/notifications/email');
    const currentConfig = response.data;
    
    console.log('   Current password:', currentConfig.password ? 'SET' : 'EMPTY');
    
    // 2. Save with test password
    console.log('\n2. Saving config with test password...');
    const testConfig = {
      ...currentConfig,
      password: 'test-password-123',
      to: ['test@example.com']
    };
    
    await axios.put('http://localhost:3000/api/notifications/email', testConfig);
    console.log('   ✅ Saved successfully');
    
    // 3. Read back
    console.log('\n3. Reading back saved config...');
    const savedResponse = await axios.get('http://localhost:3000/api/notifications/email');
    const savedConfig = savedResponse.data;
    
    console.log('   Password after save:', savedConfig.password ? 'SET' : 'EMPTY');
    console.log('   Recipients:', savedConfig.to);
    
    // 4. Test email send
    console.log('\n4. Testing email send...');
    try {
      const testResponse = await axios.post('http://localhost:3000/api/notifications/test', {
        method: 'email'
      });
      console.log('   Response:', testResponse.data);
    } catch (testError) {
      console.error('   Test email failed:', testError.response?.data?.error || testError.message);
    }
    
    // 5. Restore original (without password)
    console.log('\n5. Restoring original config...');
    await axios.put('http://localhost:3000/api/notifications/email', currentConfig);
    console.log('   ✅ Original config restored');
    
  } catch (error) {
    console.error('Error:', error.response?.data || error.message);
  }
}

testSaveWithPassword().catch(console.error);
