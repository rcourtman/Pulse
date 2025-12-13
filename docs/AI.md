# Pulse AI

Pulse AI is an intelligent monitoring assistant that helps you understand and manage your infrastructure through natural language conversations, proactive monitoring, and automated issue resolution.

## Features

### Chat Assistant
Ask questions about your infrastructure in plain English:
- "What's using the most CPU right now?"
- "Show me VMs with high memory usage"
- "Why is my backup failing?"
- "Summarize the health of my cluster"

### Patrol Mode
Automated monitoring that runs on a schedule to:
- Analyze resource utilization patterns
- Detect potential issues before they cause problems
- Generate actionable recommendations
- Track trends over time

### Auto-Fix
Automatically resolve common issues:
- Restart stuck services
- Clear disk space
- Restart unresponsive containers
- Apply recommended optimizations

### Predictive Intelligence
- Identify resources trending toward problems
- Forecast disk space exhaustion
- Detect unusual patterns
- Alert on anomalies

## Configuration

### Enable Pulse AI

1. Navigate to **Settings → AI**
2. Toggle **Enable Pulse AI**
3. Configure your AI provider

### Supported Providers

| Provider | Models | Notes |
|----------|--------|-------|
| **OpenAI** | GPT-4, GPT-4 Turbo, GPT-3.5 | Recommended for best results |
| **Anthropic** | Claude 3 Opus, Sonnet, Haiku | Excellent for complex analysis |
| **Ollama** | Llama 3, Mistral, etc. | Self-hosted, privacy-focused |
| **OpenRouter** | Multiple models | Pay-per-use routing |
| **Custom** | Any OpenAI-compatible API | For enterprise deployments |

### API Key Setup

```bash
# Environment variable (recommended for production)
export PULSE_AI_PROVIDER=openai
export PULSE_AI_API_KEY=sk-...

# Or configure via Settings UI
```

### Patrol Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| Patrol Enabled | Off | Run automated checks |
| Patrol Interval | 30 minutes | How often to run patrol |
| Auto-Fix Enabled | Off | Allow automatic remediation |
| Auto-Fix Model | Same as chat | Model for auto-fix decisions |

## Usage

### Chat Interface
Access the AI chat from the bottom-right corner of any page. Type your question and press Enter.

**Example queries:**
```
"What VMs are using more than 80% memory?"
"Show me the status of all backups"
"Why is node pve1 showing high load?"
"Compare resource usage between this week and last week"
```

### Context Selection
Click on any resource (VM, container, host) to add it to the AI context. This helps the AI provide more specific answers.

### Patrol Reports
When Patrol is enabled, Pulse AI will:
1. Run periodic health checks
2. Generate findings (issues, warnings, info)
3. Offer to auto-fix issues (if enabled)
4. Track trends over time

## Cost Tracking

Track AI API usage in **Settings → AI → Cost Dashboard**:
- Daily/monthly usage statistics
- Cost breakdown by feature
- Usage trends over time

## Privacy & Security

- **Data stays local**: Only resource metadata is sent to AI providers
- **No training**: Your data is never used for model training
- **Audit logging**: All AI interactions are logged
- **Self-hosted option**: Use Ollama for complete data privacy

## Troubleshooting

### AI not responding
1. Check API key is configured correctly
2. Verify network connectivity to AI provider
3. Check Settings → AI for error messages

### Patrol not running
1. Ensure Patrol is enabled in Settings
2. Check system resource availability
3. Review logs: `journalctl -u pulse -f`

### Auto-fix not working
1. Enable Auto-Fix in Settings → AI
2. Verify the connected agents have execute permissions
3. Check the Auto-Fix model is configured
