import { execFileSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

export function validSubject(subject) {
  return /^(feat|fix|test|refactor|perf|docs|ci|build|chore)(\([a-z0-9][a-z0-9/-]*\))?!?: [a-z].{2,89}$/.test(
    subject,
  );
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const range = process.argv[2];
  if (range === '--self-test') {
    if (!validSubject('feat(runtime): add durable timers') || validSubject('wip')) process.exit(1);
  } else if (!range) {
    console.error('Usage: npm run commitlint -- <base>..<head>');
    process.exit(1);
  } else {
    const subjects = execFileSync('git', ['log', '--format=%s', '--no-merges', range], {
      encoding: 'utf8',
    }).trim();
    const invalid = subjects.split('\n').filter((subject) => subject && !validSubject(subject));
    if (invalid.length) {
      console.error('Invalid Conventional Commit subjects:\n' + invalid.join('\n'));
      process.exit(1);
    }
  }
}
