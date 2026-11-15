import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { choose } from './helpers/select';

const mailpit = `http://127.0.0.1:${process.env.MAILPIT_PORT || '8025'}`;

test('a click on the sent email moves the step waiting for it', async ({ page, request }) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Engagement editor',
        workspace: 'Engagement',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const customer = 'reader_' + randomBytes(6).toString('hex');
  const address = customer + '@example.invalid';
  const created = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Click follow-up' } })
  ).json();
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Cart',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'email',
        type: 'email',
        label: 'Reminder',
        position: { x: 0, y: 170 },
        config: {
          subject: 'Your cart is waiting',
          html: '<p><a href="https://shop.example/cart">Finish your order</a></p>',
        },
      },
      {
        id: 'click',
        type: 'wait_event',
        label: 'Wait for a click',
        position: { x: 0, y: 340 },
        config: { event_name: 'message_clicked', message_node: 'email', timeout_seconds: 86400 },
      },
      {
        id: 'clicked',
        type: 'exit',
        label: 'Clicked',
        position: { x: -160, y: 510 },
        config: { reason: 'converted' },
      },
      {
        id: 'quiet',
        type: 'exit',
        label: 'No click',
        position: { x: 160, y: 510 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'email', port: 'started' },
      { id: 'b', source: 'email', target: 'click', port: 'next' },
      { id: 'c', source: 'click', target: 'clicked', port: 'matched' },
      { id: 'd', source: 'click', target: 'quiet', port: 'timeout' },
    ],
  };
  let journey = await (
    await page.request.put(`/api/v1/journeys/${created.id}/draft`, {
      headers,
      data: { name: created.name, description: '', tags: [], revision: created.revision, draft },
    })
  ).json();

  if (test.info().project.name === 'desktop') {
    await page.goto(`/journeys/${journey.id}`);
    await expect(page.locator('.canvas .vue-flow__node[data-id="clicked"]')).toContainText(
      'If they click a link in Reminder',
    );
    const card = page.locator('.canvas .vue-flow__node[data-id="click"]');
    await expect(card.locator('.workflow-node-detail')).toHaveText('Clicks a link · up to 1 day');
    await card.click();
    const settings = page.getByRole('complementary', { name: 'Step settings' });
    await expect(
      settings.getByRole('radio', { name: 'Message engagement', exact: true }),
    ).toBeChecked();
    await expect(settings.getByLabel(/^Message/)).toHaveText('Reminder');
    await choose(settings.getByLabel('The customer'), 'Opens the email');
    await expect(card.locator('.workflow-node-detail')).toHaveText('Opens the email · up to 1 day');
    await expect(page.locator('.canvas .vue-flow__node[data-id="clicked"]')).toContainText(
      'If they open Reminder',
    );
    await choose(settings.getByLabel('The customer'), 'Clicks a link');
    await page.getByRole('button', { name: 'Validate', exact: true }).click();
    await expect(page.getByRole('status')).toContainText('Workflow is valid');
    await expect
      .poll(
        async () =>
          (
            await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json()
          ).draft.nodes.find((n: { id: string }) => n.id === 'click').config.event_name,
      )
      .toBe('message_clicked');
    journey = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  }

  const run = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: journey.id,
        scenario: {
          name: 'Click gate',
          customer: { customer_id: customer, email: address, consents: { email: true } },
          entry_event: { event_name: 'cart_updated' },
        },
      },
    })
  ).json();
  await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
    headers,
    data: { operation: 'complete', revision: run.revision },
  });
  const published = await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
    headers,
    data: { revision: journey.revision },
  });
  expect(published.ok(), await published.text()).toBeTruthy();
  await page.request.post('/api/v1/customers', {
    headers,
    data: { customer_id: customer, email: address, consents: { email: true } },
  });
  const entry = await (
    await page.request.post('/api/v1/events', {
      headers,
      data: {
        event_id: randomBytes(12).toString('hex'),
        customer_id: customer,
        event_name: 'cart_updated',
        occurred_at: new Date().toISOString(),
      },
    })
  ).json();
  const execution = entry.executions[0];

  // The email reaches the local Mailpit inbox with its link rewritten.
  let html = '';
  await expect
    .poll(
      async () => {
        const found = await (
          await request.get(`${mailpit}/api/v1/search?query=${encodeURIComponent('to:' + address)}`)
        ).json();
        if (!found.messages?.length) return '';
        const message = await (
          await request.get(`${mailpit}/api/v1/message/${found.messages[0].ID}`)
        ).json();
        html = message.HTML;
        return html;
      },
      { timeout: 30000 },
    )
    .toContain('/api/t/');
  expect(html).not.toContain('shop.example');
  expect(html).toContain('Unsubscribe');
  const link = new URL(/href="([^"]*\/api\/t\/[A-Za-z0-9]{12})"/.exec(html)![1]);
  const click = await page.request.get(link.pathname, { maxRedirects: 0 });
  expect(click.status()).toBe(302);
  expect(click.headers()['location']).toBe('https://shop.example/cart');

  await expect
    .poll(async () => {
      const state = await (await page.request.get(`/api/v1/executions/${execution}`)).json();
      return `${state.state} ${state.exit_reason}`;
    })
    .toBe('EXITED converted');
  const delivery = (
    await (await page.request.get(`/api/v1/deliveries?customer_id=${customer}`)).json()
  )[0];
  const detail = await (await page.request.get(`/api/v1/deliveries/${delivery.id}`)).json();
  expect(detail.engagement.map((e: { kind: string }) => e.kind)).toContain('clicked');
});
