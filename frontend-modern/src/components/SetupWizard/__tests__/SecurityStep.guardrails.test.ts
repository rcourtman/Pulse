import { describe, expect, it } from 'vitest';
import { EN_MESSAGES } from '@/i18n/messages';
import securityStepSource from '../steps/SecurityStep.tsx?raw';
import welcomeStepSource from '../steps/WelcomeStep.tsx?raw';

describe('SecurityStep guardrails', () => {
  it('generates first-run credentials with browser cryptographic randomness', () => {
    expect(securityStepSource).toContain('crypto.getRandomValues');
    expect(securityStepSource).toContain('GENERATED_PASSWORD_LENGTH = 20');
    expect(securityStepSource).not.toContain('Math.random');
  });

  it('keeps security-step copy aligned with the source-choice setup model', () => {
    expect(securityStepSource).toContain("t('setup.security.description')");
    expect(securityStepSource).toContain("t('setup.security.generatedPasswordHelp')");

    const securityStepMessages = [
      EN_MESSAGES['setup.security.description'],
      EN_MESSAGES['setup.security.generatedPasswordHelp'],
    ].join('\n');

    expect(securityStepMessages).toContain('choose the first infrastructure source');
    expect(securityStepMessages).toContain('A secure 20-character password');
    expect(securityStepMessages).not.toContain('install your first monitored host');
    expect(securityStepMessages).not.toContain('16-character password');
  });

  it('does not cover first-run handoff screens with success toasts during normal step transitions', () => {
    expect(welcomeStepSource).not.toContain("showSuccess('Token verified!')");
    expect(securityStepSource).not.toContain("showSuccess('Security configured!')");
  });
});
