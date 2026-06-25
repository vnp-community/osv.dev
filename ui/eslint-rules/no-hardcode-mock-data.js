/**
 * ESLint rule: Prevent hardcoded business data arrays in component files.
 * Files in src/mocks/ are exempt.
 */
module.exports = {
  meta: {
    type: 'problem',
    docs: {
      description: 'Disallow hardcoded data arrays in component/feature files',
    },
    schema: [],
    messages: {
      noHardcode:
        'Hardcoded data array "{{name}}" detected. Use React Query hook + MSW fixture instead. See architecture.md §5.5.',
    },
  },
  create(context) {
    const filename = context.getFilename();

    // Exempt directories
    const exemptPatterns = ['/mocks/', '/utils/', '/__tests__/', '.test.', '.spec.', '/schemas/'];
    if (exemptPatterns.some((p) => filename.includes(p))) return {};

    // Only lint feature and app components
    if (!filename.includes('/features/') && !filename.includes('/app/components/')) return {};

    return {
      VariableDeclaration(node) {
        node.declarations.forEach((decl) => {
          if (
            decl.init?.type === 'ArrayExpression' &&
            decl.init.elements.length >= 3 &&
            decl.init.elements.some(
              (el) => el?.type === 'ObjectExpression' && el.properties.length >= 2
            )
          ) {
            const varName = decl.id?.name ?? 'unknown';

            // Allow UI config patterns (navigation, columns, options)
            const uiPatterns = [
              'COLUMN', 'OPTION', 'TAB', 'STEP', 'NAV', 'MENU', 'ROUTE', 'BREADCRUMB',
              'CHART_COLOR', 'SEVERITY_COLOR',
            ];
            if (uiPatterns.some((p) => varName.toUpperCase().includes(p))) return;

            context.report({
              node,
              messageId: 'noHardcode',
              data: { name: varName },
            });
          }
        });
      },
    };
  },
};
