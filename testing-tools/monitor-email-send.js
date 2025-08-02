const axios = require('axios');
const { spawn } = require('child_process');

async function monitorEmailSend() {
  console.log('=== MONITORING EMAIL SEND ===\n');
  
  // Start log monitoring
  console.log('Starting log monitor...\n');
  const logMonitor = spawn('sudo', ['tail', '-f', '/opt/pulse/pulse.log']);
  
  logMonitor.stdout.on('data', (data) => {
    const lines = data.toString().split('\n');
    lines.forEach(line => {
      if (line.includes('email') || line.includes('notification') || line.includes('smtp') || line.includes('test')) {
        console.log('[LOG]', line);
      }
    });
  });
  
  // Wait a moment for log monitor to start
  await new Promise(resolve => setTimeout(resolve, 1000));
  
  // Send test email
  console.log('\n=== SENDING TEST EMAIL ===\n');
  try {
    const response = await axios.post('http://localhost:3000/api/notifications/test', {
      method: 'email'
    });
    console.log('API Response:', response.data);
  } catch (error) {
    console.error('API Error:', error.response?.data || error.message);
  }
  
  // Keep monitoring for a few seconds
  console.log('\nMonitoring logs for 5 seconds...\n');
  await new Promise(resolve => setTimeout(resolve, 5000));
  
  // Kill log monitor
  logMonitor.kill();
  console.log('\nDone monitoring.');
}

monitorEmailSend().catch(console.error);