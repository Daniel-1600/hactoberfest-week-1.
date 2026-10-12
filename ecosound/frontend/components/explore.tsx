'use client';
import { useEffect, useState } from 'react';
import { ArrowRight, Compass, LoaderCircle, RefreshCw, Sparkles, TreePine, AudioLines, Leaf, Clock, Shuffle, Feather, Footprints } from 'lucide-react';
import { ApiError, request } from '../lib/api';
import { enqueueOperation } from '../lib/offline-store';

export type AdventureTask = { id: string; type: string; title: string; instructions: string; xp: number; completed: boolean; self_reported?: boolean };
export type Adventure = { id: string; title: string; description: string; duration_minutes: number; interest: string; environment: string; difficulty: string; generation_source: string; status: string; tasks: AdventureTask[]; self_reported_minutes?: number };
type Preferences = { duration_minutes: number; interest: string; environment: string; difficulty: string; accessibility?: string; destination_id?: string };
type DestinationChoice = { id: string; name: string; description?: string; status: string };
const initial: Preferences = { duration_minutes: 15, interest: 'birdwatching', environment: 'park', difficulty: 'beginner' };
const missions = [
  { name: 'Sound safari', description: 'Follow your ears. Meet the possible singers around you.', icon: AudioLines, preferences: initial },
  { name: 'Slow & curious', description: 'Take a breath. Notice the little things you usually miss.', icon: Leaf, preferences: { ...initial, interest: 'mindfulness', environment: 'garden', duration_minutes: 10 } },
  { name: 'The nearby wild', description: 'Turn an ordinary walk into a small discovery.', icon: Footprints, preferences: { ...initial, interest: 'exploration', environment: 'neighborhood', duration_minutes: 30 } },
];

export default function Explore({ onStart, onDestinations }: { onStart: (adventure: Adventure) => void; onDestinations: () => void }) {
  const [preferences, setPreferences] = useState(initial);
  const [adventures, setAdventures] = useState<Adventure[]>([]);
  const [destinations, setDestinations] = useState<DestinationChoice[]>([]);
  const [conceptDestination, setConceptDestination] = useState<DestinationChoice | null>(null);
  const [busy, setBusy] = useState(false);
  const [selectedMission, setSelectedMission] = useState('Sound safari');
  const [surpriseMessage, setSurpriseMessage] = useState('');
  const chooseMission = (index: number) => { const mission = missions[index]; setPreferences({ ...mission.preferences }); setSelectedMission(mission.name); setSurpriseMessage(''); };
  const [error, setError] = useState('');
  const [offline, setOffline] = useState(typeof navigator !== 'undefined' && !navigator.onLine);
  const load = async () => {
    try { const result = await request<{ adventures: Adventure[] }>('/api/adventures'); setAdventures(result.adventures || []); setError(''); }
    catch (e) { setError(e instanceof Error ? e.message : 'Adventure history is unavailable.'); }
  };
  useEffect(() => { void Promise.resolve().then(load); const update = () => setOffline(!navigator.onLine); window.addEventListener('online', update); window.addEventListener('offline', update); return () => { window.removeEventListener('online', update); window.removeEventListener('offline', update); }; }, []);
  useEffect(() => { void request<{ destinations: DestinationChoice[] }>('/api/destinations').then(data => { setDestinations(data.destinations.filter(item => item.status === 'verified')); setConceptDestination(data.destinations.find(item => item.status === 'concept') || null); }).catch(() => undefined); }, []);
  const generate = async () => {
    setBusy(true); setError('');
    try { const adventure = await request<Adventure>('/api/adventures/generate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(preferences) }); setAdventures(old => [adventure, ...old]); onStart(adventure); }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not create an adventure.'); }
    finally { setBusy(false); }
  };
  const resume = async (item: Adventure) => { try { const updated = await request<Adventure>(`/api/adventures/${item.id}`); onStart(updated); } catch { onStart(item); } };
  return <section className="feature-page">
    <header className="welcome"><span className="eyebrow"><Compass size={16}/> SMALL MISSIONS. REAL DISCOVERIES.</span><h1>Your next little adventure.</h1><p>Pick your mood and a little time. We’ll turn them into a nature mission you can take outside.</p></header>
    <div className="mission-choices" role="group" aria-label="Choose a mission mood">{missions.map((mission, index) => { const Icon = mission.icon; return <button key={mission.name} className={`mission-choice ${selectedMission === mission.name ? 'selected' : ''}`} aria-pressed={selectedMission === mission.name} disabled={busy} onClick={() => chooseMission(index)}><Icon size={25}/><strong>{mission.name}</strong><span>{mission.description}</span><small>{mission.preferences.duration_minutes} min starter <ArrowRight size={13}/></small></button>; })}</div>
    <div className="mission-builder">
    <div className="feature-card adventure-form"><div className="feature-card-heading"><span className="feature-icon"><Sparkles size={21}/></span><div><h2>A little time is all it takes.</h2><p>Start with a mood above, or make the mission your own.</p></div></div>
      <fieldset className="mission-time" disabled={busy}><legend><Clock size={15}/> How long have you got?</legend><div>{[10,15,30,60].map(n => <button type="button" key={n} aria-pressed={preferences.duration_minutes === n} className={preferences.duration_minutes === n ? 'selected' : ''} onClick={() => setPreferences({ ...preferences, duration_minutes: n })}>{n}<span>min</span></button>)}</div></fieldset>
      <div className="form-grid">
      <label>Interest<select value={preferences.interest} onChange={e => setPreferences({ ...preferences, interest: e.target.value })}>{[['wildlife','Wildlife'],['birdwatching','Birdwatching'],['exploration','Exploration'],['mindfulness','Mindfulness'],['photography','Photography'],['environmental_education','Environmental education']].map(([v,l]) => <option key={v} value={v}>{l}</option>)}</select></label>
      <label>Environment<select value={preferences.environment} onChange={e => setPreferences({ ...preferences, environment: e.target.value, destination_id: '' })}>{[['forest','Forest'],['park','Park'],['garden','Garden'],['neighborhood','Neighborhood'],['university_campus','University campus'],['approved_nature_destination','Approved nature destination']].map(([v,l]) => <option key={v} value={v}>{l}</option>)}</select></label>
      <label>Experience<select value={preferences.difficulty} onChange={e => setPreferences({ ...preferences, difficulty: e.target.value })}>{[['beginner','Beginner'],['intermediate','Intermediate'],['experienced','Experienced']].map(([v,l]) => <option key={v} value={v}>{l}</option>)}</select></label></div>
      <label className="accessibility-field">Accessibility preference (optional)<input maxLength={240} value={preferences.accessibility || ''} onChange={e => setPreferences({ ...preferences, accessibility: e.target.value })} placeholder="For example: short, level paths"/></label>
      {preferences.environment === 'approved_nature_destination' && <label>Verified destination<select value={preferences.destination_id || ''} onChange={e => setPreferences({ ...preferences, destination_id: e.target.value })}><option value="">Choose an approved destination</option>{destinations.map(item => <option value={item.id} key={item.id}>{item.name}</option>)}</select>{!destinations.length && <small>No verified destinations are configured. EcoSound will not generate trails or access claims.</small>}</label>}
      <p className="source-note">{offline ? 'You are offline. Saved adventures remain available; a new mission needs a connection.' : 'You’ll preview your tasks before starting. If your guide is unavailable, we’ll offer a ready-made mission.'}</p>
      <div className="mission-actions"><button className="primary-button" disabled={busy || offline || preferences.environment === 'approved_nature_destination' && !preferences.destination_id} onClick={() => void generate()}>{busy ? <LoaderCircle size={18} className="spin"/> : <Sparkles size={18}/>} {busy ? 'Finding your next adventure…' : 'Build my nature mission'} {!busy && <ArrowRight size={16}/>}</button><button className="text-button" disabled={busy} onClick={() => { const choices = missions.map((_, i) => i).filter(i => missions[i].name !== selectedMission); const index = choices[Math.floor(Math.random() * choices.length)]; chooseMission(index); setSurpriseMessage(`${missions[index].name} picked! Adjust anything, then build your mission.`); }}><Shuffle size={15}/> Surprise me</button></div>
      {surpriseMessage && <p className="source-note" role="status">{surpriseMessage}</p>}
      {busy && <p className="mission-loading" role="status">Your guide is putting together your tasks. The first mission can take a few minutes while it wakes up.</p>}
      {error && <div className="error-box" role="alert">{error}<button className="text-button" disabled={busy || offline} onClick={() => void generate()}><RefreshCw size={14}/>Try building again</button></div>}
    </div>
    <aside className="mission-preview"><span className="eyebrow">A SMALL ADVENTURE, A REAL MOMENT</span><h2>Less scrolling.<br/>More discovering.</h2><p>A few simple tasks. A reason to pause. Something to bring back with you.</p><ol><li><span><Compass size={18}/></span><div><strong>Get your mission</strong><p>A short set of tasks shaped around your time and surroundings.</p></div></li><li><span><AudioLines size={18}/></span><div><strong>Listen, look, explore</strong><p>Head to a place you can safely access. Capture a call or notice something new.</p></div></li><li><span><Feather size={18}/></span><div><strong>Keep the discovery</strong><p>Save bird suggestions, finish tasks and collect XP in your passport.</p></div></li></ol><div className="mission-preview-footer"><Leaf size={16}/> Leave the wildlife wild. Curiosity is enough.</div></aside>
    </div>
    {conceptDestination && <article className="feature-card featured-destination"><span className="eyebrow"><TreePine size={15}/> FEATURED ROUTE-FREE CONCEPT</span><h2>{conceptDestination.name} — Hidden Voices Adventure</h2><p>{conceptDestination.description}</p><button className="secondary-button" onClick={onDestinations}>Read destination experience <ArrowRight size={15}/></button></article>}
    <div className="section-heading"><div><span className="eyebrow">YOUR NATURE JOURNAL</span><h2>Adventure history</h2></div></div>
    {!adventures.length ? <div className="empty-state"><span className="empty-icon"><TreePine size={31}/></span><h3>Your next outdoor story starts here.</h3><p>Build your first mission above. You can come back here to finish it.</p></div> : <div className="feature-list">{adventures.map(item => <article className="feature-card adventure-row" key={item.id}><span className="feature-icon"><TreePine size={22}/></span><div className="feature-grow"><div className="adventure-title"><h3>{item.title}</h3><span className={`status-badge ${item.generation_source === 'qwen' ? 'confirmed' : 'predicted'}`}>{item.generation_source === 'qwen' ? 'Personalized mission' : item.generation_source === 'destination' ? 'Destination mission' : 'Ready-made mission'}</span></div><p>{item.description}</p><small>{item.duration_minutes} minutes · {item.tasks.filter(t => t.completed).length}/{item.tasks.length} tasks · {item.status}</small></div><button className="secondary-button" onClick={() => void resume(item)}>{item.status === 'active' ? 'Resume' : item.status === 'completed' ? 'Review' : 'Preview'} <ArrowRight size={15}/></button></article>)}</div>}
  </section>;
}

export async function completeTask(adventure: Adventure, task: AdventureTask, detail: { discovery_id?: number; self_reported?: boolean } = {}) {
  const payload = { adventure_id: adventure.id, task_id: task.id, ...detail };
  if (!navigator.onLine) { await enqueueOperation('complete_adventure_task', payload); return null; }
  try { return await request<Adventure>(`/api/adventures/${adventure.id}/tasks/${task.id}/complete`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(detail) }); }
  catch (error) { if (error instanceof ApiError && error.status === 0) { await enqueueOperation('complete_adventure_task', payload); return null; } throw error; }
}
