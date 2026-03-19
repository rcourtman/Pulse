import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { ResourcePolicySummary } from '../ResourcePolicySummary';

describe('ResourcePolicySummary', () => {
  it('renders governed resource posture counts and redaction hints', () => {
    render(() => (
      <ResourcePolicySummary
        posture={{
          total_resources: 5,
          sensitivity_counts: {
            public: 1,
            internal: 2,
            sensitive: 1,
            restricted: 1,
          },
          routing_counts: {
            'cloud-summary': 2,
            'local-first': 2,
            'local-only': 1,
          },
          redaction_counts: {
            hostname: 3,
            'ip-address': 1,
          },
        }}
      />
    ));

    expect(screen.getByText('Data Governance')).toBeInTheDocument();
    expect(screen.getByText('5 governed resources')).toBeInTheDocument();
    expect(screen.getByText('Public')).toBeInTheDocument();
    expect(screen.getByText('Local Only')).toBeInTheDocument();
    expect(screen.getByText('Hostname 3')).toBeInTheDocument();
    expect(screen.getByText('IP Address 1')).toBeInTheDocument();
  });
});
