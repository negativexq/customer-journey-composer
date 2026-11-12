import test from 'node:test';
import assert from 'node:assert/strict';
import { layoutGeneratedWorkflow } from '../apps/web/src/lib/generated-layout.ts';
const source = {
  schema_version: 1,
  nodes: [
    {
      id: 'start',
      type: 'event_trigger',
      label: 'Start',
      position: { x: 0, y: 0 },
      config: { event_name: 'entry' },
    },
    {
      id: 'email',
      type: 'email',
      label: 'Email',
      position: { x: 0, y: 0 },
      config: { text: 'Hello' },
    },
    {
      id: 'done',
      type: 'exit',
      label: 'Done',
      position: { x: 0, y: 0 },
      config: { reason: 'completed' },
    },
  ],
  edges: [
    { id: 'a', source: 'start', target: 'email', port: 'started' },
    { id: 'b', source: 'email', target: 'done', port: 'next' },
  ],
};
test('generated overlapping coordinates become readable without changing behavior or input', () => {
  const before = JSON.stringify(source);
  const result = layoutGeneratedWorkflow(source);
  assert.equal(JSON.stringify(source), before);
  assert.deepEqual(result.edges, source.edges);
  for (const n of result.nodes)
    assert.deepEqual(n.config, source.nodes.find((x) => x.id === n.id).config);
  for (let i = 0; i < result.nodes.length; i++)
    for (let j = i + 1; j < result.nodes.length; j++) {
      const a = result.nodes[i].position,
        b = result.nodes[j].position;
      assert.ok(Math.abs(a.x - b.x) >= 248 || Math.abs(a.y - b.y) >= 220);
    }
});
test('modifying messages preserves user-arranged steps and places only new steps', () => {
  const prior = structuredClone(source);
  prior.nodes[0].position = { x: 500, y: 900 };
  prior.nodes[1].position = { x: 800, y: 1200 };
  const modified = layoutGeneratedWorkflow(source, prior);
  assert.deepEqual(modified.nodes[0].position, prior.nodes[0].position);
  assert.deepEqual(modified.nodes[1].position, prior.nodes[1].position);
});
