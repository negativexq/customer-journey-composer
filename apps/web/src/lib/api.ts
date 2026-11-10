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

export type Identity = {
  id: string;
  workspace_id: string;
  workspace: string;
  email: string;
  name: string;
  role: 'Admin' | 'Editor' | 'Viewer';
  csrf_token: string;
};
export type Journey = {
  id: string;
  name: string;
  description: string;
  tags: string[];
  status: 'Draft' | 'Active' | 'Paused' | 'Sunset' | 'Completed' | 'Archived';
  draft: Record<string, unknown>;
  revision: number;
  updated_at: string;
  version?: number;
  stats?: { entries: number; active: number; completed: number; converted: number };
};
let csrf = '';
export function setIdentity(identity?: Identity) {
  csrf = identity?.csrf_token || '';
}
export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  method = 'GET',
  body?: unknown,
  extraHeaders: Record<string, string> = {},
  timeoutMs = 30000,
  signal?: AbortSignal,
): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    method,
    credentials: 'same-origin',
    signal: signal
      ? AbortSignal.any([signal, AbortSignal.timeout(timeoutMs)])
      : AbortSignal.timeout(timeoutMs),
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...(csrf ? { 'X-CSRF-Token': csrf } : {}),
      ...extraHeaders,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const problem = await response.json().catch(() => ({}));
    throw new APIError(
      response.status,
      problem.detail || problem.title || 'Request failed. Please try again.',
    );
  }
  return response.json() as Promise<T>;
}
