import type { DiscoveryFact } from '@/types/discovery';

// Mirror the backend context-pack ranking (servicediscovery filterImportantFacts):
// the human Discovery tab shows a capped subset of discovered facts, so the most
// actionable ones — how to restart (service), auth (security), what it depends
// on — must be ordered first and never dropped by the cap in favour of a purely
// informational fact (e.g. a version string).
const CATEGORY_RANK: Record<string, number> = {
  service: 0, // how to restart/reload — most actionable
  security: 1, // auth/access
  dependency: 2, // what it connects to
  hardware: 3, // GPU/TPU
  version: 4, // version info
  storage: 5, // backing dataset/disk
};

const rankOf = (category?: string): number =>
  category !== undefined && category in CATEGORY_RANK ? CATEGORY_RANK[category] : 6;

/**
 * Stable-sort facts by actionability (service > security > dependency > hardware
 * > version > storage > other), preserving the analyzer's within-category order.
 * Does not mutate the input.
 */
export function orderFactsByActionability(facts: DiscoveryFact[]): DiscoveryFact[] {
  return facts
    .map((fact, index) => ({ fact, index }))
    .sort((a, b) => rankOf(a.fact.category) - rankOf(b.fact.category) || a.index - b.index)
    .map((entry) => entry.fact);
}
