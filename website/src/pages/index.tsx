import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import styles from './index.module.css';
import { SUPPORTED_FORMATS, SCHEMA_FORMATS, FORMAT_FORMATS, formatList } from '../data/supportedFormats';

function Hero() {
  return (
    <header className={styles.hero}>
      <div className={styles['hero-inner']}>
        <img
          src="/config-file-validator/img/logo.png"
          alt="Config File Validator"
          className={styles['hero-logo']}
        />
        <h1 className={styles['hero-title']}>Config File Validator</h1>
        <p className={styles['hero-subtitle']}>
          One tool to validate every config file in your repo
        </p>
        <div className={styles['hero-actions']}>
          <Link className={styles['primary-button']} to="/docs/introduction">
            Get Started
          </Link>
          <Link
            className={styles['secondary-button']}
            to="https://github.com/Boeing/config-file-validator"
          >
            GitHub
          </Link>
        </div>
        <div className={styles['install-snippet']}>
          <Link to="/docs/installation">See installation options →</Link>
        </div>
        <img
          src="/config-file-validator/img/demo.svg"
          alt="Config File Validator validating JSON, YAML, TOML, and XML files"
          className={styles['hero-demo']}
        />
      </div>
    </header>
  );
}

const features = [
  {
    title: `${SUPPORTED_FORMATS.length} Formats, One Command`,
    description:
      'cfv check . validates syntax, enforces schemas, and checks formatting in a single pass. cfv check --fix fixes everything.',
  },
  {
    title: 'Schema Enforcement',
    description:
      `Validates ${SCHEMA_FORMATS.length} formats against JSON Schema and XSD. Automatic SchemaStore lookup — no URLs to configure.`,
  },
  {
    title: `Formats ${FORMAT_FORMATS.length} Config Languages`,
    description:
      "JSON, JSONC, YAML, TOML, HCL, XML, INI, Properties, and ENV. Reads your .prettierrc, taplo.toml, .yamlfmt, and .editorconfig for a drop-in migration.",
  },
  {
    title: 'Single Binary, No Runtime',
    description:
      'Static Go executable. No Node, no Python, no package manager. Runs on macOS, Linux, and Windows.',
  },
  {
    title: 'Built for CI',
    description:
      'JUnit, SARIF, and JSON reporters. GitHub Actions annotations on PRs. Exits non-zero on any failure.',
  },
  {
    title: 'Fits Your Workflow',
    description:
      'Respects .gitignore. Configurable via .cfv.toml. Available as a pre-commit hook, GitHub Action, or Go library.',
  },
];

function Features() {
  return (
    <section className={styles.features}>
      <div className={styles['features-grid']}>
        {features.map((feature) => (
          <div key={feature.title} className={styles['feature-card']}>
            <h3>{feature.title}</h3>
            <p>{feature.description}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

export default function Home(): React.JSX.Element {
  return (
    <Layout
      title="Config File Validator"
      description={`Validates config files across ${SUPPORTED_FORMATS.length} formats`}
    >
      <Hero />
      <main>
        <Features />
      </main>
    </Layout>
  );
}
