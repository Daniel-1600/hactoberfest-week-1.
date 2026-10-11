import { request } from './api';
import { enqueueOperation, listOperations, listQueuedRecordings, removeOperation, updateOperation, updateQueuedRecording, type QueuedOperation } from './offline-store';

export async function synchronize(): Promise<{ sent: number; pending: number }> {
  if (!navigator.onLine) return { sent: 0, pending: (await listOperations()).length + (await listQueuedRecordings()).filter(item => item.state === 'pending').length };
  let sent = 0;
  for (let round = 0; round < 10; round++) {
    const queued = await listOperations();
    if (!queued.length) break;
    let removed = 0;
    for (let i = 0; i < queued.length; i += 50) {
    const batch = queued.slice(i, i + 50);
    let outcomes: { operation_id: string; status: string; error?: string; result?: unknown }[];
    try {
      const response = await request<{ results: typeof outcomes }>('/api/sync', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ operations: batch.map(({ operation_id, type, payload }) => ({ operation_id, type, payload })) }) });
      outcomes = response.results;
    } catch { return { sent, pending: (await listOperations()).length + (await listQueuedRecordings()).filter(item => item.state === 'pending').length }; }
    const byID = new Map(outcomes.map(item => [item.operation_id, item]));
    for (const operation of batch) {
      const outcome = byID.get(operation.operation_id);
      if (outcome?.status === 'applied' || outcome?.status === 'already_applied') {
        await removeOperation(operation.operation_id); sent++; removed++;
        if (operation.type === 'save_discovery' && outcome.status === 'applied') await enqueueLinkedTask(outcome.result);
      }
      else if (outcome) { await updateOperation({ ...operation, attempts: operation.attempts + 1, last_error: outcome.error }); }
    }
    }
    if (!removed) break;
  }
  await synchronizeRecordings();
  return { sent, pending: (await listOperations()).length + (await listQueuedRecordings()).filter(item => item.state === 'pending').length };
}

async function enqueueLinkedTask(result: unknown): Promise<void> {
  if (!result || typeof result !== 'object' || !('id' in result) || typeof result.id !== 'number') return;
  try {
    const raw = localStorage.getItem('ecosound-active-adventure');
    if (!raw) return;
    const adventure = JSON.parse(raw) as { id: string; status: string; tasks: { id: string; type: string; completed: boolean }[] };
    if (adventure.status !== 'active') return;
    const task = adventure.tasks.find(item => !item.completed && item.type === 'audio_discovery') || adventure.tasks.find(item => !item.completed && item.type === 'species_discovery');
    if (task) await enqueueOperation('complete_adventure_task', { adventure_id: adventure.id, task_id: task.id, discovery_id: result.id, self_reported: false });
  } catch { /* Local adventure state can be absent or unavailable. */ }
}

export async function synchronizeRecordings(): Promise<number> {
  if (!navigator.onLine) return 0;
  const recordings = await listQueuedRecordings(); let count = 0;
  for (const recording of recordings.filter(item => item.state === 'pending')) {
    try {
      const form = new FormData(); form.append('audio', new File([recording.file], recording.name, { type: recording.file.type || 'audio/webm' }));
      const result = await request('/api/identify', { method: 'POST', headers: { 'Idempotency-Key': recording.id }, body: form });
      await updateQueuedRecording(recording.id, 'identified_review_pending', undefined, result); count++;
    } catch (error) {
      await updateQueuedRecording(recording.id, 'pending', error instanceof Error ? error.message : 'Identification is still waiting for a connection.');
      break;
    }
  }
  return count;
}

export function freshOperation(type: string, payload: unknown): QueuedOperation {
  return { operation_id: crypto.randomUUID(), type, payload, created_at: new Date().toISOString(), attempts: 0 };
}
