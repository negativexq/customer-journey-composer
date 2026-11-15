import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
test('journey breakdowns preserve version, reveal blocked recipients and paginate', async ({
  page,
}, testInfo) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Journey analyst',
        workspace: 'Journey insights',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  let j = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Recovery insights' } })
  ).json();
  const workflow = {
    schema_version: 1,
    global_exit: { event_name: 'order_refunded', reason: 'completed' },
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Cart updated',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'check',
        type: 'condition',
        label: 'Eligible cart?',
        position: { x: 0, y: 200 },
        config: { rule: { path: 'event.properties.amount', operator: 'greater_than', value: 50 } },
      },
      {
        id: 'sms',
        type: 'sms',
        label: 'Send reminder',
        position: { x: 0, y: 400 },
        config: { text: 'Your cart is waiting.' },
      },
      {
        id: 'wait',
        type: 'wait',
        label: 'Wait for purchase',
        position: { x: 0, y: 600 },
        config: { duration_seconds: 86400 },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Completed',
        position: { x: 0, y: 800 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'check', port: 'started' },
      { id: 'b', source: 'check', target: 'sms', port: 'true' },
      { id: 'c', source: 'check', target: 'done', port: 'false' },
      { id: 'd', source: 'sms', target: 'wait', port: 'next' },
      { id: 'e', source: 'wait', target: 'done', port: 'completed' },
    ],
  };
  async function publish() {
    j = await (
      await page.request.put(`/api/v1/journeys/${j.id}/draft`, {
        headers,
        data: { name: j.name, description: '', draft: workflow, revision: j.revision },
      })
    ).json();
    const run = await (
      await page.request.post('/api/v1/test-runs', {
        headers,
        data: {
          journey_id: j.id,
          scenario: {
            name: 'Gate',
            entry_event: { event_name: 'cart_updated', properties: { amount: 80 } },
            customer: { consents: { sms: false, email: false } },
            assertions: [{ type: 'no_duplicate_delivery', expected: true }],
          },
        },
      })
    ).json();
    expect(
      (
        await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
          headers,
          data: { operation: 'complete', revision: run.revision },
        })
      ).ok(),
    ).toBeTruthy();
    const r = await page.request.post(`/api/v1/journeys/${j.id}/publish`, {
      headers,
      data: { revision: j.revision },
    });
    expect(r.ok()).toBeTruthy();
    j = await (await page.request.get(`/api/v1/journeys/${j.id}`)).json();
  }
  async function enroll(id: string, consent: boolean) {
    expect(
      (
        await page.request.post('/api/v1/customers', {
          headers,
          data: {
            customer_id: id,
            phone: '+15005550006',
            consents: { sms: consent, email: false },
          },
        })
      ).ok(),
    ).toBeTruthy();
    const r = await page.request.post('/api/v1/events', {
      headers,
      data: {
        event_id: randomBytes(12).toString('hex'),
        customer_id: id,
        event_name: 'cart_updated',
        occurred_at: new Date().toISOString(),
        properties: { amount: 80 },
      },
    });
    expect(r.ok()).toBeTruthy();
  }
  await publish();
  for (let i = 0; i < 27; i++) await enroll('v1_' + String(i).padStart(2, '0'), false);
  workflow.nodes[0]!.label = 'Cart updated · revision 2';
  await publish();
  await enroll('v2_allowed', true);
  await enroll('v2_blocked', false);
  await expect
    .poll(async () => {
      const data = await (
        await page.request.get(`/api/v1/journeys/${j.id}/insights?version=2&days=7`)
      ).json();
      return (
        data.metrics?.find(
          (m: { dimension: string; key: string; metric: string }) =>
            m.dimension === 'node_delivery' && m.key === 'sms' && m.metric === 'BLOCKED',
        )?.value || 0
      );
    })
    .toBe(1);
  await page.request.post('/api/v1/events', {
    headers,
    data: {
      event_id: randomBytes(12).toString('hex'),
      customer_id: 'v2_allowed',
      event_name: 'order_refunded',
      occurred_at: new Date().toISOString(),
      properties: {},
    },
  });
  const afterRefund = await (
    await page.request.get(`/api/v1/journeys/${j.id}/executions?version=2&customer_id=v2_allowed`)
  ).json();
  expect(afterRefund[0].exit_reason).toBe('completed');
  await page.goto(`/journeys/${j.id}?tab=live-monitor`);
  await page.getByRole('combobox', { name: 'Published version', exact: true }).click();
  await page.getByRole('option', { name: 'Version 2', exact: true }).click();
  await expect(page.locator('.canvas-zoom-level')).toContainText('90%');
  // The monitor opens on the map; the people list is its own section.
  const sections = page.getByRole('navigation', { name: 'Monitor sections' });
  await expect(page.locator('.observe-people')).toHaveCount(0);
  await sections.getByRole('button', { name: 'Executions', exact: true }).click();
  await expect(page.getByRole('button', { name: /v2_blocked · WAITING/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /v1_.* · WAITING/ })).toHaveCount(0);
  await page.getByRole('combobox', { name: 'Step', exact: true }).click();
  await page.getByRole('option', { name: 'Send reminder', exact: true }).click();
  const panel = page.getByRole('region', { name: 'Step breakdown' });
  await expect
    .poll(async () => {
      const target = await page
        .locator('.monitor-canvas .vue-flow__node')
        .filter({ hasText: 'Send reminder' })
        .boundingBox();
      const area = await page.locator('.monitor-canvas').boundingBox();
      return (
        !!target && !!area && target.y >= area.y && target.y + target.height <= area.y + area.height
      );
    })
    .toBe(true);
  await expect(panel).toContainText('Channel permission missing');
  await panel.getByRole('tab', { name: 'Summary', exact: true }).focus();
  await page.keyboard.press('ArrowRight');
  await expect(panel.getByRole('tab', { name: 'People', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  await page.keyboard.press('ArrowLeft');
  await page.screenshot({ path: testInfo.outputPath('live-node-breakdown.png'), fullPage: true });
  await panel.getByRole('button', { name: /Channel permission missing/ }).click();
  await expect(panel.getByRole('button', { name: /v2_blocked/ })).toBeVisible();
  await expect(panel.getByRole('button', { name: /v2_allowed/ })).toHaveCount(0);
  await panel.getByRole('button', { name: /v2_blocked/ }).click();
  await expect(page.getByRole('region', { name: 'Execution details' })).toContainText('v2_blocked');
  await expect(
    page.getByRole('region', { name: 'Execution details' }).locator('.timeline-step'),
  ).not.toHaveCount(0);
  await page.getByRole('button', { name: 'Close customer journey', exact: true }).click();
  await page.getByRole('combobox', { name: 'Published version', exact: true }).click();
  await page.getByRole('option', { name: 'Version 1', exact: true }).click();
  await sections.getByRole('button', { name: 'Executions', exact: true }).click();
  await expect(page.getByRole('button', { name: /v1_.* · WAITING/ })).toHaveCount(25);
  await page.getByRole('button', { name: 'Load more executions', exact: true }).click();
  await expect(page.getByRole('button', { name: /v1_.* · WAITING/ })).toHaveCount(27);
  // Messages are a section of their own, shown full width without the map or the list.
  await sections.getByRole('button', { name: 'Messages', exact: true }).click();
  await expect(sections.getByRole('button', { name: 'Messages', exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  );
  const messages = page.getByRole('region', { name: 'Message breakdown' });
  await expect(messages.getByRole('heading', { name: 'Journey messages' })).toBeVisible();
  await expect(page.locator('.observe-people')).toHaveCount(0);
  await expect(page.locator('.monitor-canvas')).toHaveCount(0);
  await sections.getByRole('button', { name: 'Journey map', exact: true }).click();
  await expect(page.locator('.monitor-canvas')).toBeVisible();
  await expect(messages).toHaveCount(0);
  await page
    .locator('.journey-tabs')
    .getByRole('button', { name: 'Analytics', exact: true })
    .click();
  await expect(
    page.getByRole('combobox', { name: 'Published version', exact: true }),
  ).toContainText('Version 1');
  await page.getByRole('combobox', { name: 'Entry period', exact: true }).click();
  await page.getByRole('option', { name: 'Last 7 days', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Step reach · workflow order', exact: true }),
  ).toBeVisible();
  const steps = page.getByRole('table').first().locator('tbody tr');
  await expect(steps.first()).toContainText('Cart updated');
  await expect(steps.last()).toContainText('Completed');
  // A step count opens the people behind it and brings that list into view.
  await steps
    .filter({ hasText: 'Cart updated' })
    .getByRole('button', { name: /executions that reached Cart updated/ })
    .click();
  const people = page.locator('.observe-people');
  await expect(people.getByRole('heading')).toContainText('Executions · Cart updated');
  await expect(people).toContainText('Reached Cart updated');
  await expect(people.getByRole('button', { name: /v1_.* · WAITING/ })).toHaveCount(25);
  await expect(people).toBeInViewport();
  await people.getByRole('button', { name: 'Clear breakdown', exact: true }).click();
  await expect(people).not.toContainText('Reached Cart updated');
  await page.reload();
  await expect(page.getByRole('combobox', { name: 'Entry period', exact: true })).toContainText(
    'Last 7 days',
  );
  await expect(
    page.getByRole('combobox', { name: 'Published version', exact: true }),
  ).toContainText('Version 1');
  const channel = page.getByRole('table').last().locator('tbody tr').filter({ hasText: 'SMS' });
  await channel.getByRole('button', { name: '27', exact: true }).last().click();
  const messagePanel = page.getByRole('region', { name: 'Message breakdown' });
  await expect(messagePanel.getByRole('button', { name: /v1_\d+/ })).toHaveCount(25);
  await messagePanel.getByRole('button', { name: 'Load more messages', exact: true }).click();
  await expect(messagePanel.getByRole('button', { name: /v1_\d+/ })).toHaveCount(27);
  await page.getByRole('button', { name: 'Close step breakdown', exact: true }).click();
  await page.screenshot({ path: testInfo.outputPath('journey-analytics.png'), fullPage: true });
  const unrelated = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Unrelated journey' } })
  ).json();
  const source = await (
    await page.request.get(`/api/v1/journeys/${j.id}/executions?version=1&limit=1`)
  ).json();
  await page.goto(`/journeys/${unrelated.id}?tab=live-monitor&execution=${source[0].id}`);
  await expect(page.getByRole('alert')).toContainText('different journey');
  await expect(page.getByRole('region', { name: 'Execution details' })).toHaveCount(0);
});
