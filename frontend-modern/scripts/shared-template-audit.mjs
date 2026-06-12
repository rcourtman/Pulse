import { readFileSync } from 'node:fs';
import { relative, resolve } from 'node:path';

const root = resolve(new URL('..', import.meta.url).pathname);

const read = (path) => readFileSync(resolve(root, path), 'utf8');

const rules = [
  {
    id: 'runtime-web-interface-name-link',
    summary:
      'Runtime resource rows must launch saved web-interface URLs through the shared name-link template, not a page-local link or separate web column.',
    canonical: 'src/components/shared/WebInterfaceNameLink.tsx',
    requiredConsumers: [
      'src/components/Workloads/GuestRow.tsx',
      'src/features/standalone/AgentsMachinesTable.tsx',
    ],
    forbiddenPatterns: [
      {
        path: 'src/components/Workloads/GuestRow.tsx',
        patterns: ['target="_blank"', 'rel="noopener noreferrer"'],
      },
      {
        path: 'src/features/standalone/AgentsMachinesTable.tsx',
        patterns: [
          'target="_blank"',
          'rel="noopener noreferrer"',
          'AgentMachineWebLinkCell',
          'data-agent-machine-web-link',
        ],
      },
      {
        path: 'src/features/standalone/agentMachineTableModel.ts',
        patterns: ["id: 'web'", "label: 'Web'"],
      },
    ],
  },
];

const failures = [];

for (const rule of rules) {
  const canonicalSource = read(rule.canonical);
  const canonicalExport = rule.canonical
    .split('/')
    .at(-1)
    ?.replace(/\.[^.]+$/, '');
  if (canonicalExport && !canonicalSource.includes(canonicalExport)) {
    failures.push(`${rule.id}: canonical template ${rule.canonical} does not export itself`);
  }

  for (const consumer of rule.requiredConsumers) {
    const source = read(consumer);
    if (canonicalExport && !source.includes(canonicalExport)) {
      failures.push(`${rule.id}: ${consumer} must compose ${canonicalExport}`);
    }
  }

  for (const check of rule.forbiddenPatterns) {
    const source = read(check.path);
    for (const pattern of check.patterns) {
      if (source.includes(pattern)) {
        failures.push(`${rule.id}: ${check.path} must not contain ${JSON.stringify(pattern)}`);
      }
    }
  }
}

if (failures.length > 0) {
  console.error('Shared template audit failed:');
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log(
  `Shared template audit passed (${rules.length} rule${rules.length === 1 ? '' : 's'} from ${relative(
    process.cwd(),
    import.meta.url.replace('file://', ''),
  )})`,
);
