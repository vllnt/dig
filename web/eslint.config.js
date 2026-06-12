import { nextjs } from '@vllnt/eslint-config'

export default [
  {
    ignores: [
      'node_modules/**',
      '.next/**',
      'out/**',
      'dist/**',
      'build/**',
      'coverage/**',
      'public/**',
      'next-env.d.ts',
      'eslint.config.js',
      'next.config.*',
      'playwright.config.*',
      'postcss.config.*',
      'tailwind.config.*',
      'vitest.config.*',
    ],
  },
  ...nextjs,
  {
    rules: {
      'react-hooks/set-state-in-effect': 'off',
      'no-restricted-syntax': [
        'error',
        {
          selector: "MemberExpression[object.name='console']",
          message:
            'Use @vllnt/logger instead of console.*. Import createBackendLogger (server) or createLogger (client).',
        },
      ],
      // Mirror the @vllnt preset's prevent-abbreviations options, allowing the
      // deliberate "docs" concept (the /docs user-docs route, nav, and data
      // module). Re-declared because ESLint rule options replace, not merge.
      'unicorn/prevent-abbreviations': [
        'error',
        {
          extendDefaultReplacements: true,
          replacements: {
            ctx: false,
            db: false,
            docs: false,
            e: false,
            fn: false,
            props: false,
            ref: false,
            utils: false,
          },
          ignore: [
            'e2e',
            'a11y',
            'i18n',
            'getInitialProps',
            'generateStaticParams',
            'dynamicParams',
            '.*Ctx$',
            '.*Ref$',
          ],
        },
      ],
    },
  },
  {
    files: ['app/**/route.ts'],
    rules: {
      '@typescript-eslint/naming-convention': 'off',
    },
  },
  {
    files: ['i18n/routing.ts'],
    rules: {
      '@typescript-eslint/naming-convention': 'off',
    },
  },
  {
    files: ['i18n/request.ts'],
    rules: {
      '@typescript-eslint/no-unsafe-assignment': 'off',
      '@typescript-eslint/no-unsafe-member-access': 'off',
    },
  },
]
