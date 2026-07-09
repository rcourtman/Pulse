import { describe, expect, it } from 'vitest';
import type { HelpContent } from '../types';
import { alertsHelpContent } from '../alerts';
import { aiHelpContent } from '../ai';
import { updatesHelpContent } from '../updates';
import { getHelpContent } from '../index';

const allContent = [...alertsHelpContent, ...aiHelpContent, ...updatesHelpContent];
const registeredIds = allContent.map((item) => item.id);

describe('getHelpContent', () => {
  describe('registered id lookup', () => {
    it.each(registeredIds)('returns the source entry by reference for "%s"', (id) => {
      const expected = allContent.find((item) => item.id === id);
      const resolved = getHelpContent(id);

      expect(resolved).toBe(expected);
      expect(resolved?.id).toBe(id);
    });
  });

  describe('unknown id', () => {
    it('returns undefined for an id that is not registered', () => {
      expect(getHelpContent('does.not.exist')).toBeUndefined();
    });

    it('returns undefined for an empty string', () => {
      expect(getHelpContent('')).toBeUndefined();
    });

    it('is case-sensitive (a differently-cased registered id resolves to undefined)', () => {
      expect(getHelpContent('ALERTS.THRESHOLDS.DELAY')).toBeUndefined();
      expect(getHelpContent('ai.ollama.baseurl')).toBeUndefined();
    });

    it('does not resolve ids with surrounding whitespace', () => {
      expect(getHelpContent(' alerts.thresholds.delay')).toBeUndefined();
      expect(getHelpContent('alerts.thresholds.delay ')).toBeUndefined();
      expect(getHelpContent('alerts.thresholds.delay\n')).toBeUndefined();
      expect(getHelpContent('\tupdates.pulse.channel')).toBeUndefined();
    });

    it('does not resolve a registered id with an extra trailing segment', () => {
      expect(getHelpContent('alerts.thresholds.delay.')).toBeUndefined();
      expect(getHelpContent('updates.pulse.channel.extra')).toBeUndefined();
    });
  });

  describe('entry shape', () => {
    it('every registered id resolves to a well-formed HelpContent entry', () => {
      for (const id of registeredIds) {
        const entry = getHelpContent(id) as HelpContent;

        // Required fields are present and non-empty strings.
        expect(entry).toBeDefined();
        expect(typeof entry.id).toBe('string');
        expect(entry.id.length).toBeGreaterThan(0);
        expect(typeof entry.title).toBe('string');
        expect(entry.title.length).toBeGreaterThan(0);
        expect(typeof entry.description).toBe('string');
        expect(entry.description.length).toBeGreaterThan(0);

        // Optional fields, when present, must have the declared type.
        if (entry.examples !== undefined) {
          expect(Array.isArray(entry.examples)).toBe(true);
          expect(entry.examples.length).toBeGreaterThan(0);
          for (const example of entry.examples) {
            expect(typeof example).toBe('string');
            expect(example.length).toBeGreaterThan(0);
          }
        }
        if (entry.related !== undefined) {
          expect(Array.isArray(entry.related)).toBe(true);
          expect(entry.related.length).toBeGreaterThan(0);
          for (const relatedId of entry.related) {
            expect(typeof relatedId).toBe('string');
            expect(relatedId.length).toBeGreaterThan(0);
          }
        }
        if (entry.docUrl !== undefined) {
          expect(typeof entry.docUrl).toBe('string');
          expect(entry.docUrl.length).toBeGreaterThan(0);
        }
        if (entry.addedInVersion !== undefined) {
          expect(typeof entry.addedInVersion).toBe('string');
          expect(entry.addedInVersion.length).toBeGreaterThan(0);
        }
      }
    });

    it('every related id cross-reference resolves to a registered entry', () => {
      const withRelated = allContent.filter((item) => (item.related?.length ?? 0) > 0);
      expect(withRelated.length).toBeGreaterThan(0);

      for (const item of withRelated) {
        for (const relatedId of item.related ?? []) {
          const resolved = getHelpContent(relatedId);
          expect(resolved).toBeDefined();
          expect(resolved?.id).toBe(relatedId);
        }
      }
    });
  });
});
