const axios = require('axios');

async function saveUserPassword() {
  console.log('=== SAVING USER\'S EMAIL CONFIG WITH PASSWORD ===\n');
  
  try {
    // Get current config
    const response = await axios.get('http://localhost:3000/api/notifications/email');
    const config = response.data;
    
    // Update with user's actual credentials
    const userConfig = {
      enabled: true,
      smtpHost: 'smtp.gmail.com',
      smtpPort: 587,
      username: 'test@example.com',
      password: 'zlff ruyk bxxf cxch',  // User's app password
      from: 'test@example.com',
      to: [],  // Empty as user wants
      tls: true
    };
    
    console.log('Saving user config with password...');
    await axios.put('http://localhost:3000/api/notifications/email', userConfig);
    console.log('✅ Config saved');
    
    // Test sending
    console.log('\nTesting email send...');
    try {
      const testResponse = await axios.post('http://localhost:3000/api/notifications/test', {
        method: 'email'
      });
      console.log('✅ Test email sent:', testResponse.data);
    } catch (testError) {
      console.error('❌ Test email failed:', testError.response?.data?.error || testError.message);
    }
    
  } catch (error) {
    console.error('Error:', error.response?.data || error.message);
  }
}

saveUserPassword().catch(console.error);
