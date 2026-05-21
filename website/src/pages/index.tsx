import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import styles from './index.module.css';

function Hero() {
  return (
    <header className={styles.hero}>
      <div className={styles.heroInner}>
        <img
          src="/config-file-validator/img/logo.png"
          alt="Config File Validator"
          className={styles.heroLogo}
        />
        <h1 className={styles.heroTitle}>Config File Validator</h1>
        <p className={styles.heroSubtitle}>
          One tool to validate every config file in your repo
        </p>
        <div className={styles.heroActions}>
          <Link className={styles.primaryButton} to="/docs/introduction">
            Get Started
          </Link>
          <Link
            className={styles.secondaryButton}
            to="https://github.com/Boeing/config-file-validator"
          >
            GitHub
          </Link>
        </div>
        <div className={styles.installSnippet}>
          <Link to="/docs/installation">See installation options →</Link>
        </div>
        <img
          src="/config-file-validator/img/demo.svg"
          alt="Config File Validator validating JSON, YAML, TOML, and XML files"
          className={styles.heroDemo}
        />
      </div>
    </header>
  );
}

const features = [
  {
    title: '16 File Formats',
    description:
      'JSON, YAML, TOML, XML, HCL, INI, HOCON, ENV, CSV, Properties, EDITORCONFIG, Justfile, PList, SARIF, JSONC, and TOON.',
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
      <div className={styles.featuresGrid}>
        {features.map((feature) => (
          <div key={feature.title} className={styles.featureCard}>
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
      description="Single cross-platform CLI tool to validate configuration files"
    >
      <Hero />
      <main>
        <Features />
      </main>
    </Layout>
  );
}
