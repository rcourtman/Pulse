const axios = require('axios');

async function testNoConfig() {
  console.log('=== TEST EMAIL WITHOUT CONFIG ===\n');
  
  // First ensure we have saved config with password
  console.log('1. Saving email config with password...');
  await axios.put('http://localhost:3000/api/notifications/email', {
    enabled: true,
    smtpHost: 'smtp.gmail.com',
    smtpPort: 587,
    username: 'test@example.com',
    password: 'zlff ruyk bxxf cxch',
    from: 'test@example.com',
    to: [],
    tls: true
  });
  
  // Test without sending config (like UI now does)
  console.log('\n2. Testing email WITHOUT sending config...');
  try {
    const response = await axios.post('http://localhost:3000/api/notifications/test', {
      method: 'email'
      // No config - uses saved config
    });
    console.log('✅ Success:', response.data);
  } catch (error) {
    console.error('❌ Failed:', error.response?.data || error.message);
  }
  
  // Monitor logs
  const { spawn } = require('child_process');
  const logProcess = spawn('sudo', ['tail', '-10', '/opt/pulse/pulse.log']);
  
  logProcess.stdout.on('data', (data) => {
    const output = data.toString();
    if (output.includes('Email notification sent successfully')) {
      console.log('\n✅ Email was sent successfully\!');
    }
  });
  
  setTimeout(() => {
    logProcess.kill();
  }, 2000);
}

testNoConfig().catch(console.error);
