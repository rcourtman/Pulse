#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SCRIPT_PATH = fileURLToPath(import.meta.url);
const SCRIPT_DIR = path.dirname(SCRIPT_PATH);
const DEFAULT_FRONTEND_ROOT = path.resolve(SCRIPT_DIR, '..');
const DEFAULT_REPO_ROOT = path.resolve(DEFAULT_FRONTEND_ROOT, '..');

const INTERNAL_ANALYTICS_RULES = [
  {
    getFilePath: ({ repoRoot }) => path.join(repoRoot, 'internal', 'api', 'diagnostics.go'),
    rules: [
      {
        rule: 'canonical-settings/no-internal-analytics-in-diagnostics-api',
        regex:
          /CommercialFunnel|commercialFunnel|Commercial Funnel|Sales Funnel|InfrastructureOnboarding|infrastructureOnboarding|Infrastructure Onboarding|pricing_viewed|checkout_clicked|credentials_opened/g,
        message:
          'Do not expose maintainer/admin analytics from /api/diagnostics. Keep commercial and onboarding analytics on admin-owned metrics routes.',
      },
    ],
  },
  {
    getFilePath: ({ root }) =>
      path.join(root, 'src', 'components', 'Settings', 'DiagnosticsResultsPanel.tsx'),
    rules: [
      {
        rule: 'canonical-settings/no-internal-analytics-in-diagnostics-ui',
        regex:
          /CommercialFunnel|commercialFunnel|Commercial Funnel|Sales Funnel|InfrastructureOnboarding|infrastructureOnboarding|Infrastructure Onboarding|Pricing Views|Checkout Clicks|Credentials Opened|pricing_viewed|checkout_clicked|credentials_opened/g,
        message:
          'Do not render maintainer/admin analytics in the user-facing Settings diagnostics panel.',
      },
    ],
  },
  {
    getFilePath: ({ root }) =>
      path.join(root, 'src', 'components', 'Settings', 'diagnosticsModel.ts'),
    rules: [
      {
        rule: 'canonical-settings/no-internal-analytics-diagnostics-types',
        regex:
          /\bexport interface (?:CommercialFunnel|InfrastructureOnboarding)\w*\b|\b(?:commercialFunnel|infrastructureOnboarding)\?:|\b(?:CommercialFunnel|InfrastructureOnboarding)(?:Diagnostic|Summary|StageCounts|DayBreakdown|DimensionBreakdown|PathBreakdown|PlatformBreakdown)\b/g,
        message:
          'Do not add commercial funnel or infrastructure onboarding fields to the customer diagnostics payload model.',
      },
    ],
  },
  {
    getFilePath: ({ root }) => path.join(root, 'src', 'utils', 'upgradeMetrics.ts'),
    rules: [
      {
        rule: 'canonical-settings/no-product-upgrade-metrics-ingestion',
        regex:
          /\/api\/upgrade-metrics\/events|@\/utils\/apiClient|\bapiFetch\s*\(|\bfetch\s*\(|\bsendBeacon\b/g,
        message:
          'Do not emit maintainer/admin commercial analytics from the customer frontend. Keep upgrade-metrics ingestion out of product surfaces.',
      },
    ],
  },
  {
    getFilePaths: ({ root }) => listProductionSourceFiles(path.join(root, 'src')),
    rules: [
      {
        rule: 'canonical-settings/no-product-upgrade-metrics-endpoint',
        regex: /\/api\/upgrade-metrics\/events/g,
        message:
          'Do not call local commercial analytics ingestion from production customer frontend source.',
      },
    ],
  },
];

function isProductionSourceFile(filePath) {
  if (!/\.(?:ts|tsx)$/.test(filePath)) return false;
  if (filePath.includes(`${path.sep}__tests__${path.sep}`)) return false;
  if (/\.(?:test|spec)\.(?:ts|tsx)$/.test(filePath)) return false;
  return true;
}

function listProductionSourceFiles(dir) {
  const files = [];
  if (!fs.existsSync(dir)) return files;

  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...listProductionSourceFiles(entryPath));
      continue;
    }

    if (entry.isFile() && isProductionSourceFile(entryPath)) {
      files.push(entryPath);
    }
  }

  return files;
}

function lineForIndex(content, index) {
  let line = 1;
  for (let i = 0; i < index; i += 1) {
    if (content[i] === '\n') line += 1;
  }
  return line;
}

function relativeToRoot(root, absPath) {
  return path.relative(root, absPath).replaceAll(path.sep, '/');
}

export function collectUserDiagnosticsInternalAnalyticsFindings({
  root = DEFAULT_FRONTEND_ROOT,
  repoRoot = DEFAULT_REPO_ROOT,
} = {}) {
  const findings = [];

  for (const { getFilePath, getFilePaths, rules } of INTERNAL_ANALYTICS_RULES) {
    const filePaths = getFilePaths?.({ root, repoRoot }) ?? [getFilePath({ root, repoRoot })];

    for (const filePath of filePaths) {
      const content = fs.readFileSync(filePath, 'utf8');
      const relativePath = relativeToRoot(root, filePath);

      for (const { rule, regex, message } of rules) {
        for (const match of content.matchAll(regex)) {
          findings.push({
            file: relativePath,
            line: lineForIndex(content, match.index ?? 0),
            rule,
            message,
          });
        }
      }
    }
  }

  return findings;
}

function runStandalone() {
  const findings = collectUserDiagnosticsInternalAnalyticsFindings();
  if (findings.length === 0) {
    console.log('Settings diagnostics boundary audit passed with no findings.');
    return 0;
  }

  console.error('Settings diagnostics boundary audit findings:');
  for (const finding of findings) {
    console.error(`- ${finding.file}:${finding.line} [${finding.rule}] ${finding.message}`);
  }
  return 1;
}

if (process.argv[1] && path.resolve(process.argv[1]) === SCRIPT_PATH) {
  process.exit(runStandalone());
}
