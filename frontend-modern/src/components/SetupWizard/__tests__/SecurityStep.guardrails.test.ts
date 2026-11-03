import { describe, expect, it } from 'vitest';
import { EN_MESSAGES } from '@/i18n/messages';
import securityStepSource from '../steps/SecurityStep.tsx?raw';

describe('SecurityStep guardrails', () => {
  it('generates first-run credentials with browser cryptographic randomness', () => {
    expect(securityStepSource).toContain('crypto.getRandomValues');
    expect(securityStepSource).toContain('GENERATED_PASSWORD_LENGTH = 20');
    expect(securityStepSource).not.toContain('Math.random');
  });

  it('keeps security-step copy aligned with the source-choice setup model', () => {
    const securityStepMessages = [
      EN_MESSAGES['setup.security.description'],
      EN_MESSAGES['setup.security.generatedPasswordHelp'],
    ].join('\n');

    expect(securityStepMessages).toContain('choose the first infrastructure source');
    expect(securityStepMessages).toContain('A secure 20-character password');
    expect(securityStepMessages).not.toContain('install your first monitored host');
    expect(securityStepMessages).not.toContain('16-character password');
  });
});
