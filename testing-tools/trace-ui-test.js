const axios = require('axios');

// Create a simple proxy server to intercept requests
const http = require('http');
const url = require('url');

const BACKEND_PORT = 3000;
const PROXY_PORT = 3001;

const server = http.createServer(async (req, res) => {
  console.log(`\n=== ${req.method} ${req.url} ===`);
  
  if (req.url === '/api/notifications/test' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', async () => {
      console.log('Request body:', body);
      
      try {
        const parsed = JSON.parse(body);
        console.log('\nParsed request:');
        console.log('  Method:', parsed.method);
        if (parsed.config) {
          console.log('  Config provided: YES');
          console.log('    From:', parsed.config.from);
          console.log('    To:', parsed.config.to);
          console.log('    Password:', parsed.config.password ? '[PROVIDED]' : '[MISSING]');
          console.log('    SMTP Host:', parsed.config.smtpHost);
        } else {
          console.log('  Config provided: NO (using saved config)');
        }
      } catch (e) {
        console.log('Failed to parse body');
      }
      
      // Forward to real backend
      try {
        const response = await axios.post(`http://localhost:${BACKEND_PORT}${req.url}`, body, {
          headers: {
            'Content-Type': 'application/json'
          }
        });
        
        res.writeHead(response.status, response.headers);
        res.end(JSON.stringify(response.data));
      } catch (error) {
        res.writeHead(error.response?.status || 500);
        res.end(error.response?.data || 'Error');
      }
    });
  } else {
    // Proxy other requests
    res.writeHead(404);
    res.end('Not found');
  }
});

server.listen(PROXY_PORT, () => {
  console.log(`Proxy server listening on port ${PROXY_PORT}`);
  console.log('\nTo test:');
  console.log('1. Update frontend dev tools to redirect /api/notifications/test to localhost:3001');
  console.log('2. Or use curl: curl -X POST http://localhost:3001/api/notifications/test -d \'{"method":"email"}\' -H "Content-Type: application/json"');
  console.log('\nPress Ctrl+C to stop\n');
});
