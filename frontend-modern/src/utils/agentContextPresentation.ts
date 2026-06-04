import type {
  AgentResourceContext,
  AgentResourceContextFact,
  AgentResourceContextRedaction,
  AgentResourceContextSection,
} from '@/api/agentContext';

const formatOptionalTime = (value?: string): string => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toISOString();
};

const formatProvenance = (fact: AgentResourceContextFact): string => {
  const parts = [
    fact.source ? `source=${fact.source}` : '',
    fact.trustTier ? `trust=${fact.trustTier}` : '',
    fact.observedAt ? `observed=${formatOptionalTime(fact.observedAt)}` : '',
    fact.redacted ? 'redacted=true' : '',
  ].filter(Boolean);
  return parts.length > 0 ? ` (${parts.join(', ')})` : '';
};

const formatFact = (fact: AgentResourceContextFact): string =>
  `- ${fact.label}: ${fact.value}${formatProvenance(fact)}`;

const formatRedaction = (redaction: AgentResourceContextRedaction): string =>
  `- Redaction: ${redaction.field}${redaction.reason ? ` - ${redaction.reason}` : ''}`;

const formatSectionHeader = (section: AgentResourceContextSection): string => {
  const provenance = [
    section.source ? `source=${section.source}` : '',
    section.trustTier ? `trust=${section.trustTier}` : '',
    section.observedAt ? `observed=${formatOptionalTime(section.observedAt)}` : '',
  ].filter(Boolean);
  return provenance.length > 0
    ? `## ${section.title} (${provenance.join(', ')})`
    : `## ${section.title}`;
};

export const formatAgentResourceContextForClipboard = (context: AgentResourceContext): string => {
  const lines = [
    `# Pulse resource context: ${context.resourceName || context.canonicalId}`,
    '',
    `Generated: ${formatOptionalTime(context.generatedAt)}`,
    `Canonical ID: ${context.canonicalId}`,
    `Resource type: ${context.resourceType}`,
  ];

  if (context.technology) {
    lines.push(`Technology: ${context.technology}`);
  }

  lines.push(
    `Active findings: ${context.activeFindings.length}`,
    `Pending approvals: ${context.pendingApprovals.length}`,
    `Recent actions: ${context.recentActions.length}`,
    '',
    'Context facts below are bounded, read-only Pulse context. Redacted values were withheld by policy.',
  );

  for (const section of context.contextSections) {
    if (section.facts.length === 0 && !section.redactions?.length) continue;
    lines.push('', formatSectionHeader(section));
    if (section.summary) {
      lines.push(section.summary);
    }
    for (const fact of section.facts) {
      lines.push(formatFact(fact));
    }
    for (const redaction of section.redactions ?? []) {
      lines.push(formatRedaction(redaction));
    }
  }

  return `${lines.join('\n').trim()}\n`;
};
