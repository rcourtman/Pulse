const axios = require('axios');
const { spawn } = require('child_process');

async function testEmailDetailed() {
  console.log('=== DETAILED EMAIL TEST ===\n');
  
  // Start monitoring backend logs
  console.log('Starting backend log monitor...\n');
  const logMonitor = spawn('sudo', ['tail', '-f', '/opt/pulse/pulse.log']);
  
  let logOutput = '';
  logMonitor.stdout.on('data', (data) => {
    const output = data.toString();
    logOutput += output;
    // Show all logs for debugging
    process.stdout.write(output);
  });
  
  logMonitor.stderr.on('data', (data) => {
    console.error('[ERROR]', data.toString());
  });
  
  // Wait for log monitor to start
  await new Promise(resolve => setTimeout(resolve, 1000));
  
  // Send test email
  console.log('\n=== SENDING TEST EMAIL ===\n');
  try {
    const response = await axios.post('http://localhost:3000/api/notifications/test', {
      method: 'email'
    });
    console.log('\nAPI Response:', JSON.stringify(response.data, null, 2));
  } catch (error) {
    console.error('\nAPI Error:', error.response?.data || error.message);
  }
  
  // Wait for any SMTP errors
  console.log('\nWaiting for SMTP logs...\n');
  await new Promise(resolve => setTimeout(resolve, 3000));
  
  // Kill log monitor
  logMonitor.kill();
  
  // Check if we got any SMTP errors
  if (logOutput.includes('Failed to send email')) {
    console.log('\n❌ Email send failed - check logs above for details');
  } else if (logOutput.includes('Email notification sent')) {
    console.log('\n✅ Email appears to have been sent successfully');
    console.log('   Check your email at: test@example.com');
  } else {
    console.log('\n🤔 Could not determine email status from logs');
  }
}

testEmailDetailed().catch(console.error);