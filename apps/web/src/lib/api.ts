export type Health = { status: 'ok' | 'ready'; service: string };

export async function getReadiness(signal?: AbortSignal): Promise<Health> {
  const response = await fetch('/readyz', { signal, headers: { Accept: 'application/json' } });
  if (!response.ok) {
    throw new Error('The API or database is not ready. Check the local services and try again.');
  }
  const body: unknown = await response.json();
  if (
    typeof body !== 'object' ||
    body === null ||
    !('status' in body) ||
    body.status !== 'ready' ||
    !('service' in body) ||
    typeof body.service !== 'string'
  ) {
    throw new Error('The API returned an unexpected health response.');
  }
  return { status: body.status, service: body.service };
}
