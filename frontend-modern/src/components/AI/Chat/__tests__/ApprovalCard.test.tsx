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

  it('renders Run Command and Skip buttons', () => {
    render(() => <ApprovalCard approval={makeApproval()} onApprove={vi.fn()} onSkip={vi.fn()} />);

    expect(screen.getByText('Run Command')).toBeInTheDocument();
    expect(screen.getByText('Skip')).toBeInTheDocument();
  });

  it('calls onApprove when Run Command is clicked', () => {
    const onApprove = vi.fn();
    render(() => <ApprovalCard approval={makeApproval()} onApprove={onApprove} onSkip={vi.fn()} />);

    fireEvent.click(screen.getByText('Run Command'));
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
      expect(screen.queryByText('Run Command')).not.toBeInTheDocument();
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
});
