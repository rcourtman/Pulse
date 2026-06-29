import type { HelpContent } from './types';

/**
 * Help content for Pulse Assistant and Patrol features
 */
export const aiHelpContent: HelpContent[] = [
  {
    id: 'ai.openai.baseUrl',
    title: 'Custom OpenAI-Compatible Endpoint',
    description:
      'Use alternative providers with OpenAI-compatible APIs instead of the official OpenAI API.\n\n' +
      'Supported services:\n' +
      '- OpenRouter: Access Claude, Llama, Mistral, and 100+ models through one API key\n' +
      '- vLLM / llama.cpp: Self-hosted local inference servers\n' +
      '- Azure OpenAI: Enterprise Azure deployments\n' +
      '- Together, Anyscale, Fireworks: Alternative cloud providers\n\n' +
      'Enter the provider\'s base URL and use their API key in the API Key field.',
    examples: [
      'https://openrouter.ai/api/v1 (OpenRouter)',
      'http://localhost:8000/v1 (vLLM local)',
      'https://your-resource.openai.azure.com (Azure)',
      'https://api.together.xyz/v1 (Together)',
    ],
    addedInVersion: 'v4.5.0',
  },
  {
    id: 'ai.ollama.baseUrl',
    title: 'Ollama Server URL',
    description:
      'Connect to a local or remote Ollama instance for Pulse Assistant and Patrol features.\n\n' +
      'Ollama provides easy access to open-source models like Llama, Mistral, and CodeLlama ' +
      'without requiring cloud API keys.\n\n' +
      'If your Ollama endpoint is behind a reverse proxy, you can also store an optional Basic Auth username and password.\n\n' +
      'Default: http://localhost:11434 (local Ollama installation)',
    examples: [
      'http://localhost:11434 (local)',
      'http://192.168.1.100:11434 (LAN server)',
      'http://ollama.internal:11434 (Docker network)',
    ],
    addedInVersion: 'v4.5.0',
  },
  {
    id: 'ai.ollama.keepAlive',
    title: 'Ollama Keep Alive',
    description:
      'Control how long Ollama keeps a model loaded after a response.\n\n' +
      'Use a duration such as 10m or 24h to reduce cold starts on local hardware. ' +
      'Use seconds such as 3600 if you prefer numeric values. Use 0 to unload the model after each response, or -1 to keep it loaded indefinitely.\n\n' +
      'Leave empty to use Ollama\'s default behavior.',
    examples: [
      '10m (keep loaded for 10 minutes)',
      '24h (keep loaded for one day)',
      '0 (unload after each response)',
      '-1 (keep loaded)',
    ],
    addedInVersion: 'v4.5.0',
  },
  {
    id: 'ai.providers.overview',
    title: 'Provider Configuration',
    description:
      'Configure one or more providers to enable Pulse Assistant and Patrol features:\n\n' +
      '- Anomaly detection and pattern recognition\n' +
      '- Natural language infrastructure queries\n' +
      '- Automated troubleshooting suggestions\n' +
      '- Alert investigation assistance\n\n' +
      'You can configure multiple providers and Pulse will use the primary provider ' +
      'with fallback to others if unavailable.',
    related: ['ai.openai.baseUrl', 'ai.ollama.baseUrl', 'ai.ollama.keepAlive'],
    addedInVersion: 'v4.0.0',
  },
];
