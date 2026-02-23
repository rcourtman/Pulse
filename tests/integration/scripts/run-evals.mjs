#!/usr/bin/env node

import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const INTEGRATION_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const SCENARIOS_FILE = path.join(INTEGRATION_ROOT, 'evals', 'scenarios.json');
// Keep eval reports outside Playwright's test-results directory.
// Playwright clears test-results between invocations, which can remove report paths mid-run.
const DEFAULT_RESULTS_ROOT = path.join(INTEGRATION_ROOT, 'eval-results');

const args = process.argv.slice(2);
const hasArg = (flag) => args.includes(flag);
const argValue = (flag) => {
  const idx = args.indexOf(flag);
  if (idx === -1 || idx + 1 >= args.length) return null;
  return args[idx + 1];
};

if (hasArg('--help') || hasArg('-h')) {
  console.log(`
Usage: node ./scripts/run-evals.mjs [options]

Options:
  --scenario <id[,id2]>   Run one or more specific scenario ids
  --mode <name>           deterministic (default) | agentic
  --dry-run               Print planned commands without executing
  --help                  Show this help

Environment:
  PULSE_EVAL_MODE                         Default mode when --mode is not provided
  PULSE_EVAL_AGENT_COMMAND_TEMPLATE       Required for agentic mode. Shell command with placeholders:
                                          {{task_file}}, {{result_json}}, {{scenario_id}}, {{base_url}}
  PULSE_BASE_URL                          Base URL passed to scenarios (default http://localhost:7655)
  PULSE_E2E_USERNAME                      Username context for prompts (default admin)
  PULSE_E2E_PASSWORD                      Password context for prompts (default adminadminadmin)
`.trim());
  process.exit(0);
}

const selectedScenarioIDs = new Set(
  (argValue('--scenario') || '')
    .split(',')
    .map((v) => v.trim())
    .filter(Boolean),
);

const mode = (argValue('--mode') || process.env.PULSE_EVAL_MODE || 'deterministic').trim();
const dryRun = hasArg('--dry-run');

const baseURL = (process.env.PULSE_BASE_URL || 'http://localhost:7655').trim();
const username = (process.env.PULSE_E2E_USERNAME || 'admin').trim();
const password = (process.env.PULSE_E2E_PASSWORD || 'adminadminadmin').trim();

function nowStamp() {
  return new Date().toISOString().replace(/[:.]/g, '-');
}

function formatMs(ms) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function runCommand(command, argsList, env = {}, useShell = false) {
  return new Promise((resolve) => {
    const startedAt = Date.now();
    const child = spawn(command, argsList, {
      cwd: INTEGRATION_ROOT,
      env: { ...process.env, ...env },
      stdio: 'inherit',
      shell: useShell,
    });
    child.on('close', (code) => {
      resolve({
        code: code ?? 1,
        durationMs: Date.now() - startedAt,
      });
    });
  });
}

function renderTemplate(template, replacements) {
  let output = template;
  for (const [key, value] of Object.entries(replacements)) {
    output = output.replaceAll(`{{${key}}}`, String(value));
  }
  return output;
}

function buildMarkdownReport(summary) {
  const total = summary.results.length;
  const passed = summary.results.filter((r) => r.status === 'pass').length;
  const failed = total - passed;
  const lines = [];
  lines.push('# Pulse Agentic Eval Report');
  lines.push('');
  lines.push(`- Mode: \`${summary.mode}\``);
  lines.push(`- Base URL: \`${summary.baseURL}\``);
  lines.push(`- Total: ${total}`);
  lines.push(`- Passed: ${passed}`);
  lines.push(`- Failed: ${failed}`);
  lines.push('');
  lines.push('## Scenario Results');
  lines.push('');
  for (const result of summary.results) {
    lines.push(`- [${result.status === 'pass' ? 'PASS' : 'FAIL'}] ${result.id} (${formatMs(result.durationMs)})`);
    lines.push(`  Summary: ${result.summary || 'No summary provided'}`);
    if (Array.isArray(result.issues) && result.issues.length > 0) {
      lines.push(`  Issues: ${result.issues.join(' | ')}`);
    }
  }
  return lines.join('\n');
}

async function loadScenarios() {
  const raw = await fs.readFile(SCENARIOS_FILE, 'utf8');
  const parsed = JSON.parse(raw);
  return Array.isArray(parsed.scenarios) ? parsed.scenarios : [];
}

async function runDeterministicScenario(scenario) {
  const spec = scenario?.deterministic;
  if (!spec || !Array.isArray(spec.command) || spec.command.length === 0) {
    return {
      status: 'fail',
      summary: 'Scenario missing deterministic command',
      issues: ['invalid deterministic configuration'],
      durationMs: 0,
    };
  }

  const [command, ...argsList] = spec.command;
  const commandEnv = {
    ...spec.env,
    PULSE_BASE_URL: baseURL,
    PULSE_E2E_USERNAME: username,
    PULSE_E2E_PASSWORD: password,
  };

  if (dryRun) {
    console.log(`[dry-run] ${command} ${argsList.join(' ')}`);
    return {
      status: 'pass',
      summary: 'Dry run only',
      issues: [],
      durationMs: 0,
    };
  }

  const exec = await runCommand(command, argsList, commandEnv, false);
  return {
    status: exec.code === 0 ? 'pass' : 'fail',
    summary: exec.code === 0 ? 'Deterministic run passed' : `Deterministic run failed with exit ${exec.code}`,
    issues: exec.code === 0 ? [] : [`exit code ${exec.code}`],
    durationMs: exec.durationMs,
  };
}

async function runAgenticScenario(scenario, scenarioRunDir) {
  const agentTemplate = process.env.PULSE_EVAL_AGENT_COMMAND_TEMPLATE;
  if (dryRun && (!agentTemplate || agentTemplate.trim() === '')) {
    return {
      status: 'pass',
      summary:
        'Dry run only (set PULSE_EVAL_AGENT_COMMAND_TEMPLATE to execute agentic scenarios)',
      issues: [],
      durationMs: 0,
    };
  }
  if (!agentTemplate || agentTemplate.trim() === '') {
    return {
      status: 'fail',
      summary: 'Agentic mode requested but PULSE_EVAL_AGENT_COMMAND_TEMPLATE is unset',
      issues: ['missing PULSE_EVAL_AGENT_COMMAND_TEMPLATE'],
      durationMs: 0,
    };
  }

  const taskRelPath = scenario?.agentic?.task_file;
  if (!taskRelPath) {
    return {
      status: 'fail',
      summary: 'Scenario missing agentic task file',
      issues: ['invalid agentic configuration'],
      durationMs: 0,
    };
  }

  const taskTemplatePath = path.join(INTEGRATION_ROOT, taskRelPath);
  const taskTemplate = await fs.readFile(taskTemplatePath, 'utf8');
  const resultJSONPath = path.join(scenarioRunDir, `${scenario.id}.json`);
  const renderedTaskPath = path.join(scenarioRunDir, `${scenario.id}.task.md`);
  const renderedTask = renderTemplate(taskTemplate, {
    base_url: baseURL,
    username,
    password,
    result_json: resultJSONPath,
  });
  await fs.writeFile(renderedTaskPath, renderedTask, 'utf8');

  const renderedCommand = renderTemplate(agentTemplate, {
    task_file: renderedTaskPath,
    result_json: resultJSONPath,
    scenario_id: scenario.id,
    base_url: baseURL,
  });

  if (dryRun) {
    console.log(`[dry-run] ${renderedCommand}`);
    return {
      status: 'pass',
      summary: 'Dry run only',
      issues: [],
      durationMs: 0,
    };
  }

  const exec = await runCommand(renderedCommand, [], {
    PULSE_EVAL_SCENARIO: scenario.id,
    PULSE_EVAL_BASE_URL: baseURL,
    PULSE_EVAL_RESULT_JSON: resultJSONPath,
  }, true);

  if (exec.code !== 0) {
    return {
      status: 'fail',
      summary: `Agent command failed with exit ${exec.code}`,
      issues: [`exit code ${exec.code}`],
      durationMs: exec.durationMs,
    };
  }

  try {
    const raw = await fs.readFile(resultJSONPath, 'utf8');
    const parsed = JSON.parse(raw);
    const status = parsed?.status === 'pass' ? 'pass' : 'fail';
    const summary = typeof parsed?.summary === 'string' ? parsed.summary : 'Agentic run completed';
    const issues = Array.isArray(parsed?.issues)
      ? parsed.issues.filter((v) => typeof v === 'string')
      : [];
    return {
      status,
      summary,
      issues,
      durationMs: exec.durationMs,
    };
  } catch {
    return {
      status: 'fail',
      summary: 'Agentic command finished but no valid JSON result was produced',
      issues: ['invalid or missing scenario result JSON'],
      durationMs: exec.durationMs,
    };
  }
}

async function main() {
  const scenarios = await loadScenarios();
  const selected =
    selectedScenarioIDs.size === 0
      ? scenarios
      : scenarios.filter((scenario) => selectedScenarioIDs.has(scenario.id));

  if (selected.length === 0) {
    console.error('No matching eval scenarios found.');
    process.exit(1);
  }

  if (!['deterministic', 'agentic'].includes(mode)) {
    console.error(`Unsupported mode "${mode}". Use deterministic or agentic.`);
    process.exit(1);
  }

  const runID = nowStamp();
  const runDir = path.join(DEFAULT_RESULTS_ROOT, runID);
  await fs.mkdir(runDir, { recursive: true });

  const results = [];
  for (const scenario of selected) {
    console.log(`\n=== Eval: ${scenario.id} (${mode}) ===`);
    const startedAt = Date.now();
    const result =
      mode === 'deterministic'
        ? await runDeterministicScenario(scenario)
        : await runAgenticScenario(scenario, runDir);
    results.push({
      id: scenario.id,
      name: scenario.name,
      status: result.status,
      summary: result.summary,
      issues: result.issues || [],
      durationMs: result.durationMs || Date.now() - startedAt,
    });
  }

  const summary = {
    generatedAt: new Date().toISOString(),
    mode,
    baseURL,
    results,
  };

  const jsonPath = path.join(runDir, 'report.json');
  const mdPath = path.join(runDir, 'report.md');
  await fs.writeFile(jsonPath, JSON.stringify(summary, null, 2), 'utf8');
  await fs.writeFile(mdPath, buildMarkdownReport(summary), 'utf8');

  const failed = results.filter((r) => r.status !== 'pass');
  console.log(`\nReport: ${jsonPath}`);
  console.log(`Summary: ${mdPath}`);
  if (failed.length > 0) {
    console.error(`Failed scenarios: ${failed.map((r) => r.id).join(', ')}`);
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
