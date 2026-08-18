#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import ts from 'typescript';

const ROOT = process.cwd();
const SRC_DIR = path.join(ROOT, 'src');
const TEMPLATE_KINDS = new Set([
  ts.SyntaxKind.TemplateHead,
  ts.SyntaxKind.TemplateMiddle,
  ts.SyntaxKind.TemplateTail,
]);
// This module's entire output contract is executable shell, PowerShell, or C#
// bootstrap syntax. It does not own customer-facing prose.
const MACHINE_SYNTAX_ONLY_FILES = new Set(['src/utils/agentInstallCommand.ts']);
const TECHNICAL_DELIMITER_FILES = new Set([
  'src/components/Settings/useBackupTransferFlow.ts',
  'src/utils/resourcePolicyPresentation.ts',
]);
const findings = [];

function toRelative(absPath) {
  return path.relative(ROOT, absPath).replaceAll(path.sep, '/');
}

function collectSourceFiles(directory) {
  const files = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === '__tests__' || entry.name === 'node_modules') continue;
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...collectSourceFiles(fullPath));
      continue;
    }
    if (!entry.isFile() || !/\.(ts|tsx)$/.test(entry.name) || entry.name.includes('.test.')) {
      continue;
    }
    files.push(fullPath);
  }
  return files;
}

function removeHtmlEntities(value) {
  return value.replace(/&(?:#\d+|#x[\da-f]+|[a-z][\da-z]+);/gi, '');
}

function isTechnicalSemicolon(relativePath, value) {
  if (MACHINE_SYNTAX_ONLY_FILES.has(relativePath)) return true;
  if (TECHNICAL_DELIMITER_FILES.has(relativePath) && /^;(?: Secure)?\s*$/.test(value)) {
    return true;
  }
  if (relativePath === 'src/components/Settings/nodeModalModel.ts' && value.includes('pveum ')) {
    return true;
  }
  if (
    relativePath === 'src/components/Settings/useInfrastructureOperationsState.tsx' &&
    (/\$env:|\$\(|\b(?:bash|sudo|else|exit|fi)\b/.test(value) || /^;\s*$/.test(value))
  ) {
    return true;
  }
  if (
    relativePath === 'src/components/shared/useSearchTipsPopoverState.ts' &&
    /px;(?:top|width|max-height):/.test(value)
  ) {
    return true;
  }
  if (relativePath === 'src/utils/apiClient.ts' && /^(?:;|; Secure|=; Path=|; Path=)/.test(value)) {
    return true;
  }
  return (
    /(?:^|\s)animation-(?:delay|duration):/.test(value) ||
    /(?:^|["'])position:fixed !important;/.test(value) ||
    /charset=utf-8/i.test(value) ||
    /;base64,/i.test(value) ||
    /<[^>]+style=["'][^"']*;/.test(value)
  );
}

function inspectFile(absPath) {
  const relativePath = toRelative(absPath);
  const source = fs.readFileSync(absPath, 'utf8');
  const sourceFile = ts.createSourceFile(
    relativePath,
    source,
    ts.ScriptTarget.Latest,
    true,
    relativePath.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );

  const visit = (node) => {
    let value;
    if (
      ts.isStringLiteralLike(node) ||
      ts.isNoSubstitutionTemplateLiteral(node) ||
      ts.isJsxText(node) ||
      TEMPLATE_KINDS.has(node.kind)
    ) {
      value = node.text;
    }

    if (value) {
      const productText = removeHtmlEntities(value);
      if (productText.includes(';') && !isTechnicalSemicolon(relativePath, productText)) {
        const position = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
        findings.push({
          file: relativePath,
          line: position.line + 1,
          preview: productText.trim().replace(/\s+/g, ' ').slice(0, 120),
        });
      }
    }
    ts.forEachChild(node, visit);
  };

  visit(sourceFile);
}

for (const file of collectSourceFiles(SRC_DIR)) inspectFile(file);

if (findings.length > 0) {
  console.error(
    'Copy style audit failed. Replace semicolons in user-visible copy with sentences or a visual separator.',
  );
  for (const finding of findings) {
    console.error(`- ${finding.file}:${finding.line}: ${finding.preview}`);
  }
  process.exit(1);
}

console.log('Copy style audit passed with no user-visible semicolons.');
