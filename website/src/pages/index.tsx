import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import styles from './index.module.css';

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
          A toolchain for configuration files
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
    title: 'Validate',
    description:
      'Catches syntax errors across JSON, YAML, TOML, XML, HCL, and 13 more formats. One command, one pass.',
  },
  {
    title: 'Enforce Schemas',
    description:
      'Validates against JSON Schema and XSD. Automatic SchemaStore lookup finds the right schema by filename.',
  },
  {
    title: 'Format and Fix',
    description:
      'Checks and fixes formatting for JSON, YAML, TOML, XML, HCL, INI, Properties, and ENV. Reads your existing .prettierrc, taplo.toml, and .yamlfmt configs.',
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
      description="A toolchain for configuration files. Validates syntax, enforces schemas, and checks formatting."
    >
      <Hero />
      <main>
        <Features />
      </main>
    </Layout>
  );
}
