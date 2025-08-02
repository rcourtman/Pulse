const axios = require('axios');

async function testEmailDirect() {
  console.log('=== TESTING EMAIL SEND DIRECTLY ===\n');
  
  try {
    // Get current config
    const response = await axios.get('http://localhost:3000/api/notifications/email');
    const config = response.data;
    
    console.log('Current email config:');
    console.log('  From:', config.from);
    console.log('  Recipients:', config.to);
    console.log('  Password:', config.password ? 'SET' : 'EMPTY');
    
    // Try to send test email
    console.log('\nSending test email...');
    const testResponse = await axios.post('http://localhost:3000/api/notifications/test', {
      method: 'email'
    });
    
    console.log('Response:', testResponse.data);
    
  } catch (error) {
    console.error('\nError details:');
    console.error('  Status:', error.response?.status);
    console.error('  Message:', error.response?.data?.error || error.message);
    
    if (error.response?.data) {
      console.error('  Full response:', JSON.stringify(error.response.data, null, 2));
    }
  }
}

testEmailDirect().catch(console.error);