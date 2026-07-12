import { describe, expect, it } from 'vitest';
import { getAIChatProviderReadinessPresentation } from '@/utils/aiChatPresentation';

describe('getAIChatProviderReadinessPresentation (branch coverage)', () => {
  describe('status: checking', () => {
    it('falls back to the generic subject and generic body when routeLabel is absent', () => {
      expect(getAIChatProviderReadinessPresentation({ status: 'checking' })).toEqual({
        tone: 'checking',
        title: 'Verifying Selected model route',
        body: 'Pulse is checking the selected model route.',
      });
    });

    it('uses the route-label title and a suffix-free body when providerLabel matches "selected" case-insensitively', () => {
      expect(
        getAIChatProviderReadinessPresentation({
          status: 'checking',
          providerLabel: 'SELECTED',
          routeLabel: 'Acme: GPT-X',
        }),
      ).toEqual({
        tone: 'checking',
        title: 'Verifying selected model route',
        body: 'Pulse is checking Acme: GPT-X.',
      });
    });

    it('appends a provider suffix to the body when providerLabel is a distinct provider', () => {
      expect(
        getAIChatProviderReadinessPresentation({
          status: 'checking',
          providerLabel: 'Anthropic',
          routeLabel: 'Anthropic: Claude',
        }),
      ).toEqual({
        tone: 'checking',
        title: 'Verifying selected model route',
        body: 'Pulse is checking Anthropic: Claude through Anthropic.',
      });
    });

    it('treats a whitespace-only routeLabel as absent', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'checking',
        providerLabel: 'Groq',
        routeLabel: '   ',
      });
      expect(result.title).toBe('Verifying Groq provider route');
      expect(result.body).toBe('Pulse is checking the selected model route.');
    });
  });

  describe('status: ready', () => {
    it('prefers a trimmed summary for the body over the message', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'ready',
        summary: '  All systems go  ',
        message: 'should be ignored',
      });
      expect(result.body).toBe('All systems go');
    });

    it('falls back to the message when the summary is whitespace-only', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'ready',
        summary: '   ',
        message: '  Reachable now  ',
      });
      expect(result.body).toBe('Reachable now');
    });

    it('uses reachability copy with a provider suffix when no summary/message and routeLabel is present', () => {
      expect(
        getAIChatProviderReadinessPresentation({
          status: 'ready',
          providerLabel: 'Mistral',
          routeLabel: 'Mistral: Large',
        }),
      ).toEqual({
        tone: 'ready',
        title: 'Selected model route ready',
        body: 'Pulse can reach Mistral: Large through Mistral.',
      });
    });

    it('uses generic reachability copy and the default fallback subject title when routeLabel is absent', () => {
      expect(getAIChatProviderReadinessPresentation({ status: 'ready' })).toEqual({
        tone: 'ready',
        title: 'Selected model route ready',
        body: 'Pulse can reach the selected model route.',
      });
    });

    it('uses a provider-named fallback subject title when routeLabel is whitespace and providerLabel is distinct', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'ready',
        providerLabel: 'Groq',
        routeLabel: '   ',
      });
      expect(result.title).toBe('Groq provider route ready');
      expect(result.body).toBe('Pulse can reach the selected model route.');
    });
  });

  describe('status: error', () => {
    it('prefers a trimmed summary for the body over the message', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'error',
        summary: 'Connection refused',
        message: 'should be ignored',
      });
      expect(result.body).toBe('Connection refused');
    });

    it('falls back to the message when no summary is provided', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'error',
        message: 'Timed out',
      });
      expect(result.body).toBe('Timed out');
    });

    it('uses the canonical fallback body when neither summary nor message is provided', () => {
      expect(getAIChatProviderReadinessPresentation({ status: 'error' }).body).toBe(
        'Pulse could not verify the selected model route.',
      );
    });

    it('exposes a trimmed recommendation and a provider-named fallback subject title', () => {
      expect(
        getAIChatProviderReadinessPresentation({
          status: 'error',
          providerLabel: 'Cohere',
          recommendation: '  Verify the API key.  ',
        }),
      ).toEqual({
        tone: 'error',
        title: 'Cohere provider route issue',
        body: 'Pulse could not verify the selected model route.',
        recommendation: 'Verify the API key.',
      });
    });

    it('coerces a whitespace-only recommendation to undefined', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'error',
        recommendation: '   ',
      });
      expect(result.recommendation).toBeUndefined();
    });

    it('uses the default fallback subject title when routeLabel is absent and providerLabel defaults', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'error',
        providerLabel: '   ',
      });
      expect(result.tone).toBe('error');
      expect(result.title).toBe('Selected model route issue');
    });
  });

  describe('providerLabel defaulting and routeLabel trimming', () => {
    it('defaults an undefined providerLabel to "Selected"', () => {
      expect(getAIChatProviderReadinessPresentation({ status: 'ready' }).title).toBe(
        'Selected model route ready',
      );
    });

    it('defaults a whitespace-only providerLabel to "Selected"', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'checking',
        providerLabel: '   ',
      });
      expect(result.title).toBe('Verifying Selected model route');
      expect(result.body).toBe('Pulse is checking the selected model route.');
    });

    it('trims a non-trivial providerLabel before building the suffix', () => {
      const result = getAIChatProviderReadinessPresentation({
        status: 'checking',
        providerLabel: '  DeepSeek  ',
        routeLabel: 'DeepSeek: V4',
      });
      expect(result.body).toBe('Pulse is checking DeepSeek: V4 through DeepSeek.');
    });
  });
});
