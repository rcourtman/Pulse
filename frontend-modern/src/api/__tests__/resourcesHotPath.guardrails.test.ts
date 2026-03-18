import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const resourcesHandlerSource = readFileSync(resolve(process.cwd(), '../internal/api/resources.go'), 'utf8');

describe('resource API hot-path guardrails', () => {
  it('reuses one registry snapshot per request when deriving canonical by-type aggregations', () => {
    expect(resourcesHandlerSource.match(/allResources := registry\.List\(\)/g) ?? []).toHaveLength(
      2,
    );
    expect(
      resourcesHandlerSource.match(/computeResourceContractByType\(allResources\)/g) ?? [],
    ).toHaveLength(2);
    expect(resourcesHandlerSource).not.toContain('computeResourceContractByType(registry.List())');
  });
});
