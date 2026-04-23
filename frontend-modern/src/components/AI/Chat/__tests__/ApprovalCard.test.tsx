import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { ApprovalCard } from '../ApprovalCard';
import type { PendingApproval } from '../types';

afterEach(cleanup);

function makeApproval(overrides?: Partial<PendingApproval>): PendingApproval {
  return {
    command: 'systemctl restart nginx',
    toolId: 'tool-1',
    toolName: 'run_command',
    runOnHost: false,
    ...overrides,
  };
}

describe('ApprovalCard', () => {
  it('renders the command text', () => {
    render(() => <ApprovalCard approval={makeApproval()} onApprove={vi.fn()} onSkip={vi.fn()} />);

    expect(screen.getByText('systemctl restart nginx')).toBeInTheDocument();
  });

  it('renders the "Approval Required" header', () => {
    render(() => <ApprovalCard approval={makeApproval()} onApprove={vi.fn()} onSkip={vi.fn()} />);

    expect(screen.getByText('Approval Required')).toBeInTheDocument();
  });

  it('renders Approve & Run and Skip buttons', () => {
    render(() => <ApprovalCard approval={makeApproval()} onApprove={vi.fn()} onSkip={vi.fn()} />);

    expect(screen.getByText('Approve & Run')).toBeInTheDocument();
    expect(screen.getByText('Skip')).toBeInTheDocument();
  });

  it('calls onApprove when Approve & Run is clicked', () => {
    const onApprove = vi.fn();
    render(() => <ApprovalCard approval={makeApproval()} onApprove={onApprove} onSkip={vi.fn()} />);

    fireEvent.click(screen.getByText('Approve & Run'));
    expect(onApprove).toHaveBeenCalledOnce();
  });

  it('calls onSkip when Skip is clicked', () => {
    const onSkip = vi.fn();
    render(() => <ApprovalCard approval={makeApproval()} onApprove={vi.fn()} onSkip={onSkip} />);

    fireEvent.click(screen.getByText('Skip'));
    expect(onSkip).toHaveBeenCalledOnce();
  });

  it('shows "Agent" badge when runOnHost is true', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({ runOnHost: true })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.getByText('Agent')).toBeInTheDocument();
  });

  it('does not show "Agent" badge when runOnHost is false', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({ runOnHost: false })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.queryByText('Agent')).not.toBeInTheDocument();
  });

  it('shows target host when targetHost is set', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({ targetHost: 'web-server-01' })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.getByText('→ web-server-01')).toBeInTheDocument();
  });

  it('does not show target host when targetHost is not set', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({ targetHost: undefined })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.queryByText('→ web-server-01')).not.toBeInTheDocument();
  });

  it('does not show target host when targetHost is empty string', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({ targetHost: '' })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    // Empty string is falsy, so Show should not render
    expect(screen.queryByText('→')).not.toBeInTheDocument();
  });

  describe('executing state', () => {
    it('shows "Running..." instead of "Run Command" when isExecuting is true', () => {
      render(() => (
        <ApprovalCard
          approval={makeApproval({ isExecuting: true })}
          onApprove={vi.fn()}
          onSkip={vi.fn()}
        />
      ));

      expect(screen.getByText('Running...')).toBeInTheDocument();
      expect(screen.queryByText('Approve & Run')).not.toBeInTheDocument();
    });

    it('disables both buttons when isExecuting is true', () => {
      render(() => (
        <ApprovalCard
          approval={makeApproval({ isExecuting: true })}
          onApprove={vi.fn()}
          onSkip={vi.fn()}
        />
      ));

      const buttons = screen.getAllByRole('button');
      expect(buttons).toHaveLength(2);
      for (const button of buttons) {
        expect(button).toBeDisabled();
      }
    });

    it('does not call onApprove when approve button is clicked while executing', () => {
      const onApprove = vi.fn();
      render(() => (
        <ApprovalCard
          approval={makeApproval({ isExecuting: true })}
          onApprove={onApprove}
          onSkip={vi.fn()}
        />
      ));

      fireEvent.click(screen.getByRole('button', { name: /running/i }));
      expect(onApprove).not.toHaveBeenCalled();
    });

    it('does not call onSkip when skip button is clicked while executing', () => {
      const onSkip = vi.fn();
      render(() => (
        <ApprovalCard
          approval={makeApproval({ isExecuting: true })}
          onApprove={vi.fn()}
          onSkip={onSkip}
        />
      ));

      fireEvent.click(screen.getByRole('button', { name: /skip/i }));
      expect(onSkip).not.toHaveBeenCalled();
    });
  });

  it('shows both host badge and target host together', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({ runOnHost: true, targetHost: 'db-primary' })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.getByText('Agent')).toBeInTheDocument();
    expect(screen.getByText('→ db-primary')).toBeInTheDocument();
  });

  it('renders long commands without truncation', () => {
    const longCommand = 'apt-get update && apt-get install -y nginx certbot python3-certbot-nginx';
    render(() => (
      <ApprovalCard
        approval={makeApproval({ command: longCommand })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.getByText(longCommand)).toBeInTheDocument();
  });

  it('renders governed action context when provided', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({
          risk: 'high',
          description: 'Restart the web service after applying the new config.',
          targetHost: 'web1',
          toolName: 'pulse_control',
        })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.getByText('HIGH')).toBeInTheDocument();
    expect(
      screen.getByText('Restart the web service after applying the new config.'),
    ).toBeInTheDocument();
    expect(screen.getByText('pulse_control')).toBeInTheDocument();
    expect(screen.getByText('web1')).toBeInTheDocument();
  });

  it('renders planned action and confidence details when provided', () => {
    render(() => (
      <ApprovalCard
        approval={makeApproval({
          auditId: 'action-123',
          plan: {
            action_id: 'action-123',
            request_id: 'approval-123',
            summary: 'Restart the nginx service.',
            requires_approval: true,
            approval_policy: 'admin',
            blast_radius: 'service interruption on target',
            rollback_available: true,
            plan_hash: 'abcdef1234567890',
            expires_at: '2026-04-23T12:30:00Z',
          },
          contextConfidence: {
            level: 'verified',
            summary: 'Target was resolved to a concrete resource before approval.',
            evidence: ['Target identifier bound to agent-1.'],
          },
        })}
        onApprove={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.getByText('Governed Plan')).toBeInTheDocument();
    expect(screen.getByText('Restart the nginx service.')).toBeInTheDocument();
    expect(screen.getByText('service interruption on target')).toBeInTheDocument();
    expect(screen.getByText('VERIFIED')).toBeInTheDocument();
    expect(screen.getByText('Target identifier bound to agent-1.')).toBeInTheDocument();
    expect(screen.getByText(/Audit action-123/)).toBeInTheDocument();
  });
});
