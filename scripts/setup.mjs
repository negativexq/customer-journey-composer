import { randomBytes } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';

const password = randomBytes(24).toString('hex');
const template = readFileSync(new URL('../.env.example', import.meta.url), 'utf8');
const demoPassword = randomBytes(24).toString('hex');
const env = template
  .replace('DEMO_PASSWORD=\n', `DEMO_PASSWORD=${demoPassword}\n`)
  .replace('POSTGRES_PASSWORD=\n', `POSTGRES_PASSWORD=${password}\n`)
  .replace(
    'DATABASE_URL=\n',
    `DATABASE_URL=postgres://journey:${password}@127.0.0.1:54329/journeys?sslmode=disable\n`,
  );
try {
  writeFileSync(new URL('../.env', import.meta.url), env, { flag: 'wx', mode: 0o600 });
  console.log('Created ignored .env with a random local database password.');
} catch (error) {
  if (error.code !== 'EEXIST') throw error;
  const path = new URL('../.env', import.meta.url);
  let existing = readFileSync(path, 'utf8');
  if (!/^APP_ENV=/m.test(existing)) existing += '\nAPP_ENV=development\n';
  if (/^APP_ENV=development$/m.test(existing) && !/^DEMO_ENABLED=/m.test(existing)) {
    existing += `\nDEMO_ENABLED=true\nDEMO_EMAIL=demo@example.invalid\nDEMO_PASSWORD=${demoPassword}\n`;
    writeFileSync(path, existing, { mode: 0o600 });
    console.log('Added local demo configuration; existing settings preserved.');
  } else {
    console.log('Existing .env preserved.');
  }
}

// Local observability admin secret. Anonymous Grafana access is read-only.
const observabilityEnvPath = new URL('../.env', import.meta.url);
let observabilityEnv = readFileSync(observabilityEnvPath, 'utf8');
if (!/^GRAFANA_ADMIN_PASSWORD=.+$/m.test(observabilityEnv)) {
  observabilityEnv = observabilityEnv.replace(/^GRAFANA_ADMIN_PASSWORD=.*$/m, '');
  observabilityEnv += `\nGRAFANA_ADMIN_PASSWORD=${randomBytes(24).toString('hex')}\n`;
  writeFileSync(observabilityEnvPath, observabilityEnv, { mode: 0o600 });
}
