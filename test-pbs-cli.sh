#!/bin/bash

echo "=== PBS Form Contamination Test ==="
echo ""
echo "This test verifies the PBS form fix by:"
echo "1. Opening Pulse UI in Firefox"
echo "2. You manually test according to the steps"
echo ""

# Open the manual test instructions
echo "Opening test instructions and Pulse UI..."

# Create HTML test page with instructions
cat > /tmp/pbs-test.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>PBS Form Test</title>
    <style>
        body { font-family: Arial; padding: 20px; }
        .test { margin: 20px 0; padding: 10px; border: 1px solid #ccc; }
        .pass { background: #d4ffd4; }
        .fail { background: #ffd4d4; }
        h2 { color: #333; }
        ol li { margin: 10px 0; }
        .check { font-weight: bold; color: #0066cc; }
    </style>
</head>
<body>
    <h1>PBS Form Contamination Test</h1>
    
    <div class="test">
        <h2>Test Steps:</h2>
        <ol>
            <li>Open <a href="http://localhost:7655" target="_blank">Pulse UI (click here)</a></li>
            <li>Navigate to Settings → Nodes tab</li>
            <li>
                Click "Add PVE Node"<br>
                - Enter name: <code>test-pve</code><br>
                - Enter host: <code>https://192.168.1.100:8006</code><br>
                - <strong>Click Cancel</strong> (don't save)
            </li>
            <li>
                Click "Add PBS Node"<br>
                <span class="check">✓ CHECK: Form should be COMPLETELY EMPTY</span><br>
                <span class="fail">✗ FAIL if: Form has any data from PVE</span>
            </li>
            <li>
                Fill PBS form and save:<br>
                - Name: <code>test-pbs</code><br>
                - Host: <code>https://192.168.1.200:8007</code><br>
                - Token: <code>root@pam!pbstoken</code><br>
                - Token value: <code>test-token-12345</code><br>
                - Click "Add Node"
            </li>
            <li>
                Click Edit icon on the PBS node<br>
                <span class="check">✓ CHECK: Form should show PBS data (test-pbs, etc)</span><br>
                <span class="fail">✗ FAIL if: Form is empty or shows wrong data</span>
            </li>
        </ol>
    </div>
    
    <div class="test">
        <h2>Expected Results:</h2>
        <ul>
            <li>PBS form NEVER shows PVE data ✓</li>
            <li>PVE form NEVER shows PBS data ✓</li>
            <li>Editing PBS node shows correct PBS data ✓</li>
            <li>Editing PVE node shows correct PVE data ✓</li>
        </ul>
    </div>
</body>
</html>
EOF

# Try to open in browser if available
if command -v firefox &> /dev/null; then
    firefox /tmp/pbs-test.html 2>/dev/null &
elif command -v chromium &> /dev/null; then
    chromium /tmp/pbs-test.html 2>/dev/null &
else
    echo "Please open /tmp/pbs-test.html in a browser"
fi

echo ""
echo "Manual test page created at: /tmp/pbs-test.html"
echo "Please follow the test steps and report results."
echo ""