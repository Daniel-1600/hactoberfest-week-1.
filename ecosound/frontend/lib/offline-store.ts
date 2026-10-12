export type QueuedOperation = {
  operation_id: string;
  type: string;
  payload: unknown;
  created_at: string;
  attempts: number;
  queue_order?: number;
  last_error?: string;
};

const DB_NAME = 'ecosound-offline-v1';
const DB_VERSION = 1;
const CACHE_ENABLED = process.env.NEXT_PUBLIC_OFFLINE_CACHE_ENABLED !== 'false';

function openDB(): Promise<IDBDatabase> {
  if (typeof indexedDB === 'undefined') return Promise.reject(new Error('Offline storage is unavailable in this browser.'));
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);
    request.onupgradeneeded = () => {
      const db = request.result;
      db.createObjectStore('cache', { keyPath: 'key' });
      db.createObjectStore('operations', { keyPath: 'operation_id' });
      db.createObjectStore('recordings', { keyPath: 'id' });
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('Could not open offline storage.'));
  });
}

function requestValue<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('Offline storage request failed.'));
  });
}

export async function cacheRead<T>(key: string): Promise<T | null> {
  if (!CACHE_ENABLED) return null;
  const db = await openDB();
  try {
    const result = await requestValue<{ value: T } | undefined>(db.transaction('cache').objectStore('cache').get(key));
    return result?.value ?? null;
  } finally { db.close(); }
}

export async function cacheWrite(key: string, value: unknown): Promise<void> {
  if (!CACHE_ENABLED) return;
  const db = await openDB();
  try {
    await requestValue(db.transaction('cache', 'readwrite').objectStore('cache').put({ key, value, cached_at: new Date().toISOString() }));
  } finally { db.close(); }
}

export async function enqueueOperation(type: string, payload: unknown, operationID = crypto.randomUUID()): Promise<QueuedOperation> {
  const item: QueuedOperation = { operation_id: operationID, type, payload, created_at: new Date().toISOString(), queue_order: Date.now() * 1000 + (++operationSequence % 1000), attempts: 0 };
  const db = await openDB();
  try { await requestValue(db.transaction('operations', 'readwrite').objectStore('operations').add(item)); }
  finally { db.close(); }
  return item;
}

export async function listOperations(): Promise<QueuedOperation[]> {
  const db = await openDB();
  try {
    return (await requestValue<QueuedOperation[]>(db.transaction('operations').objectStore('operations').getAll())).sort((a, b) => (a.queue_order ?? Date.parse(a.created_at)) - (b.queue_order ?? Date.parse(b.created_at)));
  } finally { db.close(); }
}

let operationSequence = 0;

export async function removeOperation(id: string): Promise<void> {
  const db = await openDB();
  try { await requestValue(db.transaction('operations', 'readwrite').objectStore('operations').delete(id)); }
  finally { db.close(); }
}

export async function updateOperation(item: QueuedOperation): Promise<void> {
  const db = await openDB();
  try { await requestValue(db.transaction('operations', 'readwrite').objectStore('operations').put(item)); }
  finally { db.close(); }
}

export async function queueRecording(file: Blob, name: string, id = crypto.randomUUID()): Promise<string> {
  const db = await openDB();
  try {
    const store = db.transaction('recordings', 'readwrite').objectStore('recordings');
    // A retry of the same upload is already safely queued. Preserve any result.
    if (!await requestValue(store.get(id))) {
      await requestValue(store.add({ id, file, name, created_at: new Date().toISOString(), state: 'pending' }));
    }
  } finally { db.close(); }
  return id;
}

export async function listQueuedRecordings(): Promise<{ id: string; file: Blob; name: string; created_at: string; state: string }[]> {
  const db = await openDB();
  try { return await requestValue(db.transaction('recordings').objectStore('recordings').getAll()); }
  finally { db.close(); }
}

export async function updateQueuedRecording(id: string, state: string, error?: string, result?: unknown): Promise<void> {
  const db = await openDB();
  try {
    const store = db.transaction('recordings', 'readwrite').objectStore('recordings');
    const value = await requestValue<{ id: string; file: Blob; name: string; created_at: string; state: string } | undefined>(store.get(id));
    if (value) await requestValue(store.put({ ...value, state, error, result: result ?? (value as typeof value & { result?: unknown }).result }));
  } finally { db.close(); }
}

export async function removeQueuedRecording(id: string): Promise<void> {
  const db = await openDB();
  try { await requestValue(db.transaction('recordings', 'readwrite').objectStore('recordings').delete(id)); }
  finally { db.close(); }
}
