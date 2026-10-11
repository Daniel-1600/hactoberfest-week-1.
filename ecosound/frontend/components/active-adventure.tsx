'use client';
import { useCallback, useEffect, useState } from 'react';
import { ArrowLeft, Check, Leaf, LoaderCircle, RefreshCw, Sparkles, Volume2 } from 'lucide-react';
import { ApiError, request } from '../lib/api';
import { enqueueOperation, listOperations } from '../lib/offline-store';
import { synchronize } from '../lib/sync';
import { Adventure, AdventureTask, completeTask } from './explore';

export default function ActiveAdventure({ initial, onBack, onRecord, onUpdate }: { initial: Adventure; onBack: () => void; onRecord: () => void; onUpdate: (adventure: Adventure) => void }) {
  const [adventure, setAdventure] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [pending, setPending] = useState(0);
  const [reflection, setReflection] = useState({ discovered: '', surprised: '', again: false, minutes: '' });
  const [finished, setFinished] = useState(initial.status === 'completed');
  const [quietSeconds, setQuietSeconds] = useState<number | null>(null);
  const [vibrateOnEnd, setVibrateOnEnd] = useState(false);
  const syncPending = useCallback(async () => { try { const result = await synchronize(); setPending(result.pending); const refreshed = await request<Adventure>(`/api/adventures/${initial.id}`); setAdventure(refreshed); onUpdate(refreshed); } catch { /* Keep the offline copy while the server is unreachable. */ } }, [initial.id, onUpdate]);
  useEffect(() => { void Promise.resolve().then(syncPending); const online = () => void syncPending(); window.addEventListener('online', online); return () => window.removeEventListener('online', online); }, [syncPending]);
  useEffect(() => {
    if (quietSeconds === null || quietSeconds <= 0) return;
    const timer = window.setTimeout(() => setQuietSeconds(value => value === null ? null : value - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [quietSeconds]);
  useEffect(() => { if (quietSeconds === 0 && vibrateOnEnd && 'vibrate' in navigator) navigator.vibrate(150); }, [quietSeconds, vibrateOnEnd]);
  const start = async () => {
    if (adventure.status !== 'ready') return;
    if (!navigator.onLine) { setError('Connect once to start this adventure; its instructions will then stay available offline.'); return; }
    setBusy(true); setError(''); try { const next = await request<Adventure>(`/api/adventures/${adventure.id}/start`, { method: 'POST' }); setAdventure(next); onUpdate(next); } catch (e) { setError(e instanceof Error ? e.message : 'Could not start adventure.'); } finally { setBusy(false); }
  };
  const markSelfReported = async (task: AdventureTask) => {
    setBusy(true); setError('');
    try { const next = await completeTask(adventure, task, { self_reported: true }); if (next) { setAdventure(next); onUpdate(next); } else { setAdventure({ ...adventure, tasks: adventure.tasks.map(t => t.id === task.id ? { ...t, completed: true, self_reported: true } : t) }); setPending((await listOperations()).length); } }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not update this task.'); } finally { setBusy(false); }
  };
  const finish = async () => {
    setBusy(true); setError('');
    try {
      const payload = { adventure_id: adventure.id, ...(reflection.minutes !== '' ? { self_reported_minutes: Number(reflection.minutes) } : {}), what_did_you_discover: reflection.discovered, what_surprised_you: reflection.surprised, would_explore_again: reflection.again };
      if (navigator.onLine) {
        try { const next = await request<Adventure>(`/api/adventures/${adventure.id}/finish`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) }); setAdventure(next); onUpdate(next); }
        catch (e) { if (!(e instanceof ApiError) || e.status !== 0) throw e; await enqueueOperation('finish_adventure', payload); setPending((await listOperations()).length); }
      } else { await enqueueOperation('finish_adventure', payload); setPending((await listOperations()).length); }
      setFinished(true);
    } catch (e) { setError(e instanceof Error ? e.message : 'Complete your tasks before finishing.'); } finally { setBusy(false); }
  };
  const done = adventure.tasks.filter(t => t.completed).length;
  return <section className="feature-page active-adventure">
    <button className="text-button" onClick={onBack}><ArrowLeft size={16}/> All adventures</button>
    <header className="adventure-active-heading"><span className="pill"><Leaf size={14}/> OUTDOOR ADVENTURE</span><h1>{adventure.title}</h1><p>{adventure.description}</p><span className="subtle-chip">{adventure.duration_minutes} minutes · {done}/{adventure.tasks.length} tasks</span></header>
    {adventure.status === 'ready' && <div className="feature-card"><button className="primary-button" disabled={busy} onClick={() => void start()}>{busy ? <LoaderCircle className="spin" size={17}/> : <Sparkles size={17}/>} Start adventure</button></div>}
    {adventure.status === 'active' && <div className="phone-away"><Volume2 size={19}/><div><strong>Your adventure begins now. Put your phone away and discover what&apos;s around you.</strong><span>Current task: {adventure.tasks.find(task => !task.completed)?.title || 'Your reflections'}</span></div></div>}
    {adventure.status === 'active' && adventure.tasks.some(task => !task.completed && task.type === 'outdoor_observation') && <div className="quiet-timer"><div><strong>{quietSeconds === null ? 'Optional quiet listening pause' : quietSeconds > 0 ? `Quiet pause · ${String(Math.floor(quietSeconds / 60)).padStart(2,'0')}:${String(quietSeconds % 60).padStart(2,'0')}` : 'Quiet pause complete'}</strong><small>Background timers may pause in your browser; this pause does not complete or verify a task.</small></div>{quietSeconds === null || quietSeconds === 0 ? <button className="secondary-button" onClick={() => setQuietSeconds(5 * 60)}>Start five-minute timer</button> : <button className="secondary-button" onClick={() => setQuietSeconds(null)}>Stop timer</button>}<label className="checkbox-row"><input type="checkbox" checked={vibrateOnEnd} onChange={e => setVibrateOnEnd(e.target.checked)}/> Vibrate at the end if this browser supports it</label></div>}
    <div className="task-stack">{adventure.tasks.map((task, index) => <article className={`feature-card adventure-task ${task.completed ? 'is-done' : ''}`} key={task.id}><span className="task-count">{task.completed ? <Check size={18}/> : String(index + 1).padStart(2,'0')}</span><div className="feature-grow"><div className="adventure-title"><h3>{task.title}</h3><span className="xp-label">{task.xp} XP</span></div><p>{task.instructions}</p>{task.completed && task.type === 'outdoor_observation' && <small>Self-reported task completed</small>}{task.self_reported && <small>Self-reported activity · not independently verified</small>}{!task.completed && task.type === 'species_discovery' && <small>Use a separate saved recording; one clip can earn credit for only one task.</small>}</div>{!task.completed && adventure.status === 'active' && task.type === 'audio_discovery' && <button className="primary-button compact-button" onClick={onRecord}>Record wildlife</button>}{!task.completed && adventure.status === 'active' && task.type === 'species_discovery' && <button className="primary-button compact-button" onClick={onRecord}>Record another call</button>}{!task.completed && adventure.status === 'active' && ['outdoor_observation','reflection'].includes(task.type) && <button disabled={busy} className="secondary-button compact-button" onClick={() => void markSelfReported(task)}>Mark complete</button>}</article>)}</div>
    {adventure.status === 'active' && done === adventure.tasks.length && !finished && <div className="feature-card reflection-card"><span className="eyebrow">A MOMENT TO REFLECT</span><h2>What will you remember?</h2><p>Your answers are optional. Time outdoors is self-reported, not measured activity.</p><label>What did you discover?<textarea maxLength={240} value={reflection.discovered} onChange={e => setReflection({ ...reflection, discovered: e.target.value })}/></label><label>What surprised you?<textarea maxLength={240} value={reflection.surprised} onChange={e => setReflection({ ...reflection, surprised: e.target.value })}/></label><label>Outdoor minutes (optional, self-reported)<input type="number" min={0} max={240} value={reflection.minutes} onChange={e => setReflection({ ...reflection, minutes: e.target.value })}/></label><label className="checkbox-row"><input type="checkbox" checked={reflection.again} onChange={e => setReflection({ ...reflection, again: e.target.checked })}/> I would explore again</label><button disabled={busy} className="primary-button" onClick={() => void finish()}>Finish adventure <Check size={16}/></button></div>}
    {(finished || adventure.status === 'completed') && <div className="quest-complete-banner"><Sparkles size={20}/><span>Adventure complete! You explored the natural world today.</span></div>}
    {pending > 0 && <div className="offline-notice" role="status">{pending} offline action{pending === 1 ? '' : 's'} waiting to sync. <button className="text-button" onClick={() => void syncPending()}><RefreshCw size={14}/> Sync now</button></div>}{error && <p className="error-box" role="alert">{error}</p>}
  </section>;
}
