import type { AgentWorkflowPrompt } from '@/api/agentCapabilities';
import type { AIChatContext } from '@/stores/aiChat';

export type AssistantWorkflowStarterKind = 'fleet' | 'resource' | 'finding' | 'workflow';

export interface AssistantWorkflowStarter {
  id: string;
  name: string;
  label: string;
  description?: string;
  kind: AssistantWorkflowStarterKind;
  arguments: Record<string, string>;
}

const WORKFLOW_STARTER_KINDS = new Set<AssistantWorkflowStarterKind>([
  'fleet',
  'resource',
  'finding',
  'workflow',
]);

const PATROL_TARGET_TYPES = new Set(['patrol-run', 'patrol-assessment', 'patrol-configuration']);

const normalizeIdentifier = (value: unknown): string => {
  if (typeof value !== 'string') return '';
  return value.trim();
};

const workflowStarterLabel = (prompt: AgentWorkflowPrompt): string => {
  const label = normalizeIdentifier(prompt.label);
  if (label) return label;

  const name = normalizeIdentifier(prompt.name);
  const description = normalizeIdentifier(prompt.description);
  if (description) return description;

  return name
    .replace(/^pulse_/, '')
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
};

const workflowStarterKind = (prompt: AgentWorkflowPrompt): AssistantWorkflowStarterKind => {
  const kind = normalizeIdentifier(prompt.presentationKind);
  if (WORKFLOW_STARTER_KINDS.has(kind as AssistantWorkflowStarterKind)) {
    return kind as AssistantWorkflowStarterKind;
  }
  return 'workflow';
};

export const getAssistantWorkflowStarterResourceId = (context: AIChatContext): string => {
  for (const resource of context.handoffResources ?? []) {
    const id = normalizeIdentifier(resource.id);
    if (id) return id;
  }

  for (const action of context.handoffActions ?? []) {
    const id = normalizeIdentifier(action.targetResourceId);
    if (id) return id;
  }

  const targetId = normalizeIdentifier(context.targetId);
  const targetType = normalizeIdentifier(context.targetType);
  if (targetId && !PATROL_TARGET_TYPES.has(targetType)) {
    return targetId;
  }

  return '';
};

export const getAssistantWorkflowStarterFindingId = (context: AIChatContext): string => {
  const direct = normalizeIdentifier(context.findingId);
  if (direct) return direct;

  for (const action of context.handoffActions ?? []) {
    const id = normalizeIdentifier(action.findingId);
    if (id) return id;
  }

  return normalizeIdentifier(context.context?.findingId);
};

const resolveWorkflowStarterArgument = (
  name: string,
  context: AIChatContext,
): string | undefined => {
  switch (name) {
    case 'resourceId':
      return getAssistantWorkflowStarterResourceId(context);
    case 'finding_id':
      return getAssistantWorkflowStarterFindingId(context);
    default:
      return undefined;
  }
};

const buildWorkflowStarterArguments = (
  prompt: AgentWorkflowPrompt,
  context: AIChatContext,
): Record<string, string> | null => {
  const args: Record<string, string> = {};
  for (const argument of prompt.arguments ?? []) {
    const name = normalizeIdentifier(argument.name);
    if (!name) continue;

    const value = resolveWorkflowStarterArgument(name, context);
    if (value) {
      args[name] = value;
      continue;
    }

    if (argument.required) return null;
  }
  return args;
};

export const getAssistantWorkflowStarters = (
  prompts: AgentWorkflowPrompt[],
  context: AIChatContext,
): AssistantWorkflowStarter[] => {
  const starters: AssistantWorkflowStarter[] = [];
  const seen = new Set<string>();

  for (const prompt of prompts) {
    const name = normalizeIdentifier(prompt.name);
    if (!name || seen.has(name)) continue;

    const args = buildWorkflowStarterArguments(prompt, context);
    if (!args) continue;

    seen.add(name);
    const description = normalizeIdentifier(prompt.description);
    starters.push({
      id: name,
      name,
      label: workflowStarterLabel(prompt),
      description: description || undefined,
      kind: workflowStarterKind(prompt),
      arguments: args,
    });
  }

  return starters;
};
