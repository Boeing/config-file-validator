import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import styles from './index.module.css';
import { SUPPORTED_FORMATS, formatList } from '../data/supportedFormats';

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
    title: `${SUPPORTED_FORMATS.length} File Formats`,
    description: formatList(SUPPORTED_FORMATS),
  },
  {
    title: 'Syntax + Schema Validation',
    description:
      'Validates structure with JSON Schema and XSD. Automatic SchemaStore integration for hundreds of common config files.',
  },
  {
    title: 'Single Binary, Zero Dependencies',
    description:
      'No runtimes, no package managers. One executable that runs on macOS, Linux, and Windows.',
  },
  {
    title: 'CI/CD Ready',
    description:
      'JSON, JUnit, and SARIF output. GitHub Actions integration with PR annotations. Pre-commit hook support.',
  },
  {
    title: 'Configurable',
    description:
      'Project-level .cfv.toml config files, glob patterns, schema mappings, type overrides, and environment variables.',
  },
  {
    title: 'Go Library',
    description:
      'Embed validation in your own tools. The full CLI is available as a Go package.',
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
