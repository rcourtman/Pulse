export interface FrozenLineStyleSnapshot {
  version: 1;
  generated_from: { source_sha: string; source_proof_receipt?: string };
  stylesheet: { path: string; sha256: string; bytes?: number };
  inputs: Record<string, string>;
}

export function frozenLineStyleInputs(root?: string): Record<string, string>;
export function verifyFrozenLineStyles(root?: string): FrozenLineStyleSnapshot;
