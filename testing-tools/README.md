# Pulse Testing Tools

This directory contains automated testing tools for the Pulse monitoring system.

## Setup

```bash
cd /opt/pulse/testing-tools
npm install
```

## Available Tests

### 1. Email Configuration Test
Tests email notification setup and persistence.
```bash
npm run test:email
```

### 2. API Endpoints Test
Tests all API endpoints for availability and correct responses.
```bash
npm run test:api
```

### 3. Button Functionality Test
Tests UI buttons and their actions using Playwright.
```bash
npm run test:buttons
```

### 4. Comprehensive Settings Test
Tests all major settings including thresholds, notifications, and encryption.
```bash
npm run test:comprehensive
```

### 5. Email Final Test
Comprehensive email functionality verification.
```bash
node test-email-final.js
```

### 6. UI Integration Test
Tests frontend-backend integration for email.
```bash
node test-ui-integration.js
```

### 7. Run All Tests
```bash
npm run test:all
```

## Test Results

Tests will output:
- ✅ PASS for successful tests
- ❌ FAIL for failed tests
- Summary statistics at the end

## Screenshots

UI tests may create screenshot files:
- `email-config-page.png` - Email configuration page
- `error-*.png` - Error screenshots if tests fail

## Notes

- Tests run against `http://localhost:3000` (backend) and `http://192.168.0.123:7655` (frontend)
- Playwright tests run in headless mode by default
- All tests are non-destructive and clean up after themselves