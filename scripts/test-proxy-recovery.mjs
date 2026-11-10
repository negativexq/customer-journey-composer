import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import assert from 'node:assert/strict';
const network = 'cjc-proxy-' + randomUUID();
const containers = [];
const docker = (...args) =>
  execFileSync('docker', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
function run(args) {
  const id = docker('run', '-d', '--network', network, ...args);
  containers.push(id);
  return id;
}
function address(id) {
  return JSON.parse(docker('inspect', id))[0].NetworkSettings.Networks[network].IPAddress;
}
try {
  docker('network', 'create', network);
  const server = () =>
    run([
      '--network-alias',
      'api',
      'busybox:1.37.0',
      'sh',
      '-c',
      'mkdir -p /www; printf "ready" > /www/readyz; exec httpd -f -p 8080 -h /www',
    ]);
  const original = server();
  const oldIP = address(original);
  const web = run(['-p', '127.0.0.1::8080', 'customer-journey-composer-web:dev']);
  const port = docker('port', web, '8080/tcp').split(':').at(-1);
  async function ready() {
    const until = Date.now() + 20000;
    while (Date.now() < until) {
      try {
        const r = await fetch(`http://127.0.0.1:${port}/readyz`, {
          signal: AbortSignal.timeout(1000),
        });
        if (r.ok && (await r.text()) === 'ready') return;
      } catch {
        /* container starts asynchronously */
      }
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
    throw new Error('Proxy did not resolve the available backend');
  }
  await ready();
  docker('stop', original);
  docker('rm', original);
  run(['--ip', oldIP, 'busybox:1.37.0', 'sleep', '60']);
  const replacement = server();
  assert.notEqual(address(replacement), oldIP);
  await ready();
  assert.equal(JSON.parse(docker('inspect', web))[0].State.Running, true);
  console.log(
    'PASS: web proxy recovers after backend IP replacement without a web restart; developer containers were untouched.',
  );
} finally {
  for (const id of containers) {
    try {
      docker('stop', '-t', '1', id);
      docker('rm', id);
    } catch {
      /* already removed */
    }
  }
  try {
    docker('network', 'rm', network);
  } catch {
    /* preserve original failure */
  }
}
