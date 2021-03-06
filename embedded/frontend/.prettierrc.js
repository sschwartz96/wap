module.exports = {
  useTabs: false,
  printWidth: 80,
  tabWidth: 2,
  semi: false,
  trailingComma: 'none',
  singleQuote: true,
  plugins: [require('prettier-plugin-svelte')],
  svelteSortOrder: 'options-scripts-markup-styles',
  svelteStrictMode: false,
  svelteBracketNewLine: false,
  svelteAllowShorthand: true,
  svelteIndentScriptAndStyle: true,
  overrides: [
    {
      files: '**/*.svx',
      options: { parser: 'markdown' }
    },
    {
      files: '**/*.ts',
      options: { parser: 'typescript' }
    },
    {
      files: '**/CHANGELOG.md',
      options: {
        requirePragma: true
      }
    }
  ]
}
