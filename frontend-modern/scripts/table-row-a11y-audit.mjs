import fs from 'node:fs';
import path from 'node:path';
import ts from 'typescript';

const sourceRoot = path.resolve('src');
const sourceFiles = [];

const collectSourceFiles = (directory) => {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== '__tests__') collectSourceFiles(entryPath);
    } else if (
      entry.isFile() &&
      entry.name.endsWith('.tsx') &&
      !entry.name.includes('.test.')
    ) {
      sourceFiles.push(entryPath);
    }
  }
};

collectSourceFiles(sourceRoot);

const diagnostics = [];
const isZeroTabIndex = (attribute, sourceFile) => {
  const value = attribute.initializer?.getText(sourceFile) ?? '';
  return /^(?:['"]0['"]|\{0\})$/.test(value);
};

for (const sourcePath of sourceFiles.sort()) {
  const sourceText = fs.readFileSync(sourcePath, 'utf8');
  const sourceFile = ts.createSourceFile(
    sourcePath,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );

  const visit = (node) => {
    const element =
      ts.isJsxElement(node) || ts.isJsxSelfClosingElement(node)
        ? ts.isJsxElement(node)
          ? node.openingElement
          : node
        : undefined;
    if (element && ['tr', 'TableRow'].includes(element.tagName.getText(sourceFile))) {
      const tabIndex = element.attributes.properties.find(
        (attribute) =>
          ts.isJsxAttribute(attribute) &&
          ['tabIndex', 'tabindex'].includes(attribute.name.getText(sourceFile)),
      );
      if (tabIndex && ts.isJsxAttribute(tabIndex) && isZeroTabIndex(tabIndex, sourceFile)) {
        const position = sourceFile.getLineAndCharacterOfPosition(tabIndex.getStart(sourceFile));
        diagnostics.push(
          `${path.relative(process.cwd(), sourcePath)}:${position.line + 1}:${position.character + 1} ` +
            'native data-table rows must not enter the tab sequence; put disclosure behaviour on a button inside the row',
        );
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
}

if (diagnostics.length > 0) {
  console.error('Table row accessibility audit failed:\n');
  diagnostics.forEach((diagnostic) => console.error(`- ${diagnostic}`));
  process.exit(1);
}

console.log(`Table row accessibility audit passed (${sourceFiles.length} TSX files checked).`);
