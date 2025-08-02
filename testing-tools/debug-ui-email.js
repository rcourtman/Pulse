const axios = require('axios');
const { spawn } = require('child_process');

async function debugUIEmail() {
  console.log('=== DEBUG UI EMAIL ISSUE ===\n');
  
  // Start monitoring logs
  console.log('Starting log monitor...');
  const logProcess = spawn('sudo', ['tail', '-f', '/opt/pulse/pulse.log']);
  
  logProcess.stdout.on('data', (data) => {
    const output = data.toString();
    
    // Filter for relevant logs
    if (output.includes('Test notification request') ||
        output.includes('Testing email') ||
        output.includes('Using From address') ||
        output.includes('Attempting to send email') ||
        output.includes('Email notification sent') ||
        output.includes('Failed to send email') ||
        output.includes('password')) {
      process.stdout.write(output);
    }
  });
  
  console.log('\n👉 Please click "Send Test Email" in the UI now...\n');
  console.log('Monitoring for 30 seconds...\n');
  
  // Also check what's in the saved config
  setTimeout(async () => {
    console.log('\n=== CHECKING SAVED CONFIG ===');
    try {
      const response = await axios.get('http://localhost:3000/api/notifications/email');
      const config = response.data;
      console.log('Saved email config:');
      console.log('  Enabled:', config.enabled);
      console.log('  From:', config.from);
      console.log('  Recipients:', config.to);
      console.log('  SMTP Host:', config.smtpHost);
      console.log('  SMTP Port:', config.smtpPort);
      console.log('  Username:', config.username);
      console.log('  Password:', config.password ? '[SET]' : '[EMPTY]');
    } catch (error) {
      console.error('Error getting config:', error.message);
    }
  }, 5000);
  
  // Wait for user action
  await new Promise(resolve => setTimeout(resolve, 30000));
  
  logProcess.kill();
  console.log('\n\nDone monitoring.');
}

debugUIEmail().catch(console.error);