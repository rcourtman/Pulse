const axios = require('axios');

async function testWithConfig() {
  console.log('=== TESTING EMAIL WITH EXPLICIT CONFIG ===\n');
  
  // Test 1: With empty recipients array
  console.log('Test 1: Empty recipients array');
  try {
    const response1 = await axios.post('http://localhost:3000/api/notifications/test', {
      method: 'email',
      config: {
        enabled: true,
        smtpHost: 'smtp.gmail.com',
        smtpPort: 587,
        username: 'test@example.com',
        password: 'zlff ruyk bxxf cxch',
        from: 'test@example.com',
        to: [],  // Empty array
        tls: true
      }
    });
    console.log('✅ Success:', response1.data);
  } catch (error) {
    console.error('❌ Failed:', error.response?.data || error.message);
  }
  
  // Test 2: With no password
  console.log('\nTest 2: No password (should fail)');
  try {
    const response2 = await axios.post('http://localhost:3000/api/notifications/test', {
      method: 'email',
      config: {
        enabled: true,
        smtpHost: 'smtp.gmail.com',
        smtpPort: 587,
        username: 'test@example.com',
        password: '',  // Empty password
        from: 'test@example.com',
        to: [],
        tls: true
      }
    });
    console.log('✅ Success:', response2.data);
  } catch (error) {
    console.error('❌ Failed:', error.response?.data || error.message);
  }
  
  // Test 3: Check what frontend sends (server vs smtpHost)
  console.log('\nTest 3: Using "server" field like frontend');
  try {
    const response3 = await axios.post('http://localhost:3000/api/notifications/test', {
      method: 'email',
      config: {
        enabled: true,
        server: 'smtp.gmail.com',  // Frontend uses 'server'
        port: 587,
        username: 'test@example.com',
        password: 'zlff ruyk bxxf cxch',
        from: 'test@example.com',
        to: [],
        tls: true
      }
    });
    console.log('✅ Success:', response3.data);
  } catch (error) {
    console.error('❌ Failed:', error.response?.data || error.message);
  }
}

testWithConfig().catch(console.error);