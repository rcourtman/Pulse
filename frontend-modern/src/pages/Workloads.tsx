import { Dashboard as WorkloadsSurface } from '@/components/Dashboard/Dashboard';

export function Workloads() {
  return <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />;
}

export default Workloads;
