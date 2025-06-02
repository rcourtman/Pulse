const express = require('express');
const app = express();
const port = 3001;

app.use(express.json());

app.post('/webhook', (req, res) => {
    console.log('=== WEBHOOK RECEIVED ===');
    console.log('Headers:', req.headers);
    console.log('Body:', JSON.stringify(req.body, null, 2));
    console.log('========================');
    
    res.status(200).json({ received: true });
});

app.listen(port, () => {
    console.log(`Test webhook server running at http://localhost:${port}/webhook`);
    console.log('Configure Pulse to send webhooks to: http://localhost:3001/webhook');
});