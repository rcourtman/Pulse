import type { Agent, Node } from '@/types/api';
import {
  getInfrastructureDiscoveryHostname,
  getInfrastructureMetadataId,
} from '@/utils/resourceIdentity';

export interface InfrastructureDetailsDrawerProps {
  node: Node;
  agent?: Agent;
  customUrl?: string;
  onCustomUrlChange?: (agentId: string, url: string) => void;
}

export const resolveInfrastructureDetailsDrawerMetadataId = (node: Node, agent?: Agent) =>
  getInfrastructureMetadataId(node, agent);

export const resolveInfrastructureDetailsDrawerDiscoveryHostname = (
  node: Node,
  agent?: Agent,
) => getInfrastructureDiscoveryHostname(node, agent);
