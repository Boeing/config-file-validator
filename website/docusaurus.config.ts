import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Config File Validator',
  tagline: 'One tool to validate every config file in your repo',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  url: 'https://boeing.github.io',
  baseUrl: '/config-file-validator/',

  organizationName: 'Boeing',
  projectName: 'config-file-validator',

  onBrokenLinks: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl:
            'https://github.com/Boeing/config-file-validator/edit/main/website/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: true,
      },
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Config File Validator',
      logo: {
        alt: 'Config File Validator logo',
        src: 'img/logo.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          href: 'https://github.com/Boeing/config-file-validator',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/introduction',
            },
            {
              label: 'CLI Reference',
              to: '/docs/reference/cli-flags',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/Boeing/config-file-validator/discussions',
            },
            {
              label: 'Issues',
              href: 'https://github.com/Boeing/config-file-validator/issues',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/Boeing/config-file-validator',
            },
            {
              label: 'Go Package',
              href: 'https://pkg.go.dev/github.com/Boeing/config-file-validator/v2',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} The Boeing Company. Licensed under Apache 2.0.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['toml', 'ini', 'hcl', 'json', 'yaml', 'bash', 'go'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
