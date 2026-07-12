import { describe, expect, it } from 'vitest';

import type { Resource, ResourceStatus } from '@/types/resource';
import {
  dockerContainerLifecycleName,
  getDockerContainerLifecycleDisabledReason,
} from '../dockerContainerLifecycleActions';

const ALL_CAPABILITIES = [
  { name: 'start', type: 'common', platform: 'docker', minimumApprovalLevel: 'admin' },
  { name: 'stop', type: 'common', platform: 'docker', minimumApprovalLevel: 'admin' },
  { name: 'restart', type: 'common', platform: 'docker', minimumApprovalLevel: 'admin' },
];

const resource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'app-container:docker-host:web',
  name: 'web',
  displayName: 'web',
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'running',
  type: 'app-container',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    agentId: 'agent-1',
    containerId: 'abc123',
    containerState: 'running',
  },
  capabilities: ALL_CAPABILITIES,
  ...overrides,
});

describe('dockerContainerLifecycleActions.branchcov2', () => {
  describe('dockerContainerLifecycleName', () => {
    it('returns the trimmed resource name when present', () => {
      expect(dockerContainerLifecycleName(resource({ name: '  web-app  ' }))).toBe('web-app');
    });

    it('falls back to displayName when name is blank', () => {
      expect(
        dockerContainerLifecycleName(resource({ name: '   ', displayName: 'Web App' })),
      ).toBe('Web App');
    });

    it('falls back to docker.displayName when name and displayName are blank', () => {
      expect(
        dockerContainerLifecycleName(
          resource({
            name: '',
            displayName: '  ',
            docker: { runtime: 'docker', displayName: 'Container Display' },
          }),
        ),
      ).toBe('Container Display');
    });

    it('falls back to docker.containerId when name, displayName, and docker.displayName are blank', () => {
      expect(
        dockerContainerLifecycleName(
          resource({
            name: '',
            displayName: '',
            docker: { runtime: 'docker', containerId: 'cid-9' },
          }),
        ),
      ).toBe('cid-9');
    });

    it('falls back to resource.id when every name source is blank', () => {
      expect(
        dockerContainerLifecycleName(
          resource({
            name: '',
            displayName: '',
            docker: { runtime: 'docker' },
          }),
        ),
      ).toBe('app-container:docker-host:web');
    });
  });

  describe('getDockerContainerLifecycleDisabledReason', () => {
    describe('runtime gating', () => {
      it('reports an unsupported runtime using the runtime label', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({ docker: { runtime: 'containerd', agentId: 'agent-1' } }),
            'restart',
          ),
        ).toBe('containerd is not supported for governed container lifecycle actions.');
      });

      it('treats podman as supported and reaches the agent check with a Podman label', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({ docker: { runtime: 'podman' } }),
            'restart',
          ),
        ).toBe('No reporting Pulse agent is attached to this Podman host.');
      });

      it('reports a missing runtime when docker is absent entirely', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(resource({ docker: undefined }), 'restart'),
        ).toBe('Container runtime is not reported.');
      });

      it('reports a missing runtime when the runtime token is whitespace-only', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              docker: { runtime: '   ', agentId: 'agent-1', containerState: 'running' },
            }),
            'restart',
          ),
        ).toBe('Container runtime is not reported.');
      });
    });

    describe('sourceStatusDisabledReason (exercised via orchestrator)', () => {
      it('blocks on an offline inventory status', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({ sourceStatus: { docker: { status: 'offline' } } }),
            'restart',
          ),
        ).toBe('Docker inventory is offline; refresh inventory before running lifecycle actions.');
      });

      it('blocks on a missing inventory status', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({ sourceStatus: { docker: { status: 'missing' } } }),
            'restart',
          ),
        ).toBe('Docker inventory is missing; refresh inventory before running lifecycle actions.');
      });

      it('blocks on an inventory error when the status itself is healthy', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              sourceStatus: { docker: { status: 'healthy', error: 'connection refused' } },
            }),
            'restart',
          ),
        ).toBe('Docker inventory is not healthy: connection refused');
      });

      it('blocks when lastSeen is zero (falsy timestamp)', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(resource({ lastSeen: 0 }), 'restart'),
        ).toBe('Docker inventory has not reported a valid last-seen timestamp.');
      });

      it('blocks when lastSeen is negative (truthy but <= 0)', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(resource({ lastSeen: -5 }), 'restart'),
        ).toBe('Docker inventory has not reported a valid last-seen timestamp.');
      });
    });

    describe('security gating', () => {
      it('uses the default host-policy copy when mutatingCommandsBlockedReason is absent', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc123',
                containerState: 'running',
                security: { mutatingCommandsBlocked: true },
              },
            }),
            'restart',
          ),
        ).toBe('Docker host policy blocks mutating container lifecycle commands.');
      });
    });

    describe('stateDisabledReason (exercised via orchestrator)', () => {
      it('reports a non-startable, non-running state for start', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc',
                containerState: 'paused',
              },
            }),
            'start',
          ),
        ).toBe('Container state paused is not startable.');
      });

      it('prefers containerState over status for the state lookup', () => {
        // status 'stopped' would be startable; containerState 'paused' is not -> proves containerState wins.
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              status: 'stopped',
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc',
                containerState: 'paused',
              },
            }),
            'start',
          ),
        ).toBe('Container state paused is not startable.');
      });

      it('falls back to status when containerState is blank', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              status: 'paused',
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc',
                containerState: '',
              },
            }),
            'start',
          ),
        ).toBe('Container state paused is not startable.');
      });

      it('reports an unknown state for start when both containerState and status are blank', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              status: '' as unknown as ResourceStatus,
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc',
                containerState: '',
              },
            }),
            'start',
          ),
        ).toBe('Container state is unknown.');
      });

      it('reports that the container must be running before stop', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc',
                containerState: 'exited',
              },
            }),
            'stop',
          ),
        ).toBe('Container must be running before stop.');
      });

      it('reports an unknown state for stop when both containerState and status are blank', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              status: '' as unknown as ResourceStatus,
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc',
                containerState: '',
              },
            }),
            'stop',
          ),
        ).toBe('Container state is unknown.');
      });

      it('does not block start for a startable state (created)', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              status: 'stopped',
              docker: {
                runtime: 'docker',
                agentId: 'agent-1',
                containerId: 'abc',
                containerState: 'created',
              },
            }),
            'start',
          ),
        ).toBeUndefined();
      });
    });

    describe('actionReadinessDisabledReason (exercised via orchestrator)', () => {
      it('prefers an explicit reason over the reasonCode switch', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                {
                  name: 'restart',
                  available: false,
                  reasonCode: 'command_agent_disconnected',
                  reason: 'Agent reboot in progress',
                },
              ],
            }),
            'restart',
          ),
        ).toBe('Agent reboot in progress');
      });

      it('maps command_agent_disconnected to the not-connected copy', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'restart', available: false, reasonCode: 'command_agent_disconnected' },
              ],
            }),
            'restart',
          ),
        ).toBe('Docker / Podman command agent is not connected.');
      });

      it('maps command_agent_unavailable to the execution-unavailable copy', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'restart', available: false, reasonCode: 'command_agent_unavailable' },
              ],
            }),
            'restart',
          ),
        ).toBe('Docker / Podman command execution is not available.');
      });

      it('maps stale_inventory to the not-fresh copy', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'restart', available: false, reasonCode: 'stale_inventory' },
              ],
            }),
            'restart',
          ),
        ).toBe('Docker / Podman inventory is not fresh enough to run lifecycle actions.');
      });

      it('maps host_policy_blocked to the policy-blocked copy', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'restart', available: false, reasonCode: 'host_policy_blocked' },
              ],
            }),
            'restart',
          ),
        ).toBe('Docker / Podman host policy blocks mutating lifecycle actions.');
      });

      it('maps unsupported_handler to the unsupported-executor copy', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'restart', available: false, reasonCode: 'unsupported_handler' },
              ],
            }),
            'restart',
          ),
        ).toBe('This container action is not routed through the supported lifecycle executor.');
      });

      it('falls through the default switch arm without blocking when a capability is advertised', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'restart', available: false, reasonCode: 'totally_unknown_code' },
              ],
            }),
            'restart',
          ),
        ).toBeUndefined();
      });

      it('does not block when the matched readiness item is still available', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'restart', available: true, reasonCode: 'command_agent_disconnected' },
              ],
            }),
            'restart',
          ),
        ).toBeUndefined();
      });

      it('does not block when readiness is reported for a different action', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'start', available: false, reasonCode: 'command_agent_disconnected' },
              ],
            }),
            'restart',
          ),
        ).toBeUndefined();
      });

      it('matches readiness names case-insensitively', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(
            resource({
              actionReadiness: [
                { name: 'RESTART', available: false, reasonCode: 'stale_inventory' },
              ],
            }),
            'restart',
          ),
        ).toBe('Docker / Podman inventory is not fresh enough to run lifecycle actions.');
      });
    });

    describe('capability gating', () => {
      it('reports the missing capability using the requested action name', () => {
        expect(
          getDockerContainerLifecycleDisabledReason(resource({ capabilities: [] }), 'stop'),
        ).toBe(
          'Pulse does not currently advertise a fresh stop command capability for this container.',
        );
      });
    });

    describe('happy path', () => {
      it('returns undefined for stop and restart on a fully healthy running container', () => {
        const healthy = resource();

        expect(getDockerContainerLifecycleDisabledReason(healthy, 'stop')).toBeUndefined();
        expect(getDockerContainerLifecycleDisabledReason(healthy, 'restart')).toBeUndefined();
      });

      it('returns undefined for start on a stopped container with healthy inventory', () => {
        const stopped = resource({
          status: 'offline',
          docker: {
            runtime: 'docker',
            agentId: 'agent-1',
            containerId: 'abc',
            containerState: 'stopped',
          },
        });

        expect(getDockerContainerLifecycleDisabledReason(stopped, 'start')).toBeUndefined();
      });
    });
  });
});
