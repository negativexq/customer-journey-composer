import { randomBytes } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';

const password = randomBytes(24).toString('hex');
const template = readFileSync(new URL('../.env.example', import.meta.url), 'utf8');
const env = template
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
  console.log('Existing .env preserved.');
}
