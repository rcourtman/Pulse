import ts from 'typescript';

// Inspect surviving JSX, not symbol mentions: removing a whole link is valid,
// but leaving an import or comment must not conceal a raw or misdirected link.
export function settingsDocumentationLinkViolations(source: string): string[] {
  const file = ts.createSourceFile(
    'settings.tsx',
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const imports = new Map<string, { module: string; symbol: string }>();
  const violations: string[] = [];
  let termsLinks = 0;

  for (const statement of file.statements) {
    if (!ts.isImportDeclaration(statement) || !ts.isStringLiteral(statement.moduleSpecifier))
      continue;
    const bindings = statement.importClause?.namedBindings;
    if (!bindings || !ts.isNamedImports(bindings)) continue;
    for (const binding of bindings.elements) {
      imports.set(binding.name.text, {
        module: statement.moduleSpecifier.text,
        symbol: (binding.propertyName ?? binding.name).text,
      });
    }
  }

  const visit = (node: ts.Node): void => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const name = node.tagName.getText(file);
      const imported = imports.get(name);
      if (name === 'a')
        violations.push('Settings documentation text links must use ExternalTextLink');
      if (name === 'ExternalTextLink' || imported?.symbol === 'ExternalTextLink') {
        if (imported?.module !== '@/components/shared/ExternalTextLink') {
          violations.push('ExternalTextLink must come from the shared primitive');
        }
        const attributes = node.attributes.properties.filter(ts.isJsxAttribute);
        if (
          attributes.some((attribute) =>
            ['preserveOpener', 'rel', 'target'].includes(attribute.name.getText(file)),
          )
        ) {
          violations.push('Documentation links must keep shared new-tab safety');
        }
        const href = attributes.find(
          (attribute) => attribute.name.getText(file) === 'href',
        )?.initializer;
        const identifier =
          href && ts.isJsxExpression(href) && href.expression && ts.isIdentifier(href.expression)
            ? href.expression.text
            : undefined;
        const hrefImport = identifier ? imports.get(identifier) : undefined;
        const label =
          ts.isJsxOpeningElement(node) && ts.isJsxElement(node.parent)
            ? node.parent.children
                .map((child) => child.getText(file))
                .join(' ')
                .replace(/\s+/g, ' ')
            : '';
        if (/Terms(?: of Service)?/.test(label) || hrefImport?.symbol === 'TERMS_DOC_URL') {
          termsLinks += 1;
          if (hrefImport?.symbol !== 'TERMS_DOC_URL' || hrefImport.module !== '@/utils/docsLinks') {
            violations.push('Surviving Terms links must use the shipped TERMS_DOC_URL');
          }
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  if (
    [...imports.values()].some((binding) => binding.symbol === 'TERMS_DOC_URL') &&
    termsLinks === 0
  ) {
    violations.push('A Terms import alone does not establish a surviving documentation link');
  }
  return violations;
}
