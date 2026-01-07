import type { HelpContent } from './types';

/**
 * Help content for AI-related features
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
      '- Together AI, Anyscale, Fireworks: Alternative cloud providers\n\n' +
      'Enter the provider\'s base URL and use their API key in the API Key field.',
    examples: [
      'https://openrouter.ai/api/v1 (OpenRouter)',
      'http://localhost:8000/v1 (vLLM local)',
      'https://your-resource.openai.azure.com (Azure)',
      'https://api.together.xyz/v1 (Together AI)',
    ],
    addedInVersion: 'v4.5.0',
  },
  {
    id: 'ai.ollama.baseUrl',
    title: 'Ollama Server URL',
    description:
      'Connect to a local or remote Ollama instance for AI features.\n\n' +
      'Ollama provides easy access to open-source models like Llama, Mistral, and CodeLlama ' +
      'without requiring cloud API keys.\n\n' +
      'Default: http://localhost:11434 (local Ollama installation)',
    examples: [
      'http://localhost:11434 (local)',
      'http://192.168.1.100:11434 (LAN server)',
      'http://ollama.internal:11434 (Docker network)',
    ],
    addedInVersion: 'v4.5.0',
  },
  {
    id: 'ai.providers.overview',
    title: 'AI Provider Configuration',
    description:
      'Configure one or more AI providers to enable intelligent features:\n\n' +
      '- Anomaly detection and pattern recognition\n' +
      '- Natural language infrastructure queries\n' +
      '- Automated troubleshooting suggestions\n' +
      '- Alert investigation assistance\n\n' +
      'You can configure multiple providers and Pulse will use the primary provider ' +
      'with fallback to others if unavailable.',
    related: ['ai.openai.baseUrl', 'ai.ollama.baseUrl'],
    addedInVersion: 'v4.0.0',
  },
];
