import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'introduction',
        'installation',
        'quick-start',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/schema-validation',
        'guides/configuration-file',
        'guides/glob-patterns',
        'guides/file-type-detection',
        'guides/filtering-exclusion',
        'guides/output-reporters',
        'guides/stdin',
      ],
    },
    {
      type: 'category',
      label: 'Integrations',
      items: [
        'integrations/github-actions',
        'integrations/pre-commit',
        {
          type: 'doc',
          id: 'integrations/ci-cd',
          label: 'CI/CD Pipelines',
        },
        'integrations/go-library',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/cli-flags',
        'reference/configuration-keys',
        'reference/environment-variables',
        'reference/supported-file-types',
        'reference/exit-codes',
        'reference/known-files',
      ],
    },
    {
      type: 'category',
      label: 'Contributing',
      items: [
        'contributing/development-setup',
        'contributing/contributing-guide',
      ],
    },
  ],
};

export default sidebars;
