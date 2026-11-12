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
  creation_step?: string;
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
export type GenerationProgress = { stage: string; attempt?: number };

// One POST per user action. Never reconnect or resubmit a paid request implicitly.
export async function generationStream<T>(
  path: string,
  body: unknown,
  onProgress: (progress: GenerationProgress) => void,
  signal: AbortSignal,
): Promise<T> {
  const response = await fetch(`/api/v1${path}/stream`, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      Accept: 'text/event-stream',
      'Content-Type': 'application/json',
      ...(csrf ? { 'X-CSRF-Token': csrf } : {}),
    },
    body: JSON.stringify(body),
    signal: AbortSignal.any([signal, AbortSignal.timeout(90000)]),
  });
  if (!response.ok) {
    const problem = await response.json().catch(() => ({}));
    throw new APIError(response.status, problem.detail || problem.title || 'Generation failed');
  }
  if (!response.body || !response.headers.get('Content-Type')?.includes('text/event-stream'))
    throw new Error(
      'The generation connection returned an unexpected response. Your draft is unchanged.',
    );
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      const { done, value } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      if (buffer.length > 2 * 1024 * 1024)
        throw new Error('Generation response exceeded the size limit');
      let boundary: number;
      while ((boundary = buffer.indexOf('\n\n')) >= 0) {
        const frame = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary + 2);
        if (!frame.startsWith('data: ')) continue;
        const event = JSON.parse(frame.slice(6)) as {
          type: string;
          stage: string;
          attempt?: number;
          proposal?: T;
          message?: string;
          status?: number;
        };
        if (event.type === 'progress') onProgress(event);
        if (event.type === 'error')
          throw new APIError(event.status || 500, event.message || 'Generation failed');
        if (event.type === 'ready' && event.proposal) return event.proposal;
      }
      if (done)
        throw new Error(
          'Generation connection closed before a result arrived. Review before retrying; the provider may already have processed the request.',
        );
    }
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
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
