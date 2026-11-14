import { api } from './api';
import type { Workflow } from './workflow';

export type Definition = { version: number; workflow: Workflow };
// Published versions are immutable, so a loaded definition stays valid for the page's lifetime.
const cache = new Map<string, Promise<Definition>>();

export function loadDefinition(journeyID: string, versionID: string) {
  const key = journeyID + ':' + versionID;
  let pending = cache.get(key);
  if (!pending) {
    pending = (async () => {
      const versions = await api<{ id: string; version: number }[]>(
        `/journeys/${journeyID}/versions`,
      );
      const match = versions.find((item) => item.id === versionID);
      if (!match) throw new Error('This journey version is no longer available.');
      const result = await api<{ workflow: Workflow }>(
        `/journeys/${journeyID}/versions/${match.version}`,
      );
      return { version: match.version, workflow: result.workflow };
    })();
    cache.set(key, pending);
    pending.catch(() => cache.delete(key));
  }
  return pending;
}
