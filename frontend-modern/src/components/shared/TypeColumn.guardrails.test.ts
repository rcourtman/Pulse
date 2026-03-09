import { describe, expect, it } from 'vitest';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import recoverySource from '@/components/Recovery/Recovery.tsx?raw';

const tsxSources = import.meta.glob('../../**/*.tsx', {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

const TYPE_COLUMN_PATTERN =
  /\{\s*id:\s*'type',\s*label:\s*'Type'[\s\S]*?toggleable:\s*true[\s\S]*?\}/;
const TYPE_COLUMN_DEFINITION_PATTERN = /\{\s*id:\s*'type',\s*label:\s*'Type'[\s\S]*?\}/g;

describe('type column guardrails', () => {
  it('keeps runtime type columns user-toggleable', () => {
    expect(guestRowSource).toMatch(TYPE_COLUMN_PATTERN);
    expect(recoverySource).toMatch(TYPE_COLUMN_PATTERN);
  });

  it('limits runtime Type columns to the known allowlist', () => {
    const runtimeEntries = Object.entries(tsxSources).filter(
      ([path]) => !path.endsWith('.test.tsx'),
    );

    const typeColumnUsers = runtimeEntries
      .filter(([, source]) => {
        const matches = source.match(TYPE_COLUMN_DEFINITION_PATTERN);
        return Boolean(matches && matches.some((match) => match.includes("id: 'type'")));
      })
      .map(([path]) => path)
      .sort();

    expect(typeColumnUsers).toEqual([
      '../Dashboard/GuestRow.tsx',
      '../Recovery/Recovery.tsx',
    ]);
  });
});
