import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const sourceText = readFileSync(
  resolve(__dirname, '..', 'ResourceActionHistory.tsx'),
  'utf-8',
);

describe('ResourceActionHistory verification rendering', () => {
  it('renders the post-dispatch verification outcome on each audit row when ran=true', () => {
    // The broker's read-after-write verification outcome lives on
    // result.verification (ActionVerificationResult). The audit history row
    // must surface it so operators can see "Pulse confirmed the workload
    // service is active" — not just "command exit 0". Pin the wiring so the
    // surface cannot silently regress to an output-only render.
    expect(sourceText).toContain('result()?.verification?.ran');
    expect(sourceText).toContain('Verified');
    expect(sourceText).toContain('Verification failed');
  });

  it('shows the verification command and output verbatim when present', () => {
    // The verification command (e.g. "systemctl is-active 'nginx'") is
    // operator-trusted context, not just internal plumbing — it must be
    // rendered so the operator can see exactly what Pulse read back, not
    // just a yes/no.
    expect(sourceText).toContain('v.command');
    expect(sourceText).toContain('v.output');
    expect(sourceText).toContain('v.note');
  });
});
