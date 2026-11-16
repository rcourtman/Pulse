import fs from 'node:fs';
import path from 'node:path';
import ts from 'typescript';

const sourceRoot = path.resolve('src');

const sourceFiles = [];
const collectSourceFiles = (directory) => {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      collectSourceFiles(entryPath);
    } else if (entry.isFile() && entry.name.endsWith('.tsx')) {
      sourceFiles.push(entryPath);
    }
  }
};

collectSourceFiles(sourceRoot);

const labelableElements = new Set([
  'button',
  'input',
  'meter',
  'output',
  'progress',
  'select',
  'textarea',
]);
const diagnostics = [];

const getTagName = (node, sourceFile) => node.tagName.getText(sourceFile);
const getAttributeValue = (attribute, sourceFile) =>
  attribute.initializer?.getText(sourceFile).replace(/^['"]|['"]$/g, '');

const getStaticString = (initializer) => {
  let value = ts.isJsxExpression(initializer) ? initializer.expression : initializer;
  while (value && ts.isParenthesizedExpression(value)) value = value.expression;
  return value && (ts.isStringLiteral(value) || ts.isNoSubstitutionTemplateLiteral(value))
    ? value.text
    : undefined;
};

const isLabelableControl = (element, sourceFile) => {
  const tag = getTagName(element, sourceFile);
  if (!labelableElements.has(tag)) return false;
  if (tag !== 'input') return true;

  const attributes = element.attributes.properties;
  const typeIndex = attributes.findLastIndex(
    (attribute) => ts.isJsxAttribute(attribute) && attribute.name.getText(sourceFile) === 'type',
  );
  if (typeIndex < 0) return true;
  // A later spread can change the type; do not infer its runtime value. A
  // static hidden input, unlike other native inputs, cannot be a label target.
  if (attributes.slice(typeIndex + 1).some(ts.isJsxSpreadAttribute)) return true;
  const initializer = attributes[typeIndex].initializer;
  const type = initializer && getStaticString(initializer);
  return type?.toLowerCase() !== 'hidden';
};

const hasLabelTarget = (element, sourceFile, labelableIds) => {
  const attributes = element.openingElement.attributes.properties;
  const forAttribute = attributes.find(
    (attribute) =>
      ts.isJsxAttribute(attribute) &&
      ['for', 'htmlFor'].includes(attribute.name.getText(sourceFile)),
  );
  if (forAttribute) {
    const target = getAttributeValue(forAttribute, sourceFile);
    return target !== undefined && labelableIds.has(target);
  }

  let containsLabelableElement = false;
  const visit = (node) => {
    if (
      (ts.isJsxElement(node) && isLabelableControl(node.openingElement, sourceFile)) ||
      (ts.isJsxSelfClosingElement(node) && isLabelableControl(node, sourceFile))
    ) {
      containsLabelableElement = true;
      return;
    }
    ts.forEachChild(node, visit);
  };
  element.children.forEach(visit);
  return containsLabelableElement;
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

  const labelableIds = new Set();
  const collectLabelableIds = (node) => {
    const element = ts.isJsxElement(node)
      ? node.openingElement
      : ts.isJsxSelfClosingElement(node)
        ? node
        : undefined;
    if (element && isLabelableControl(element, sourceFile)) {
      const idAttribute = element.attributes.properties.find(
        (attribute) => ts.isJsxAttribute(attribute) && attribute.name.getText(sourceFile) === 'id',
      );
      if (idAttribute) {
        const id = getAttributeValue(idAttribute, sourceFile);
        if (id !== undefined) labelableIds.add(id);
      }
    }
    ts.forEachChild(node, collectLabelableIds);
  };
  collectLabelableIds(sourceFile);

  const visit = (node) => {
    if (
      ts.isJsxElement(node) &&
      getTagName(node.openingElement, sourceFile) === 'label' &&
      !hasLabelTarget(node, sourceFile, labelableIds)
    ) {
      const position = sourceFile.getLineAndCharacterOfPosition(node.openingElement.getStart());
      diagnostics.push(
        `${path.relative(process.cwd(), sourcePath)}:${position.line + 1}:${position.character + 1} ` +
          'label must target an ID on a native labelable control or contain that control; hidden inputs are not labelable',
      );
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
}

if (diagnostics.length > 0) {
  console.error('Form label audit failed:\n');
  diagnostics.forEach((diagnostic) => console.error(`- ${diagnostic}`));
  process.exit(1);
}

console.log(`Form label audit passed (${sourceFiles.length} TSX files checked).`);
