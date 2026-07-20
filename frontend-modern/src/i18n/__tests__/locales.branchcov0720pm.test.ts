import { describe, expect, it } from 'vitest';

import {
  DEFAULT_LOCALE,
  SUPPORTED_LOCALES,
  getLocaleFallbackChain,
  resolveSupportedLocale,
} from '@/i18n/locales';

// `LOCALE_ALIASES`, `SUPPORTED_LOCALE_SET`, `isSupportedLocale`, and
// `normalizeLocale` are module-private (not exported); their branches are
// exercised transitively through `resolveSupportedLocale` and
// `getLocaleFallbackChain`, asserting on the observable return values.

describe('resolveSupportedLocale — branch coverage', () => {
  describe('(a) direct supported-locale hit — isSupportedLocale(normalized) true arm', () => {
    it('returns every supported locale verbatim (locks the supported set)', () => {
      expect(SUPPORTED_LOCALES).toEqual(['en', 'de', 'es']);
      for (const locale of SUPPORTED_LOCALES) {
        expect(resolveSupportedLocale(locale)).toBe(locale);
      }
    });

    it('lowercases upper/mixed-case input before the direct lookup (toLowerCase step)', () => {
      // 'EN' -> trim -> 'EN' -> replace _ -> 'EN' -> toLowerCase -> 'en' -> direct hit.
      expect(resolveSupportedLocale('EN')).toBe('en');
      expect(resolveSupportedLocale('De')).toBe('de');
      expect(resolveSupportedLocale('eS')).toBe('es');
    });
  });

  describe('(b) normalization arm (trim + underscore->hyphen)', () => {
    it('trims surrounding whitespace before the direct-hit lookup', () => {
      // '  en  ' -> trim -> 'en' -> direct hit.
      expect(resolveSupportedLocale('  en  ')).toBe('en');
      // trim() also strips tabs and newlines.
      expect(resolveSupportedLocale('\tde\n')).toBe('de');
    });

    it('rewrites underscores to hyphens before the alias lookup', () => {
      // 'es_MX' -> 'es-MX' -> toLowerCase 'es-mx' -> LOCALE_ALIASES hit -> 'es'.
      expect(resolveSupportedLocale('es_MX')).toBe('es');
      // 'DE_AT' exercises both toLowerCase and underscore->hyphen before the alias hit.
      expect(resolveSupportedLocale('DE_AT')).toBe('de');
    });

    it('rewrites underscores to hyphens before the base-locale fallback', () => {
      // 'de_XX' (no alias entry) -> 'de-xx' -> not alias -> base 'de' supported -> 'de'.
      expect(resolveSupportedLocale('de_XX')).toBe('de');
    });
  });

  describe('(c) LOCALE_ALIASES lookup hit arm', () => {
    it('maps English regional variants to en', () => {
      expect(resolveSupportedLocale('en-GB')).toBe('en');
      expect(resolveSupportedLocale('en-US')).toBe('en');
    });

    it('maps German regional variants to de', () => {
      expect(resolveSupportedLocale('de-AT')).toBe('de');
      expect(resolveSupportedLocale('de-CH')).toBe('de');
      expect(resolveSupportedLocale('de-DE')).toBe('de');
    });

    it('maps Spanish regional variants (including es-419) to es', () => {
      expect(resolveSupportedLocale('es-419')).toBe('es');
      expect(resolveSupportedLocale('es-AR')).toBe('es');
      expect(resolveSupportedLocale('es-MX')).toBe('es');
      expect(resolveSupportedLocale('es-ES')).toBe('es');
      expect(resolveSupportedLocale('es-US')).toBe('es');
    });
  });

  describe('(d) base-locale fallback arm (region unlisted, base supported)', () => {
    it('falls back to the base locale when the region is not aliased but the base is supported', () => {
      // 'de-XX' -> not direct, not in LOCALE_ALIASES, base 'de' supported -> 'de'.
      expect(resolveSupportedLocale('de-XX')).toBe('de');
      // 'en-CA' -> not in alias map -> base 'en' supported -> 'en'.
      expect(resolveSupportedLocale('en-CA')).toBe('en');
      // 'es-EC' -> not in alias map -> base 'es' supported -> 'es'.
      expect(resolveSupportedLocale('es-EC')).toBe('es');
    });
  });

  describe('null/false arm — base unsupported or input falsy', () => {
    it('returns null when the base locale is itself unsupported (ternary false arm)', () => {
      // 'fr-FR' -> not direct, not alias, base 'fr' NOT supported -> null.
      expect(resolveSupportedLocale('fr-FR')).toBeNull();
      // 'zh-Hans' -> base 'zh' not supported -> null.
      expect(resolveSupportedLocale('zh-Hans')).toBeNull();
      // 'pt-BR' -> base 'pt' not supported -> null.
      expect(resolveSupportedLocale('pt-BR')).toBeNull();
      // 'ja-JP' -> base 'ja' not supported -> null.
      expect(resolveSupportedLocale('ja-JP')).toBeNull();
      // A bare unsupported code with no region also returns null.
      expect(resolveSupportedLocale('fr')).toBeNull();
      expect(resolveSupportedLocale('xyz')).toBeNull();
    });

    it('returns null for null input (optional-chain short-circuits to undefined)', () => {
      // value?.trim().replace().toLowerCase() -> undefined -> !normalized -> null.
      expect(resolveSupportedLocale(null)).toBeNull();
    });

    it('returns null for undefined input (optional-chain short-circuits to undefined)', () => {
      expect(resolveSupportedLocale(undefined)).toBeNull();
    });

    it('returns null for the empty string (normalized is falsy)', () => {
      // '' -> trim '' -> replace '' -> toLowerCase '' -> !normalized -> null.
      expect(resolveSupportedLocale('')).toBeNull();
    });

    it('returns null for a whitespace-only string (trims to empty -> falsy)', () => {
      expect(resolveSupportedLocale('    ')).toBeNull();
      expect(resolveSupportedLocale('\t\n')).toBeNull();
    });
  });
});

describe('getLocaleFallbackChain — branch coverage', () => {
  describe('single-element chain arm (locale === fallbackLocale)', () => {
    it('returns [locale] when the resolved locale is its own fallback (en)', () => {
      // SUPPORTED_LOCALE_REGISTRY.en.fallbackLocale === 'en' -> locale === fallbackLocale
      // -> ternary true -> [locale].
      expect(getLocaleFallbackChain('en')).toEqual(['en']);
    });

    it('returns a single-element chain when a regional alias resolves to en', () => {
      // 'en-GB' -> resolveSupportedLocale -> 'en' -> 'en' === 'en' -> ['en'].
      expect(getLocaleFallbackChain('en-GB')).toEqual(['en']);
    });

    it('returns a single-element chain for null/undefined/empty input (defaults to en)', () => {
      // normalizeLocale falls back to DEFAULT_LOCALE -> ['en'].
      expect(DEFAULT_LOCALE).toBe('en');
      expect(getLocaleFallbackChain(null)).toEqual(['en']);
      expect(getLocaleFallbackChain(undefined)).toEqual(['en']);
      expect(getLocaleFallbackChain('')).toEqual(['en']);
    });

    it('returns a single-element chain for an unsupported locale (defaults to en)', () => {
      // 'fr-FR' resolves to null -> normalizeLocale -> DEFAULT_LOCALE 'en' -> ['en'].
      expect(getLocaleFallbackChain('fr-FR')).toEqual(['en']);
    });
  });

  describe('multi-element chain arm (locale !== fallbackLocale)', () => {
    it('returns [locale, fallback] for the de supported locale', () => {
      // SUPPORTED_LOCALE_REGISTRY.de.fallbackLocale === 'en', 'de' !== 'en'
      // -> ternary false -> [locale, fallbackLocale].
      expect(getLocaleFallbackChain('de')).toEqual(['de', 'en']);
    });

    it('returns [locale, fallback] for the es supported locale', () => {
      expect(getLocaleFallbackChain('es')).toEqual(['es', 'en']);
    });

    it('returns a multi-element chain when a regional alias resolves to de/es', () => {
      expect(getLocaleFallbackChain('de-AT')).toEqual(['de', 'en']);
      expect(getLocaleFallbackChain('es-MX')).toEqual(['es', 'en']);
    });

    it('returns a multi-element chain when a base-fallback region resolves to de', () => {
      // 'de-XX' base-fallback -> 'de' -> ['de', 'en'].
      expect(getLocaleFallbackChain('de-XX')).toEqual(['de', 'en']);
    });
  });

  describe('return shape', () => {
    it('always emits members of SUPPORTED_LOCALES', () => {
      for (const input of ['en', 'de', 'es', 'en-GB', 'de-AT', 'es-MX', 'de-XX', 'fr-FR', null]) {
        const chain = getLocaleFallbackChain(input);
        for (const member of chain) {
          expect((SUPPORTED_LOCALES as readonly string[]).includes(member)).toBe(true);
        }
      }
    });
  });
});
