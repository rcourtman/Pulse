import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { AICostDashboard } from '../AICostDashboard';
import type { AICostSummary, AISettings } from '@/types/ai';

// ---- mock function handles ----
const getCostSummaryMock = vi.fn();
const getSettingsMock = vi.fn();
const resetCostHistoryMock = vi.fn();
const exportCostHistoryMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();

// ---- module mocks ----
vi.mock('@/api/ai', () => ({
  AIAPI: {
    getCostSummary: (...args: unknown[]) => getCostSummaryMock(...args),
    getSettings: (...args: unknown[]) => getSettingsMock(...args),
    resetCostHistory: (...args: unknown[]) => resetCostHistoryMock(...args),
    exportCostHistory: (...args: unknown[]) => exportCostHistoryMock(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
    info: vi.fn(),
    warning: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
  },
}));

// ---- fixtures ----

function baseSummary(overrides?: Partial<AICostSummary>): AICostSummary {
  return {
    days: 30,
    retention_days: 365,
    effective_days: 30,
    truncated: false,
    pricing_as_of: '2026-03-01',
    provider_models: [
      {
        provider: 'anthropic',
        model: 'claude-sonnet-4-6',
        input_tokens: 50000,
        output_tokens: 10000,
        total_tokens: 60000,
        estimated_usd: 0.42,
        pricing_known: true,
      },
    ],
    use_cases: [
      {
        use_case: 'chat',
        input_tokens: 30000,
        output_tokens: 6000,
        total_tokens: 36000,
        estimated_usd: 0.25,
        pricing_known: true,
      },
      {
        use_case: 'patrol',
        input_tokens: 20000,
        output_tokens: 4000,
        total_tokens: 24000,
        estimated_usd: 0.17,
        pricing_known: true,
      },
    ],
    targets: [],
    daily_totals: [
      { date: '2026-02-28', input_tokens: 25000, output_tokens: 5000, total_tokens: 30000, estimated_usd: 0.21 },
      { date: '2026-03-01', input_tokens: 25000, output_tokens: 5000, total_tokens: 30000, estimated_usd: 0.21 },
    ],
    totals: {
      provider: '',
      model: '',
      input_tokens: 50000,
      output_tokens: 10000,
      total_tokens: 60000,
      estimated_usd: 0.42,
      pricing_known: true,
    },
    ...overrides,
  };
}

function baseSettings(overrides?: Partial<AISettings>): AISettings {
  return {
    enabled: true,
    provider: 'anthropic',
    model: 'claude-sonnet-4-6',
    configured: true,
    configured_providers: [],
    autonomy_level: 'approval',
    control_level: 'read_only',
    cost_budget_usd_30d: 10,
    ...overrides,
  } as AISettings;
}

// ---- helpers ----

function renderDashboard() {
  return render(() => <AICostDashboard />);
}

// ---- tests ----

describe('AICostDashboard', () => {
  beforeEach(() => {
    getCostSummaryMock.mockReset();
    getSettingsMock.mockReset();
    resetCostHistoryMock.mockReset();
    exportCostHistoryMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();

    getCostSummaryMock.mockResolvedValue(baseSummary());
    getSettingsMock.mockResolvedValue(baseSettings());
  });

  // Save/restore URL methods that may be overwritten in export tests
  let origCreateObjectURL: typeof URL.createObjectURL;
  let origRevokeObjectURL: typeof URL.revokeObjectURL;

  beforeEach(() => {
    origCreateObjectURL = globalThis.URL.createObjectURL;
    origRevokeObjectURL = globalThis.URL.revokeObjectURL;
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    // Restore URL methods that vi.restoreAllMocks() doesn't handle
    globalThis.URL.createObjectURL = origCreateObjectURL;
    globalThis.URL.revokeObjectURL = origRevokeObjectURL;
  });

  // ---- initial load ----

  it('fetches cost summary and settings on mount', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(getCostSummaryMock).toHaveBeenCalledWith(30);
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });
  });

  it('shows loading state initially then displays data', async () => {
    renderDashboard();
    // "Loading…" appears during fetch
    expect(screen.getByText('Loading…')).toBeInTheDocument();
    // After data loads, summary appears
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
  });

  it('renders header with title', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Pulse Cost & Usage')).toBeInTheDocument();
    });
  });

  // ---- summary cards ----

  it('displays total tokens', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Total tokens')).toBeInTheDocument();
    });
    // 60000 formatted — appears in summary card and provider table, so use getAllByText
    const matches = screen.getAllByText('60,000');
    expect(matches.length).toBeGreaterThanOrEqual(1);
  });

  it('displays model/provider pair count', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Model/provider pairs')).toBeInTheDocument();
    });
    // Verify the count is rendered in the same card container
    const pairCard = screen.getByText('Model/provider pairs').closest('.p-3')!;
    expect(pairCard.textContent).toContain('1');
  });

  it('displays estimated spend in USD', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
    // The estimated spend card should contain 0.42 formatted as currency
    // (locale-dependent: may show $0.42 or US$0.42)
    const spendLabel = screen.getByText('Estimated spend');
    const spendCard = spendLabel.closest('.p-3')!;
    expect(spendCard.textContent).toMatch(/0\.42/);
  });

  // ---- use case breakdown ----

  it('displays chat and patrol token breakdowns', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Chat')).toBeInTheDocument();
    });
    expect(screen.getByText('36,000 tokens')).toBeInTheDocument();
    expect(screen.getByText('Patrol')).toBeInTheDocument();
    expect(screen.getByText('24,000 tokens')).toBeInTheDocument();
  });

  // ---- provider/model table ----

  it('renders provider/model table with correct data', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Anthropic')).toBeInTheDocument();
    });
    expect(screen.getByText('claude-sonnet-4-6')).toBeInTheDocument();
    expect(screen.getByText('50,000')).toBeInTheDocument(); // input tokens
    expect(screen.getByText('10,000')).toBeInTheDocument(); // output tokens
  });

  // ---- range buttons ----

  it('switches range to 7 days when clicking 7d button', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(getCostSummaryMock).toHaveBeenCalledWith(30);
    });

    getCostSummaryMock.mockClear();
    const button7d = screen.getByText('7d');
    fireEvent.click(button7d);

    await waitFor(() => {
      expect(getCostSummaryMock).toHaveBeenCalledWith(7);
    });
  });

  it('does not refetch when clicking the already-selected range', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(getCostSummaryMock).toHaveBeenCalledWith(30);
    });

    getCostSummaryMock.mockClear();
    const button30d = screen.getByText('30d');
    fireEvent.click(button30d);

    // Should not have been called again — same range
    expect(getCostSummaryMock).not.toHaveBeenCalled();
  });

  it('renders all range buttons', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
    expect(screen.getByText('1d')).toBeInTheDocument();
    expect(screen.getByText('7d')).toBeInTheDocument();
    expect(screen.getByText('30d')).toBeInTheDocument();
    expect(screen.getByText('90d')).toBeInTheDocument();
    expect(screen.getByText('1y')).toBeInTheDocument();
  });

  // ---- budget ----

  it('displays budget info from AI settings', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Budget alert (USD per 30d)')).toBeInTheDocument();
    });
    // Budget is $10/30d — should show 10.00 formatted as currency
    const budgetCard = screen.getByText('Budget alert (USD per 30d)').closest('.p-3')!;
    expect(budgetCard.textContent).toMatch(/10\.00/);
  });

  it('shows over-budget warning when spend exceeds budget', async () => {
    getCostSummaryMock.mockResolvedValue(
      baseSummary({
        totals: {
          provider: '',
          model: '',
          input_tokens: 50000,
          output_tokens: 10000,
          total_tokens: 60000,
          estimated_usd: 15,
          pricing_known: true,
        },
      }),
    );
    // Budget is $10/30d, spend is $15
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText(/is above your budget/)).toBeInTheDocument();
    });
  });

  it('does not show over-budget warning when within budget', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
    expect(screen.queryByText(/is above your budget/)).not.toBeInTheDocument();
  });

  it('does not show over-budget when no budget is configured', async () => {
    getSettingsMock.mockResolvedValue(baseSettings({ cost_budget_usd_30d: undefined }));
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
    expect(screen.queryByText(/is above your budget/)).not.toBeInTheDocument();
  });

  // ---- truncation ----

  it('shows truncation notice when data is truncated', async () => {
    getCostSummaryMock.mockResolvedValue(
      baseSummary({ truncated: true, effective_days: 90, retention_days: 365 }),
    );
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText(/Showing the last/)).toBeInTheDocument();
    });
    // Verify the truncation banner contains the retention days
    expect(screen.getByText(/Showing the last/).textContent).toContain('365');
  });

  // ---- pricing disclaimer ----

  it('shows pricing disclaimer', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(
        screen.getByText(/USD is an estimate based on public list prices/),
      ).toBeInTheDocument();
    });
  });

  it('shows pricing_as_of date', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText(/Prices as of 2026-03-01/)).toBeInTheDocument();
    });
  });

  it('lists unpriced provider/models when present', async () => {
    getCostSummaryMock.mockResolvedValue(
      baseSummary({
        provider_models: [
          {
            provider: 'anthropic',
            model: 'claude-sonnet-4-6',
            input_tokens: 50000,
            output_tokens: 10000,
            total_tokens: 60000,
            estimated_usd: 0.42,
            pricing_known: true,
          },
          {
            provider: 'ollama',
            model: 'llama3',
            input_tokens: 5000,
            output_tokens: 1000,
            total_tokens: 6000,
            pricing_known: false,
          },
        ],
      }),
    );
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText(/Pricing is unknown for/)).toBeInTheDocument();
    });
    expect(screen.getByText(/Ollama\/llama3/)).toBeInTheDocument();
  });

  // ---- targets table ----

  it('renders target table when targets are present', async () => {
    getCostSummaryMock.mockResolvedValue(
      baseSummary({
        targets: [
          {
            target_type: 'vm',
            target_id: '100',
            calls: 5,
            input_tokens: 10000,
            output_tokens: 2000,
            total_tokens: 12000,
            estimated_usd: 0.08,
            pricing_known: true,
          },
        ],
      }),
    );
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Top targets')).toBeInTheDocument();
    });
    expect(screen.getByText('vm:100')).toBeInTheDocument();
    expect(screen.getByText('12,000')).toBeInTheDocument();
  });

  it('does not render target table when no targets', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
    expect(screen.queryByText('Top targets')).not.toBeInTheDocument();
  });

  // ---- error states ----

  it('shows error message on initial load failure', async () => {
    getCostSummaryMock.mockRejectedValue(new Error('Network error'));
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });
    // Should NOT show notification on initial load failure
    expect(notificationErrorMock).not.toHaveBeenCalled();
  });

  it('shows fallback error message for non-Error thrown values', async () => {
    getCostSummaryMock.mockRejectedValue('some string');
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Failed to load usage data')).toBeInTheDocument();
    });
  });

  it('shows stale data banner with retry on refresh failure', async () => {
    // First load succeeds
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    // Second load fails (refresh)
    getCostSummaryMock.mockRejectedValue(new Error('Timeout'));
    const button7d = screen.getByText('7d');
    fireEvent.click(button7d);

    await waitFor(() => {
      expect(screen.getByText(/Couldn.t refresh/)).toBeInTheDocument();
    });
    // Should show notification for refresh failure
    expect(notificationErrorMock).toHaveBeenCalledWith(
      'Failed to refresh Pulse Assistant usage summary',
    );
    // Retry button is present
    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows loading state then transitions to data after resolve', async () => {
    let resolvePromise: (v: AICostSummary) => void;
    getCostSummaryMock.mockReturnValue(
      new Promise<AICostSummary>((resolve) => {
        resolvePromise = resolve;
      }),
    );
    renderDashboard();

    // While loading, we see "Loading usage…"
    expect(screen.getByText('Loading usage…')).toBeInTheDocument();
    // The "No usage data yet" message should NOT appear while loading
    expect(screen.queryByText('No usage data yet.')).not.toBeInTheDocument();

    // Now resolve — data appears
    resolvePromise!(baseSummary());
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
  });

  // ---- reset history ----

  it('calls resetCostHistory and shows success notification with backup file', async () => {
    // Mock window.confirm to return true
    vi.spyOn(globalThis, 'confirm').mockReturnValue(true);
    resetCostHistoryMock.mockResolvedValue({ ok: true, backup_file: '/data/backup.json' });

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Reset history'));

    await waitFor(() => {
      expect(resetCostHistoryMock).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(notificationSuccessMock).toHaveBeenCalledWith(
        expect.stringContaining('backup: /data/backup.json'),
      );
    });

  });

  it('calls resetCostHistory and shows success notification without backup file', async () => {
    vi.spyOn(globalThis, 'confirm').mockReturnValue(true);
    resetCostHistoryMock.mockResolvedValue({ ok: true });

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Reset history'));

    await waitFor(() => {
      expect(notificationSuccessMock).toHaveBeenCalledWith(
        'Pulse Assistant usage history reset',
      );
    });

  });

  it('does not call resetCostHistory when user cancels confirm', async () => {
    vi.spyOn(globalThis, 'confirm').mockReturnValue(false);

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Reset history'));

    expect(resetCostHistoryMock).not.toHaveBeenCalled();
  });

  it('shows error notification when reset fails', async () => {
    vi.spyOn(globalThis, 'confirm').mockReturnValue(true);
    resetCostHistoryMock.mockRejectedValue(new Error('Server error'));

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Reset history'));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'Failed to reset Pulse Assistant usage history',
      );
    });

  });

  // ---- export ----

  it('exports CSV by calling API with correct params', async () => {
    const mockBlob = new Blob(['csv,data'], { type: 'text/csv' });
    exportCostHistoryMock.mockResolvedValue({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    // Mock URL methods and anchor to prevent jsdom navigation errors
    globalThis.URL.createObjectURL = vi.fn().mockReturnValue('blob:test');
    globalThis.URL.revokeObjectURL = vi.fn();
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Export CSV'));

    await waitFor(() => {
      expect(exportCostHistoryMock).toHaveBeenCalledWith(30, 'csv');
    });
    // Wait for the full download flow: blob -> createObjectURL -> revokeObjectURL
    await waitFor(() => {
      expect(globalThis.URL.revokeObjectURL).toHaveBeenCalled();
    });
  });

  it('exports JSON by calling API with correct params', async () => {
    const mockBlob = new Blob(['{}'], { type: 'application/json' });
    exportCostHistoryMock.mockResolvedValue({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    globalThis.URL.createObjectURL = vi.fn().mockReturnValue('blob:test');
    globalThis.URL.revokeObjectURL = vi.fn();
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Export JSON'));

    await waitFor(() => {
      expect(exportCostHistoryMock).toHaveBeenCalledWith(30, 'json');
    });
    // Wait for the full download flow
    await waitFor(() => {
      expect(globalThis.URL.revokeObjectURL).toHaveBeenCalled();
    });
  });

  it('shows error notification when export fails with bad status', async () => {
    exportCostHistoryMock.mockResolvedValue({
      ok: false,
      status: 500,
    });

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Export CSV'));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'Failed to export Pulse Assistant usage history',
      );
    });
  });

  it('shows error notification when export throws', async () => {
    exportCostHistoryMock.mockRejectedValue(new Error('Network'));

    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Export JSON'));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'Failed to export Pulse Assistant usage history',
      );
    });

  });

  // ---- no pricing known ----

  it('shows em-dash for estimated spend when no pricing is known', async () => {
    getCostSummaryMock.mockResolvedValue(
      baseSummary({
        provider_models: [
          {
            provider: 'ollama',
            model: 'llama3',
            input_tokens: 5000,
            output_tokens: 1000,
            total_tokens: 6000,
            pricing_known: false,
          },
        ],
        totals: {
          provider: '',
          model: '',
          input_tokens: 5000,
          output_tokens: 1000,
          total_tokens: 6000,
          pricing_known: false,
        },
      }),
    );
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });
    // Estimated spend card should show "—"
    const spendCard = screen.getByText('Estimated spend').closest('.p-3')!;
    expect(spendCard.textContent).toContain('—');
  });

  // ---- sparkline rendering ----

  it('shows daily trend sparklines when multiple daily totals exist', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Daily estimated USD')).toBeInTheDocument();
      expect(screen.getByText('Daily total tokens')).toBeInTheDocument();
    });
    // With 2 daily totals, sparklines should render (SVGs present)
    const svgs = document.querySelectorAll('svg');
    expect(svgs.length).toBeGreaterThanOrEqual(2);
  });

  it('shows "No daily token trend yet" when less than 2 daily totals', async () => {
    getCostSummaryMock.mockResolvedValue(
      baseSummary({ daily_totals: [] }),
    );
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('No daily token trend yet.')).toBeInTheDocument();
    });
  });

  // ---- retention ----

  it('shows retention days', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText(/History retention: 365 days/)).toBeInTheDocument();
    });
  });

  // ---- budget pro-rating ----

  it('pro-rates budget for non-30d ranges', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Estimated spend')).toBeInTheDocument();
    });

    // Default 30d: budget is $10 * 30/30 = $10
    expect(screen.getByText(/Pro-rated for 30d/)).toBeInTheDocument();

    // Switch to 7d: budget becomes $10 * 7/30 ≈ $2.33
    getCostSummaryMock.mockResolvedValue(baseSummary());
    fireEvent.click(screen.getByText('7d'));
    await waitFor(() => {
      expect(screen.getByText(/Pro-rated for 7d/)).toBeInTheDocument();
    });
  });

  // ---- budget with zero/negative/NaN values ----

  it('handles zero budget gracefully (no budget shown)', async () => {
    getSettingsMock.mockResolvedValue(baseSettings({ cost_budget_usd_30d: 0 }));
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Budget alert (USD per 30d)')).toBeInTheDocument();
    });
    // parsedBudgetUSD30d should return null for 0, so budget shows "—"
    const budgetCard = screen.getByText('Budget alert (USD per 30d)').closest('.p-3')!;
    expect(budgetCard.textContent).toContain('—');
  });

  it('handles negative budget gracefully (no budget shown)', async () => {
    getSettingsMock.mockResolvedValue(baseSettings({ cost_budget_usd_30d: -5 }));
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Budget alert (USD per 30d)')).toBeInTheDocument();
    });
    const budgetCard = screen.getByText('Budget alert (USD per 30d)').closest('.p-3')!;
    expect(budgetCard.textContent).toContain('—');
  });

  // ---- settings load failure ----

  it('handles settings load failure gracefully (no budget shown)', async () => {
    getSettingsMock.mockRejectedValue(new Error('Auth error'));
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Budget alert (USD per 30d)')).toBeInTheDocument();
    });
    const budgetCard = screen.getByText('Budget alert (USD per 30d)').closest('.p-3')!;
    expect(budgetCard.textContent).toContain('—');
  });
});
