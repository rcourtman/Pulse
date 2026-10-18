import { describe, expect, it } from 'vitest';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import {
  buildGuestParentNodeMap,
  buildNodeByInstance,
  workloadNodeScopeId,
} from '../workloadTopology';

const node = (id: string, total: number, status: string): Node =>
  ({ id, instance: 'lab-east', name: 'pve1', memory: { total }, status }) as Node;
const first = node('native-a', 8 * 1024 ** 3, 'offline');
const second = node('native-c', 64 * 1024 ** 3, 'online');
const guest = (fields: Partial<WorkloadGuest> = {}): WorkloadGuest =>
  ({
    id: 'native-a-101',
    vmid: 101,
    instance: 'lab-east',
    node: 'pve1',
    type: 'qemu',
    workloadType: 'vm',
    platformScopes: ['proxmox-pve'],
    ...fields,
  }) as WorkloadGuest;
const shapes: Array<{ label: string; fields: Partial<WorkloadGuest> }> = [
  { label: 'complete fields', fields: {} },
  { label: 'missing node', fields: { node: '' } },
  { label: 'missing instance', fields: { instance: '' } },
  { label: 'missing both', fields: { instance: '', node: '' } },
  { label: 'whitespace fields', fields: { instance: ' ', node: '\t' } },
  { label: 'absent fields', fields: { instance: undefined, node: undefined } },
];

describe('Workloads ambiguous parent fallback', () => {
  for (const order of ['forward', 'reversed'] as const) {
    const inventory = order === 'forward' ? [first, second] : [second, first];
    describe(`${order} inventory`, () => {
      it.each(
        shapes.flatMap(({ label, fields }) =>
          ['native-a-101', 'native-c-101'].flatMap((id) =>
            ['qemu', 'lxc'].map((type) => ({ label, fields, id, type })),
          ),
        ),
      )('cannot restore an ambiguous tuple: $label / $id / $type', ({ fields, id, type }) => {
        const map = buildNodeByInstance(inventory);
        expect(map[workloadNodeScopeId({ instance: 'lab-east', node: 'pve1' })]).toBeUndefined();
        // Both IDs are unique; neither supplies evidence that the tuple is unique.
        expect(map['native-a']).toBe(first);
        expect(map['native-c']).toBe(second);
        const row = guest({
          ...fields,
          id,
          type,
          workloadType: type === 'lxc' ? 'system-container' : 'vm',
        });
        expect(buildGuestParentNodeMap([row], map)[getCanonicalWorkloadId(row)]).toBeUndefined();
      });

      it.each(shapes)(
        'retains consistent fallback when the parent tuple is unique: $label',
        ({ fields }) => {
          const other = { ...second, instance: 'elsewhere' };
          const unique = order === 'forward' ? [first, other] : [other, first];
          const row = guest(fields);
          expect(
            buildGuestParentNodeMap([row], buildNodeByInstance(unique))[
              getCanonicalWorkloadId(row)
            ],
          ).toBe(first);
        },
      );

      it('does not revive a duplicate tuple after a third snapshot', () => {
        const third = node('native-d', 16 * 1024 ** 3, 'online');
        const row = guest({ instance: '', node: '', id: 'native-d-101' });
        expect(
          buildGuestParentNodeMap([row], buildNodeByInstance([...inventory, third]))[
            getCanonicalWorkloadId(row)
          ],
        ).toBeUndefined();
      });
    });
  }
});
