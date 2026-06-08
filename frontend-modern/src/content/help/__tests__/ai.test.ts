import { describe, expect, it } from 'vitest';
import { aiHelpContent } from '../ai';

describe('AI help content', () => {
  it('describes provider route recovery as explicit instead of automatic fallback', () => {
    const providerHelp = aiHelpContent.find((item) => item.id === 'ai.providers.overview');

    expect(providerHelp?.description).toContain('choose the route');
    expect(providerHelp?.description).toContain('switch routes explicitly');
    expect(providerHelp?.description).not.toMatch(/fallback to others|automatic provider fallback/i);
  });
});
