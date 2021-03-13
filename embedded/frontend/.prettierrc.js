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

// export const useTabs = false
// export const printWidth = 80
// export const tabWidth = 2
// export const semi = false
// export const trailingComma = 'none'
// export const singleQuote = true
// export const plugins = [require('prettier-plugin-svelte')]
// export const svelteSortOrder = 'options-scripts-markup-styles'
// export const svelteStrictMode = false
// export const svelteBracketNewLine = false
// export const svelteAllowShorthand = true
// export const svelteIndentScriptAndStyle = true

// export const overrides = [
//   {
//     files: '**/*.ts',
//     options: { parser: 'typescript' },
//   },
// ]
