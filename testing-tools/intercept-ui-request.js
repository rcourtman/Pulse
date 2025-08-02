const express = require('express');
const app = express();

app.use(express.json());

// Intercept test requests
app.post('/api/notifications/test', (req, res) => {
  console.log('\n=== TEST EMAIL REQUEST ===');
  console.log('Headers:', req.headers);
  console.log('\nBody:', JSON.stringify(req.body, null, 2));
  
  if (req.body.config) {
    console.log('\nConfig details:');
    console.log('  From:', req.body.config.from);
    console.log('  To:', req.body.config.to);
    console.log('  Recipients count:', req.body.config.to ? req.body.config.to.length : 0);
    console.log('  Has password:', !!req.body.config.password);
    console.log('  SMTP Host:', req.body.config.server);
    console.log('  SMTP Port:', req.body.config.port);
  }
  
  // Return error to see what frontend shows
  res.status(400).send('Test intercept - check console output');
});

// Proxy other requests to backend
app.use((req, res) => {
  console.log('Proxying:', req.method, req.url);
  res.status(404).send('Not implemented in test server');
});

const PORT = 3001;
app.listen(PORT, () => {
  console.log(`Test server listening on port ${PORT}`);
  console.log('Update frontend to use http://localhost:3001 temporarily');
  console.log('Or use browser dev tools to intercept the request');
});