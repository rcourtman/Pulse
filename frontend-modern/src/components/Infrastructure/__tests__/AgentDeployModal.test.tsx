import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';

/* ── hoisted mocks ────────────────────────────────────────────── */

// Build a controllable mock wizard factory.
// Each test can override individual fields before render.
const { useDeployWizardMock } = vi.hoisted(() => ({
  useDeployWizardMock: vi.fn(),
}));

vi.mock('@/hooks/useDeployWizard', () => ({
  useDeployWizard: useDeployWizardMock,
}));

// Track ConfirmStep props to verify parent wiring.
const { confirmStepPropsSpy } = vi.hoisted(() => ({
  confirmStepPropsSpy: vi.fn(),
}));

// Stub child step components — we don't test their internals here.
vi.mock('../deploy/CandidatesStep', () => ({
  CandidatesStep: () => <div data-testid="candidates-step">CandidatesStep</div>,
}));
vi.mock('../deploy/PreflightStep', () => ({
  PreflightStep: () => <div data-testid="preflight-step">PreflightStep</div>,
}));
vi.mock('../deploy/ConfirmStep', () => ({
  ConfirmStep: (props: Record<string, unknown>) => {
    confirmStepPropsSpy(props);
    return <div data-testid="confirm-step">ConfirmStep</div>;
  },
}));
vi.mock('../deploy/DeployingStep', () => ({
  DeployingStep: () => <div data-testid="deploying-step">DeployingStep</div>,
}));
vi.mock('../deploy/ResultsStep', () => ({
  ResultsStep: () => <div data-testid="results-step">ResultsStep</div>,
}));

// Stub StepIndicator to expose the currentStep prop for assertion.
vi.mock('@/components/SetupWizard/StepIndicator', () => ({
  StepIndicator: (props: { steps: string[]; currentStep: number }) => (
    <div data-testid="step-indicator" data-current-step={props.currentStep}>
      {props.steps.join(' | ')}
    </div>
  ),
}));

/* ── component import (after mocks) ──────────────────────────── */
import { AgentDeployModal } from '../AgentDeployModal';

/* ── helpers ──────────────────────────────────────────────────── */

type WizardStep = 'candidates' | 'preflight' | 'confirm' | 'deploying' | 'results';

/** Creates a full mock wizard with sensible defaults. */
function createMockWizard(overrides: Partial<Record<string, unknown>> = {}) {
  const [step, setStep] = createSignal<WizardStep>((overrides.step as WizardStep) ?? 'candidates');
  const [selectedNodeIds] = createSignal<Set<string>>(
    (overrides.selectedNodeIds as Set<string>) ?? new Set<string>(),
  );
  const [selectedSourceAgent] = createSignal<string>(
    (overrides.selectedSourceAgent as string) ?? '',
  );
  const [startingPreflight] = createSignal<boolean>(
    (overrides.startingPreflight as boolean) ?? false,
  );
  const [startingDeploy] = createSignal<boolean>((overrides.startingDeploy as boolean) ?? false);
  const [canceling] = createSignal<boolean>((overrides.canceling as boolean) ?? false);
  const [retrying] = createSignal<boolean>((overrides.retrying as boolean) ?? false);

  const [confirmSelectedNodeIds] = createSignal<Set<string>>(
    (overrides.confirmSelectedNodeIds as Set<string>) ?? new Set<string>(),
  );

  const readyNodesData = (overrides.readyNodes as unknown[]) ?? [];
  const readyNodes: Accessor<unknown[]> = () => readyNodesData;

  const retryableTargetsData = (overrides.retryableTargets as unknown[]) ?? [];
  const retryableTargets: Accessor<unknown[]> = () => retryableTargetsData;

  const isOperationActive: Accessor<boolean> =
    (overrides.isOperationActive as Accessor<boolean>) ??
    (() => step() === 'preflight' || step() === 'deploying');

  return {
    step,
    setStep,
    selectedNodeIds,
    selectedSourceAgent,
    confirmSelectedNodeIds,
    startingPreflight,
    startingDeploy,
    canceling,
    retrying,
    readyNodes,
    retryableTargets,
    isOperationActive,
    startPreflight: overrides.startPreflight ?? vi.fn(),
    startDeploy: overrides.startDeploy ?? vi.fn(),
    cancelDeploy: overrides.cancelDeploy ?? vi.fn(),
    retryFailed: overrides.retryFailed ?? vi.fn(),
  };
}

const defaultProps = () => ({
  isOpen: true,
  clusterId: 'cluster-1',
  clusterName: 'My Cluster',
  onClose: vi.fn(),
});

/* ── lifecycle ────────────────────────────────────────────────── */
beforeEach(() => {
  vi.clearAllMocks();
  // Default: return a basic wizard in 'candidates' step.
  useDeployWizardMock.mockReturnValue(createMockWizard());
});

afterEach(() => {
  cleanup();
  document.body.style.overflow = '';
});

/* ================================================================
   Tests
   ================================================================ */

describe('AgentDeployModal', () => {
  /* ── Rendering ─────────────────────────────────────────────── */

  describe('rendering', () => {
    it('renders the dialog with cluster name in the title', () => {
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Deploy Agents — My Cluster')).toBeInTheDocument();
    });

    it('sets aria-label on the dialog with cluster name', () => {
      render(() => <AgentDeployModal {...defaultProps()} />);

      const dialog = screen.getByRole('dialog');
      expect(dialog).toHaveAttribute('aria-label', 'Deploy Agents — My Cluster');
    });

    it('renders the StepIndicator with step labels', () => {
      render(() => <AgentDeployModal {...defaultProps()} />);

      const indicator = screen.getByTestId('step-indicator');
      expect(indicator).toHaveTextContent('Select Hosts | Preflight | Confirm | Deploy | Results');
    });

    it('has a close X button with aria-label', () => {
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByLabelText('Close')).toBeInTheDocument();
    });

    it('does not render when isOpen is false', () => {
      const props = defaultProps();
      props.isOpen = false;
      render(() => <AgentDeployModal {...props} />);

      expect(screen.queryByText('Deploy Agents — My Cluster')).not.toBeInTheDocument();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(document.querySelector('[data-dialog-backdrop]')).toBeNull();
    });
  });

  /* ── Step routing ──────────────────────────────────────────── */

  describe('step routing', () => {
    const allStepTestIds = [
      'candidates-step',
      'preflight-step',
      'confirm-step',
      'deploying-step',
      'results-step',
    ];

    function expectOnlyStep(activeTestId: string) {
      expect(screen.getByTestId(activeTestId)).toBeInTheDocument();
      for (const id of allStepTestIds) {
        if (id !== activeTestId) {
          expect(screen.queryByTestId(id)).not.toBeInTheDocument();
        }
      }
    }

    it('shows only CandidatesStep when step is candidates', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'candidates' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expectOnlyStep('candidates-step');
    });

    it('shows only PreflightStep when step is preflight', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'preflight' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expectOnlyStep('preflight-step');
    });

    it('shows only ConfirmStep when step is confirm', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'confirm' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expectOnlyStep('confirm-step');
    });

    it('shows only DeployingStep when step is deploying', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'deploying' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expectOnlyStep('deploying-step');
    });

    it('shows only ResultsStep when step is results', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'results' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expectOnlyStep('results-step');
    });
  });

  /* ── Step indicator index ──────────────────────────────────── */

  describe('step indicator index', () => {
    const cases: [WizardStep, number][] = [
      ['candidates', 0],
      ['preflight', 1],
      ['confirm', 2],
      ['deploying', 3],
      ['results', 4],
    ];

    it.each(cases)('maps wizard step "%s" to indicator index %d', (wizardStep, expectedIndex) => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: wizardStep }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      const indicator = screen.getByTestId('step-indicator');
      expect(indicator.dataset.currentStep).toBe(String(expectedIndex));
    });
  });

  /* ── Candidates step footer ────────────────────────────────── */

  describe('candidates step footer', () => {
    it('shows Cancel and Run Preflight buttons', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'candidates' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Cancel')).toBeInTheDocument();
      expect(screen.getByText('Run Preflight')).toBeInTheDocument();
    });

    it('Cancel calls onClose', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'candidates' }));
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      fireEvent.click(screen.getByText('Cancel'));
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });

    it('disables Run Preflight when no nodes selected', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'candidates',
          selectedNodeIds: new Set<string>(),
          selectedSourceAgent: 'agent-1',
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Run Preflight')).toBeDisabled();
    });

    it('disables Run Preflight when no source agent selected', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'candidates',
          selectedNodeIds: new Set(['node-1']),
          selectedSourceAgent: '',
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Run Preflight')).toBeDisabled();
    });

    it('enables Run Preflight when nodes and source agent are selected', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'candidates',
          selectedNodeIds: new Set(['node-1']),
          selectedSourceAgent: 'agent-1',
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Run Preflight')).not.toBeDisabled();
    });

    it('disables and shows Starting... when startingPreflight is true', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'candidates',
          selectedNodeIds: new Set(['node-1']),
          selectedSourceAgent: 'agent-1',
          startingPreflight: true,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Starting...')).toBeInTheDocument();
      // The button containing "Starting..." should be disabled.
      const btn = screen.getByText('Starting...').closest('button');
      expect(btn).toBeDisabled();
    });

    it('calls startPreflight when Run Preflight is clicked', () => {
      const startPreflightMock = vi.fn();
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'candidates',
          selectedNodeIds: new Set(['node-1']),
          selectedSourceAgent: 'agent-1',
          startPreflight: startPreflightMock,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Run Preflight'));
      expect(startPreflightMock).toHaveBeenCalledTimes(1);
    });
  });

  /* ── Preflight step footer ─────────────────────────────────── */

  describe('preflight step footer', () => {
    it('shows "Preflight in progress..." message', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'preflight' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Preflight in progress...')).toBeInTheDocument();
    });
  });

  /* ── Confirm step footer ───────────────────────────────────── */

  describe('confirm step footer', () => {
    it('shows Back button that returns to candidates step', () => {
      const wizard = createMockWizard({ step: 'confirm' });
      useDeployWizardMock.mockReturnValue(wizard);
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Back')).toBeInTheDocument();
      fireEvent.click(screen.getByText('Back'));
      expect(wizard.step()).toBe('candidates');
    });

    it('shows Deploy button with confirmed node count', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'confirm',
          confirmSelectedNodeIds: new Set(['n1', 'n2', 'n3']),
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Deploy 3 Hosts')).toBeInTheDocument();
    });

    it('uses singular "Node" when exactly 1 confirmed node', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'confirm',
          confirmSelectedNodeIds: new Set(['n1']),
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Deploy 1 Host')).toBeInTheDocument();
    });

    it('disables Deploy button when no confirmed nodes', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'confirm',
          confirmSelectedNodeIds: new Set<string>(),
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Deploy 0 Hosts')).toBeDisabled();
    });

    it('disables and shows Starting... when startingDeploy is true', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'confirm',
          confirmSelectedNodeIds: new Set(['n1']),
          startingDeploy: true,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Starting...')).toBeInTheDocument();
      const btn = screen.getByText('Starting...').closest('button');
      expect(btn).toBeDisabled();
    });

    it('calls startDeploy when Deploy button is clicked', () => {
      const startDeployMock = vi.fn();
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'confirm',
          confirmSelectedNodeIds: new Set(['n1']),
          startDeploy: startDeployMock,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Deploy 1 Host'));
      expect(startDeployMock).toHaveBeenCalledTimes(1);
    });
  });

  /* ── Deploying step footer ─────────────────────────────────── */

  describe('deploying step footer', () => {
    it('shows Cancel Deployment button', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'deploying' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Cancel Deployment')).toBeInTheDocument();
    });

    it('calls cancelDeploy when Cancel Deployment is clicked', () => {
      const cancelDeployMock = vi.fn();
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'deploying',
          cancelDeploy: cancelDeployMock,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Cancel Deployment'));
      expect(cancelDeployMock).toHaveBeenCalledTimes(1);
    });

    it('shows Canceling... when canceling is true', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'deploying',
          canceling: true,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Canceling...')).toBeInTheDocument();
      const btn = screen.getByText('Canceling...').closest('button');
      expect(btn).toBeDisabled();
    });

    it('shows warning about closing during active operation', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'deploying',
          isOperationActive: () => true,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText("Closing won't cancel the deployment.")).toBeInTheDocument();
    });

    it('hides close-warning when operation is not active', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'deploying',
          isOperationActive: () => false,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.queryByText("Closing won't cancel the deployment.")).not.toBeInTheDocument();
    });
  });

  /* ── Results step footer ───────────────────────────────────── */

  describe('results step footer', () => {
    it('shows Close button', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'results' }));
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Close')).toBeInTheDocument();
    });

    it('Close button calls onClose', () => {
      useDeployWizardMock.mockReturnValue(createMockWizard({ step: 'results' }));
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      fireEvent.click(screen.getByText('Close'));
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });

    it('shows Retry button when retryable targets exist', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'results',
          retryableTargets: [{ id: 't1' }, { id: 't2' }],
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Retry 2 Failed')).toBeInTheDocument();
    });

    it('hides Retry button when no retryable targets', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'results',
          retryableTargets: [],
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.queryByText(/Retry/)).not.toBeInTheDocument();
    });

    it('calls retryFailed when Retry button is clicked', () => {
      const retryFailedMock = vi.fn();
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'results',
          retryableTargets: [{ id: 't1' }],
          retryFailed: retryFailedMock,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      fireEvent.click(screen.getByText('Retry 1 Failed'));
      expect(retryFailedMock).toHaveBeenCalledTimes(1);
    });

    it('disables and shows Retrying... when retrying is true', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'results',
          retryableTargets: [{ id: 't1' }],
          retrying: true,
        }),
      );
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(screen.getByText('Retrying...')).toBeInTheDocument();
      const btn = screen.getByText('Retrying...').closest('button');
      expect(btn).toBeDisabled();
    });
  });

  /* ── Close behavior ────────────────────────────────────────── */

  describe('close behavior', () => {
    it('X button calls onClose', () => {
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      fireEvent.click(screen.getByLabelText('Close'));
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });

    it('X button still calls onClose when operation is active', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'deploying',
          isOperationActive: () => true,
        }),
      );
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      fireEvent.click(screen.getByLabelText('Close'));
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });

    it('disables backdrop close when operation is active', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'deploying',
          isOperationActive: () => true,
        }),
      );
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      // Click on backdrop — the Dialog component receives closeOnBackdrop=false.
      // We verify this by clicking the backdrop and seeing that onClose is NOT called.
      const backdrop = document.querySelector('[data-dialog-backdrop]');
      expect(backdrop).not.toBeNull();
      fireEvent.click(backdrop!);
      expect(props.onClose).not.toHaveBeenCalled();
    });

    it('allows backdrop close when operation is not active', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'candidates',
          isOperationActive: () => false,
        }),
      );
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      const backdrop = document.querySelector('[data-dialog-backdrop]');
      expect(backdrop).not.toBeNull();
      fireEvent.click(backdrop!);
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });
  });

  /* ── Wizard initialization ─────────────────────────────────── */

  describe('wizard initialization', () => {
    it('passes clusterId and clusterName to useDeployWizard', () => {
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(useDeployWizardMock).toHaveBeenCalledWith({
        clusterId: 'cluster-1',
        clusterName: 'My Cluster',
      });
    });
  });

  /* ── Keyboard close (Escape) ───────────────────────────────── */

  describe('keyboard close', () => {
    it('calls onClose when Escape key is pressed', () => {
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      fireEvent.keyDown(document, { key: 'Escape' });
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose on Escape even when operation is active', () => {
      useDeployWizardMock.mockReturnValue(
        createMockWizard({
          step: 'deploying',
          isOperationActive: () => true,
        }),
      );
      const props = defaultProps();
      render(() => <AgentDeployModal {...props} />);

      fireEvent.keyDown(document, { key: 'Escape' });
      expect(props.onClose).toHaveBeenCalledTimes(1);
    });
  });

  /* ── ConfirmStep prop wiring ───────────────────────────────── */

  describe('ConfirmStep prop wiring', () => {
    it('passes wizard to ConfirmStep', () => {
      const wizard = createMockWizard({ step: 'confirm' });
      useDeployWizardMock.mockReturnValue(wizard);
      render(() => <AgentDeployModal {...defaultProps()} />);

      expect(confirmStepPropsSpy).toHaveBeenCalled();
      const receivedProps = confirmStepPropsSpy.mock.calls[0][0];
      expect(receivedProps.wizard).toBe(wizard);
    });
  });
});
