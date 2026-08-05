import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const resourcesHandlerSource = readFileSync(
  resolve(process.cwd(), '../internal/api/resources.go'),
  'utf8',
);

describe('resource API hot-path guardrails', () => {
  it('reuses the per-generation presentation snapshot across list and stats requests', () => {
    expect(
      resourcesHandlerSource.match(/sharedPresentationResources\(orgID\)/g) ?? [],
    ).toHaveLength(2);
    expect(resourcesHandlerSource).toContain('allResources := flatCopyResources(sharedResources)');
    expect(
      resourcesHandlerSource.match(/computeResourceContractStats\(allResources\)/g) ?? [],
    ).toHaveLength(2);
    expect(resourcesHandlerSource).not.toContain('allResources := registry.List()');
    expect(resourcesHandlerSource).not.toContain('computeResourceContractByType(registry.List())');
  });
});
