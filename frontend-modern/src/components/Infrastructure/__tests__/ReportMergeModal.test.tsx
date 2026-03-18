import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@solidjs/testing-library';

/* ── hoisted mocks ────────────────────────────────────────────── */
const { apiFetchMock, showErrorMock, showSuccessMock } = vi.hoisted(() => ({
  apiFetchMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
}));

vi.mock('@/utils/toast', () => ({
  showError: (...args: unknown[]) => showErrorMock(...args),
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
}));

/* ── component import (after mocks) ──────────────────────────── */
import { ReportMergeModal } from '../ReportMergeModal';

/* ── helpers ──────────────────────────────────────────────────── */
const defaultProps = () => ({
  isOpen: true,
  resourceId: 'host:abc-123',
  resourceName: 'my-server',
  sources: ['proxmox', 'agent'],
  onClose: vi.fn(),
  onReported: vi.fn(),
});

/* ── lifecycle ────────────────────────────────────────────────── */
beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  cleanup();
  document.body.style.overflow = '';
});

/* ================================================================
   Tests
   ================================================================ */

describe('ReportMergeModal', () => {
  /* ── Rendering ─────────────────────────────────────────────── */

  describe('rendering', () => {
    it('shows the dialog title and description', () => {
      render(() => <ReportMergeModal {...defaultProps()} />);

      expect(screen.getByText('Split Merged Resource')).toBeInTheDocument();
      expect(
        screen.getByText(/Use this when two systems were combined incorrectly/),
      ).toBeInTheDocument();
    });

    it('displays the resource name and id', () => {
      render(() => <ReportMergeModal {...defaultProps()} />);

      expect(screen.getByText('my-server')).toBeInTheDocument();
      expect(screen.getByText('host:abc-123')).toBeInTheDocument();
    });

    it('maps source strings to canonical platform labels', () => {
      const props = defaultProps();
      props.sources = ['proxmox', 'agent', 'docker', 'pbs', 'pmg', 'kubernetes'];
      render(() => <ReportMergeModal {...props} />);

      expect(screen.getByText('PVE')).toBeInTheDocument();
      expect(screen.getByText('Agent')).toBeInTheDocument();
      expect(screen.getByText('Containers')).toBeInTheDocument();
      expect(screen.getByText('PBS')).toBeInTheDocument();
      expect(screen.getByText('PMG')).toBeInTheDocument();
      expect(screen.getByText('K8s')).toBeInTheDocument();
    });

    it('passes through unknown source strings as-is', () => {
      const props = defaultProps();
      props.sources = ['custom-source', 'agent'];
      render(() => <ReportMergeModal {...props} />);

      expect(screen.getByText('Custom Source')).toBeInTheDocument();
    });

    it('normalises source strings case-insensitively', () => {
      const props = defaultProps();
      props.sources = ['PROXMOX', 'Docker'];
      render(() => <ReportMergeModal {...props} />);

      expect(screen.getByText('PVE')).toBeInTheDocument();
      expect(screen.getByText('Containers')).toBeInTheDocument();
    });

    it('shows a notes textarea with placeholder', () => {
      render(() => <ReportMergeModal {...defaultProps()} />);

      const textarea = screen.getByPlaceholderText(
        'Example: Agent running on a different host with same hostname.',
      );
      expect(textarea).toBeInTheDocument();
      expect(textarea.tagName).toBe('TEXTAREA');
    });

    it('shows Cancel and Split Resource buttons', () => {
      render(() => <ReportMergeModal {...defaultProps()} />);

      expect(screen.getByText('Cancel')).toBeInTheDocument();
      expect(screen.getByText('Split Resource')).toBeInTheDocument();
    });
  });

  /* ── ARIA / accessibility ──────────────────────────────────── */

  describe('accessibility', () => {
    it('generates sanitised aria ids from resourceId', () => {
      render(() => <ReportMergeModal {...defaultProps()} />);

      const title = screen.getByText('Split Merged Resource');
      expect(title.id).toBe('report-merge-title-host-abc-123');

      const desc = screen.getByText(/Use this when two systems were combined incorrectly/);
      expect(desc.id).toBe('report-merge-description-host-abc-123');
    });

    it('sanitises special characters in resourceId for aria ids', () => {
      const props = defaultProps();
      props.resourceId = 'node/pve1.local@cluster';
      render(() => <ReportMergeModal {...props} />);

      const title = screen.getByText('Split Merged Resource');
      expect(title.id).toBe('report-merge-title-node-pve1-local-cluster');
    });

    it('has a close button with aria-label', () => {
      render(() => <ReportMergeModal {...defaultProps()} />);

      expect(screen.getByLabelText('Close')).toBeInTheDocument();
    });
  });

  /* ── Button states ─────────────────────────────────────────── */

  describe('button states', () => {
    it('disables submit when fewer than 2 sources', () => {
      const props = defaultProps();
      props.sources = ['proxmox'];
      render(() => <ReportMergeModal {...props} />);

      const btn = screen.getByText('Split Resource');
      expect(btn).toBeDisabled();
    });

    it('enables submit when 2+ sources', () => {
      render(() => <ReportMergeModal {...defaultProps()} />);

      const btn = screen.getByText('Split Resource');
      expect(btn).not.toBeDisabled();
    });
  });

  /* ── Close / Cancel ────────────────────────────────────────── */

  describe('close actions', () => {
    it('calls onClose when Cancel is clicked', () => {
      const props = defaultProps();
      render(() => <ReportMergeModal {...props} />);

      fireEvent.click(screen.getByText('Cancel'));
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when X button is clicked', () => {
      const props = defaultProps();
      render(() => <ReportMergeModal {...props} />);

      fireEvent.click(screen.getByLabelText('Close'));
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });
  });

  /* ── Successful submission ─────────────────────────────────── */

  describe('successful submission', () => {
    it('sends POST to correct endpoint with sources and trimmed notes', async () => {
      apiFetchMock.mockResolvedValueOnce({ ok: true });
      const props = defaultProps();
      render(() => <ReportMergeModal {...props} />);

      // Type notes
      const textarea = screen.getByPlaceholderText(
        'Example: Agent running on a different host with same hostname.',
      );
      fireEvent.input(textarea, {
        target: { value: '  different hostname  ' },
      });

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(apiFetchMock).toHaveBeenCalledTimes(1);
      });

      const [url, opts] = apiFetchMock.mock.calls[0];
      expect(url).toBe('/api/resources/host%3Aabc-123/report-merge');
      expect(opts.method).toBe('POST');
      expect(opts.headers['Content-Type']).toBe('application/json');

      const body = JSON.parse(opts.body);
      expect(body.sources).toEqual(['proxmox', 'agent']);
      expect(body.notes).toBe('different hostname');
    });

    it('omits notes from body when textarea is empty', async () => {
      apiFetchMock.mockResolvedValueOnce({ ok: true });
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(apiFetchMock).toHaveBeenCalledTimes(1);
      });

      const body = JSON.parse(apiFetchMock.mock.calls[0][1].body);
      expect(body.notes).toBeUndefined();
    });

    it('shows success toast after submit', async () => {
      apiFetchMock.mockResolvedValueOnce({ ok: true });
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(showSuccessMock).toHaveBeenCalledWith('Resource split applied');
      });
    });

    it('calls onReported and onClose after success', async () => {
      apiFetchMock.mockResolvedValueOnce({ ok: true });
      const props = defaultProps();
      render(() => <ReportMergeModal {...props} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(props.onReported).toHaveBeenCalledTimes(1);
        expect(props.onClose).toHaveBeenCalledTimes(1);
      });
    });

    it('resets notes after success', async () => {
      apiFetchMock.mockResolvedValueOnce({ ok: true });
      render(() => <ReportMergeModal {...defaultProps()} />);

      const textarea = screen.getByPlaceholderText(
        'Example: Agent running on a different host with same hostname.',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'some notes' } });

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(textarea.value).toBe('');
      });
    });

    it('works without onReported callback', async () => {
      apiFetchMock.mockResolvedValueOnce({ ok: true });
      const props = defaultProps();
      props.onReported = undefined as any;
      render(() => <ReportMergeModal {...props} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(props.onClose).toHaveBeenCalledTimes(1);
      });
      expect(showSuccessMock).toHaveBeenCalledWith('Resource split applied');
      expect(showErrorMock).not.toHaveBeenCalled();
    });
  });

  /* ── Error handling ────────────────────────────────────────── */

  describe('error handling', () => {
    it('shows error when API returns non-ok status', async () => {
      apiFetchMock.mockResolvedValueOnce({
        ok: false,
        text: () => Promise.resolve('Merge not found'),
      });
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(screen.getByText('Merge not found')).toBeInTheDocument();
      });
      expect(showErrorMock).toHaveBeenCalledWith('Unable to report merge', 'Merge not found');
    });

    it('uses fallback message when response body is empty', async () => {
      apiFetchMock.mockResolvedValueOnce({
        ok: false,
        text: () => Promise.resolve(''),
      });
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(screen.getByText('Failed to report merge')).toBeInTheDocument();
      });
    });

    it('uses fallback message when response.text() rejects', async () => {
      apiFetchMock.mockResolvedValueOnce({
        ok: false,
        text: () => Promise.reject(new Error('read error')),
      });
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(screen.getByText('Failed to report merge')).toBeInTheDocument();
      });
    });

    it('handles network error (fetch rejects)', async () => {
      apiFetchMock.mockRejectedValueOnce(new Error('Network error'));
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(screen.getByText('Network error')).toBeInTheDocument();
      });
      expect(showErrorMock).toHaveBeenCalledWith('Unable to report merge', 'Network error');
    });

    it('uses fallback for non-Error thrown values', async () => {
      apiFetchMock.mockRejectedValueOnce('string error');
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(screen.getByText('Failed to report merge')).toBeInTheDocument();
      });
    });

    it('does not call onClose or onReported on error', async () => {
      apiFetchMock.mockRejectedValueOnce(new Error('fail'));
      const props = defaultProps();
      render(() => <ReportMergeModal {...props} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(showErrorMock).toHaveBeenCalled();
      });
      expect(props.onReported).not.toHaveBeenCalled();
      expect(props.onClose).not.toHaveBeenCalled();
    });
  });

  /* ── Submitting state ──────────────────────────────────────── */

  describe('submitting state', () => {
    it('shows Submitting... text during submission', async () => {
      let resolveSubmit!: (v: unknown) => void;
      apiFetchMock.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveSubmit = resolve;
        }),
      );
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      await waitFor(() => {
        expect(screen.getByText('Submitting...')).toBeInTheDocument();
      });

      // Cancel button should be disabled during submission
      expect(screen.getByText('Cancel')).toBeDisabled();

      // Resolve to complete
      resolveSubmit({ ok: true });

      await waitFor(() => {
        expect(screen.queryByText('Submitting...')).not.toBeInTheDocument();
      });
    });

    it('prevents double submission', async () => {
      let resolveSubmit!: (v: unknown) => void;
      apiFetchMock.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveSubmit = resolve;
        }),
      );
      const props = defaultProps();
      render(() => <ReportMergeModal {...props} />);

      const btn = screen.getByText('Split Resource');
      fireEvent.click(btn);

      await waitFor(() => {
        expect(screen.getByText('Submitting...')).toBeInTheDocument();
      });

      // Click again while submitting - should not trigger another call
      fireEvent.click(screen.getByText('Submitting...'));

      expect(apiFetchMock).toHaveBeenCalledTimes(1);

      // Settle the promise so callbacks run before test ends
      resolveSubmit({ ok: true });
      await waitFor(() => {
        expect(props.onClose).toHaveBeenCalled();
      });
    });

    it('re-enables submit after error', async () => {
      apiFetchMock.mockRejectedValueOnce(new Error('fail'));
      render(() => <ReportMergeModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Split Resource'));

      // First confirm we entered the submitting state
      await waitFor(() => {
        expect(showErrorMock).toHaveBeenCalled();
      });

      // Now verify it recovered: button text is back and it's not disabled
      const btn = screen.getByText('Split Resource');
      expect(btn).not.toBeDisabled();
    });
  });
});
