const axios = require('axios');

async function testEmailSend() {
  console.log('=== TESTING EMAIL SEND FUNCTIONALITY ===\n');
  
  try {
    // 1. Get current email config
    console.log('1. Getting current email configuration...');
    const configResponse = await axios.get('http://localhost:3000/api/notifications/email');
    const emailConfig = configResponse.data;
    
    console.log('   Email enabled:', emailConfig.enabled);
    console.log('   SMTP Host:', emailConfig.smtpHost);
    console.log('   SMTP Port:', emailConfig.smtpPort);
    console.log('   From:', emailConfig.from);
    console.log('   Recipients:', emailConfig.to);
    
    // 2. Test sending email
    console.log('\n2. Testing email send...');
    
    try {
      const testResponse = await axios.post('http://localhost:3000/api/notifications/test', {
        method: 'email'
      });
      
      console.log('   Response:', testResponse.status);
      console.log('   Data:', testResponse.data);
      
      if (testResponse.data.success) {
        console.log('   ✅ Test email sent successfully!');
      } else {
        console.log('   ❌ Test email failed:', testResponse.data.message);
      }
    } catch (error) {
      console.log('   ❌ Error sending test email:');
      console.log('   Status:', error.response?.status);
      console.log('   Error:', error.response?.data || error.message);
      
      // If 400, might be missing config
      if (error.response?.status === 400) {
        console.log('\n3. Checking what the backend expects...');
        console.log('   The backend might be expecting different fields or format');
      }
    }
    
    // 3. Try with full config in request
    console.log('\n4. Testing with config included in request...');
    try {
      const testWithConfig = await axios.post('http://localhost:3000/api/notifications/test', {
        method: 'email',
        config: emailConfig
      });
      
      console.log('   Response:', testWithConfig.status);
      console.log('   Data:', testWithConfig.data);
    } catch (error) {
      console.log('   Error with config:', error.response?.data || error.message);
    }
    
  } catch (error) {
    console.error('Failed to get email config:', error.message);
  }
}

if (require.main === module) {
  testEmailSend().catch(console.error);
}

module.exports = { testEmailSend };