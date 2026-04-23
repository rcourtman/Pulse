import { describe, expect, it } from 'vitest';
import securityStepSource from '../steps/SecurityStep.tsx?raw';
import welcomeStepSource from '../steps/WelcomeStep.tsx?raw';

describe('SecurityStep guardrails', () => {
  it('generates first-run credentials with browser cryptographic randomness', () => {
    expect(securityStepSource).toContain('crypto.getRandomValues');
    expect(securityStepSource).toContain('GENERATED_PASSWORD_LENGTH = 20');
    expect(securityStepSource).not.toContain('Math.random');
  });

  it('keeps security-step copy aligned with the source-choice setup model', () => {
    expect(securityStepSource).toContain('choose the first infrastructure source');
    expect(securityStepSource).toContain('A secure 20-character password');
    expect(securityStepSource).not.toContain('install your first monitored host');
    expect(securityStepSource).not.toContain('16-character password');
  });

  it('does not cover first-run handoff screens with success toasts during normal step transitions', () => {
    expect(welcomeStepSource).not.toContain("showSuccess('Token verified!')");
    expect(securityStepSource).not.toContain("showSuccess('Security configured!')");
  });
});
