'use client';
import { ChangeEvent, useEffect, useRef, useState } from 'react';
import { ArrowRight, AudioLines, Check, Circle, Feather, LoaderCircle, Mic, ShieldCheck, Sparkles, Square, Upload, Volume2 } from 'lucide-react';
import { ApiError, audioUrl, Discovery, Identification, MAX_AUDIO_BYTES, request, strongestCandidates } from '../lib/api';
import { useRecorder } from '../lib/use-recorder';
import NatureGuide from './nature-guide';
import NatureLMPanel from './naturelm-panel';
import { enqueueOperation, listQueuedRecordings, queueRecording } from '../lib/offline-store';
import { synchronize, synchronizeRecordings } from '../lib/sync';

export default function Listen({ onSaved, onCollection }: { onSaved: (discoveryId: number) => Promise<void>; onCollection: () => void }) {
  const recorder = useRecorder();
  const playbackRef = useRef<HTMLAudioElement>(null);
  const [result, setResult] = useState<Identification | null>(null);
  const [analyzing, setAnalyzing] = useState(false);
  const [saving, setSaving] = useState<number | null>(null);
  const [saved, setSaved] = useState(false);
  const [queuedSave, setQueuedSave] = useState(false);
  const [savedDiscoveryID, setSavedDiscoveryID] = useState<number | undefined>();
  const [savedPredictionID, setSavedPredictionID] = useState<number | undefined>();
  const [queuedRecordings, setQueuedRecordings] = useState<{ id: string; name: string; created_at: string; state: string; result?: Identification; error?: string }[]>([]);
  const [offlineMessage, setOfflineMessage] = useState('');
  const [selectedLocalID, setSelectedLocalID] = useState('');
  const uploadKeyRef = useRef('');
  const [error, setError] = useState('');
  const [saveError, setSaveError] = useState('');
  useEffect(() => {
    if (!recorder.file) return;
    const url = URL.createObjectURL(recorder.file);
    if (playbackRef.current) playbackRef.current.src = url;
    return () => URL.revokeObjectURL(url);
  }, [recorder.file]);
  const refreshQueued = async () => {
    try { setQueuedRecordings(await listQueuedRecordings() as { id: string; name: string; created_at: string; state: string; result?: Identification; error?: string }[]); }
    catch { /* Offline storage is optional on unsupported browsers. */ }
  };
  useEffect(() => {
    void Promise.resolve().then(refreshQueued);
    const online = () => { void synchronize().then(() => { setQueuedSave(false); void refreshQueued(); void onSaved(0); }).catch(() => undefined); };
    window.addEventListener('online', online);
    return () => window.removeEventListener('online', online);
  }, [onSaved]);
  const clearResult = () => { setResult(null); setSaved(false); setQueuedSave(false); setSavedDiscoveryID(undefined); setSavedPredictionID(undefined); setError(''); setSaveError(''); };
  const upload = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (file.size > MAX_AUDIO_BYTES) { recorder.setError('Choose an audio file smaller than 15 MiB.'); return; }
    if (!/\.(wav|webm|mp4|m4a|ogg|mp3|flac)$/i.test(file.name)) { recorder.setError('Choose a WAV, WebM, MP4/M4A, Ogg, MP3, or FLAC audio file.'); return; }
    clearResult(); uploadKeyRef.current = ''; setOfflineMessage(''); recorder.setError(''); recorder.setFile(file);
  };
  const analyze = async () => {
    if (!recorder.file || analyzing) return;
    setAnalyzing(true); setError(''); setOfflineMessage(''); setResult(null); setSaved(false); setQueuedSave(false); setSavedDiscoveryID(undefined); setSavedPredictionID(undefined); setSaveError('');
    try {
      if (!uploadKeyRef.current) uploadKeyRef.current = crypto.randomUUID();
      const idempotencyKey = uploadKeyRef.current;
      if (!navigator.onLine) {
        await queueRecording(recorder.file, recorder.file.name || 'outdoor-recording.webm', idempotencyKey);
        setOfflineMessage('Recording safely stored on this device. BirdNET identification needs a connection; EcoSound will upload it when you reconnect.');
        await refreshQueued(); return;
      }
      const form = new FormData(); form.append('audio', recorder.file);
      setResult(await request<Identification>('/api/identify', { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: form }));
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 0 && recorder.file) {
        try { if (!uploadKeyRef.current) uploadKeyRef.current = crypto.randomUUID(); await queueRecording(recorder.file, recorder.file.name || 'outdoor-recording.webm', uploadKeyRef.current); setOfflineMessage('Recording safely stored on this device. Identification will run when EcoSound reconnects.'); await refreshQueued(); }
        catch { setError('Connection failed and this browser could not store the recording offline. Keep the original audio file and try again when connected.'); }
      } else setError(cause instanceof Error ? cause.message : 'Analysis failed. Please try again.');
    }
    finally { setAnalyzing(false); }
  };
  const save = async (predictionId: number) => {
    if (saved || saving !== null) return;
    setSaving(predictionId); setSaveError('');
    try {
      if (!navigator.onLine) {
        await enqueueOperation('save_discovery', { prediction_id: predictionId });
        setSaved(true); setQueuedSave(true); setSaveError('Discovery queued on this device. It will be saved after synchronization; no quest credit is awarded until the server accepts it.'); await refreshQueued(); return;
      }
      const discovery = await request<Discovery>('/api/discoveries', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ prediction_id: predictionId }) });
      setSaved(true); setSavedDiscoveryID(discovery.id); setSavedPredictionID(predictionId);
      if (selectedLocalID) { const { removeQueuedRecording } = await import('../lib/offline-store'); await removeQueuedRecording(selectedLocalID); setSelectedLocalID(''); await refreshQueued(); }
      await onSaved(discovery.id);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 409) {
        setSaved(true);
        setSaveError('A discovery from this recording is already in your collection. Each recording can be saved once.');
        await onSaved(0);
      } else if (cause instanceof ApiError && cause.status === 0) {
        try { await enqueueOperation('save_discovery', { prediction_id: predictionId }); setSaved(true); setQueuedSave(true); setSaveError('Connection dropped. Discovery queued for synchronization; quest progress updates when the server accepts it.'); await refreshQueued(); }
        catch { setSaveError('Could not reach the server or save this action offline. Try again when connected.'); }
      } else setSaveError(cause instanceof Error ? cause.message : 'Could not save your discovery. Please try again.');
    } finally { setSaving(null); }
  };
  const recording = recorder.status === 'recording';
  const busy = recorder.status !== 'idle' || analyzing || saving !== null;
  const candidates = result ? strongestCandidates(result.predictions) : [];
  return <>
    <section className="welcome"><span className="eyebrow"><AudioLines size={16}/> LISTEN & IDENTIFY</span><h1>Who’s singing out there?</h1><p>Find a quiet spot, take a breath, and let the birds do the talking.</p></section>
    <div className="listen-layout"><section className="recorder-card" aria-label="Audio recorder"><div className="recorder-heading"><span className="pill"><Circle size={9} fill="currentColor"/> {recording ? 'RECORDING' : 'YOUR NEXT DISCOVERY'}</span><span className="muted">60 seconds max</span></div><div className={`recorder-visual ${recording ? 'is-recording' : ''}`}><div className="recorder-ring"><span className="recorder-circle"><Mic size={43} strokeWidth={1.5}/></span></div><div className="waveform" aria-hidden="true">{Array.from({ length: 25 }, (_, index) => <span key={index} style={{ height: `${10 + ((index * 17 + 9) % 37)}px`, animationDelay: `${index * 50}ms` }}/>)}</div></div>
      <h2>{recording ? 'Listening to your world…' : recorder.status === 'permission' ? 'Waiting for your microphone…' : recorder.file ? 'A wild moment, captured.' : 'A good discovery starts with a listen.'}</h2><p>{recording ? 'Stay still and let the sounds come to you.' : 'Record a clear bird call or upload an audio file.'}</p>
      {recording && <div className="recording-time" role="timer" aria-label="Recording time">{String(Math.floor(recorder.seconds / 60)).padStart(2, '0')}:{String(recorder.seconds % 60).padStart(2, '0')} <span>/ 01:00</span></div>}
      <div className="recorder-actions">{recording ? <button className="stop-button" onClick={recorder.stop}><Square size={16} fill="currentColor"/> Stop recording</button> : <button className="primary-button" disabled={busy} onClick={() => { clearResult(); void recorder.start(); }}>{recorder.status === 'permission' ? <LoaderCircle size={18} className="spin"/> : <Mic size={18}/>} {recorder.status === 'permission' ? 'Opening microphone…' : recorder.file ? 'Record again' : 'Start recording'}</button>}</div>
      <p className="permission-note"><ShieldCheck size={13}/> Microphone access is requested when you start recording.</p>
      {recorder.error && <p className="error-box" role="alert">{recorder.error}</p>}
      <div className="upload-divider"><span/>or bring a sound you’ve captured<span/></div><label className={`upload-button ${busy ? 'disabled' : ''}`}><Upload size={17}/> Upload audio<input type="file" aria-label="Upload audio" accept=".wav,.webm,.mp4,.m4a,.ogg,.mp3,.flac,audio/*" onChange={upload} disabled={busy}/></label><p className="file-note">WAV, WebM, MP4/M4A, Ogg, MP3, FLAC · 1–60 sec · up to 15 MiB</p>
      {offlineMessage && <p className="offline-notice" role="status">{offlineMessage}</p>}
      {!!queuedRecordings.length && <div className="queued-recordings"><strong>On this device · {queuedRecordings.length}</strong><p>Unidentified clips wait for a server connection. They are private to this browser.</p>{queuedRecordings.map(item => <div key={item.id}><span>{item.name} · {item.state === 'identified_review_pending' ? 'ready to review' : 'waiting to upload'}</span>{item.result && <button className="text-button" onClick={() => { setResult(item.result || null); setSaved(false); setSavedDiscoveryID(undefined); setSelectedLocalID(item.id); }}>Review identification</button>}</div>)}<button className="text-button" onClick={() => { void synchronizeRecordings().then(refreshQueued); }}>Try synchronization</button></div>}
      {recorder.file && !recording && <div className="audio-preview"><div><Volume2 size={16}/><span>{recorder.file.name}</span><span className="muted">{(recorder.file.size / 1024 / 1024).toFixed(2)} MiB</span></div><audio ref={playbackRef} controls aria-label="Play your recorded audio"/><button className="primary-button full-width" onClick={() => void analyze()} disabled={busy}>{analyzing ? <LoaderCircle size={18} className="spin"/> : <Sparkles size={18}/>} {analyzing ? 'Listening with BirdNET…' : 'Identify this sound'}{!analyzing && <ArrowRight size={16}/>}</button></div>}
      {error && <p className="error-box" role="alert">{error}</p>}
    </section><aside className="listening-tips"><span className="tips-icon"><LeafIcon/></span><span className="eyebrow">MAKE ROOM FOR NATURE</span><h3>A few little listening tips</h3><ul><li><span>01</span><div><strong>Find your quiet corner</strong><p>Step away from traffic, conversations, and music.</p></div></li><li><span>02</span><div><strong>Hold still, listen longer</strong><p>Keep your phone steady. A few clear calls help more than a quick snippet.</p></div></li><li><span>03</span><div><strong>Let the wildlife be wild</strong><p>Keep your distance and avoid playing calls to attract birds.</p></div></li></ul><div className="tips-note"><Sparkles size={17}/><p>Powered by BirdNET. AI offers possibilities; your curiosity does the rest.</p></div></aside></div>
    {analyzing && <section className="analysis-status" role="status"><LoaderCircle size={25} className="spin"/><div><h3>Listening for something wonderful…</h3><p>BirdNET is analyzing your audio. This may take a moment.</p></div></section>}
    {result && <section className="results-section" aria-label="Identification results"><div className="section-heading"><div><span className="eyebrow">{candidates.length ? 'POSSIBLE SINGERS' : 'KEEP YOUR CURIOSITY'}</span><h2>{candidates.length ? 'Here’s what we heard.' : 'No bird calls identified this time.'}</h2></div><span className="subtle-chip">{result.duration_seconds.toFixed(1)} sec · {result.model} {result.model_version}</span></div>
      {!candidates.length ? <div className="empty-state compact"><Feather size={29}/><p>Try a clearer recording with less background noise, or listen a little longer. Nature doesn’t always sing on cue.</p></div> : <><p className="results-caveat"><Sparkles size={15}/> AI identification scores are model suggestions, not guaranteed accuracy. Save one candidate per recording.</p><div className="candidate-grid">{candidates.map((candidate, index) => <article className="candidate-card" key={candidate.id}><div className="candidate-top"><span className="bird-icon"><Feather size={25}/></span><span className="status-badge predicted">AI predicted · {index === 0 ? 'Top suggestion' : 'Also possible'}</span></div><h3>{candidate.common_name || candidate.scientific_name || candidate.raw_species_name || 'Unidentified bird'}</h3><p className="scientific-name">{candidate.scientific_name}</p><div className="score-line"><Sparkles size={14}/> AI identification score <strong>{Math.round(candidate.confidence * 100)}%</strong></div><div className="progress-track score-track"><span style={{ width: `${candidate.confidence * 100}%` }}/></div><p className="candidate-time">{candidate.start_time !== null && candidate.end_time !== null ? `Heard at ${candidate.start_time.toFixed(1)}–${candidate.end_time.toFixed(1)} sec` : 'Call timing unavailable'}</p><button className={saved ? 'secondary-button full-width' : 'primary-button full-width'} disabled={saved || saving !== null} onClick={() => void save(candidate.id)}>{saved ? <Check size={16}/> : saving === candidate.id ? <LoaderCircle size={16} className="spin"/> : <BookIcon/>}{saved ? queuedSave ? 'Queued to sync' : 'Recording saved' : saving === candidate.id ? 'Saving…' : 'Save discovery'}</button><NatureGuide scientificName={candidate.scientific_name} discoveryId={savedPredictionID === candidate.id ? savedDiscoveryID : undefined}/></article>)}</div></>}
      {saveError && <p className="error-box" role="alert">{saveError}</p>}{saved && <div className="saved-message" role="status"><Check size={21}/><div><strong>Your wild moment is in your collection.</strong><p>Quest progress is refreshed from your saved discoveries.</p></div><button className="text-button" onClick={onCollection}>View collection <ArrowRight size={16}/></button></div>}
      <NatureLMPanel recordingID={result.recording_id}/>
      <div className="result-audio"><Volume2 size={16}/><span>Listen to the analyzed recording</span><audio controls preload="none" src={audioUrl(result.audio_url)} aria-label="Play analyzed recording"/></div>
    </section>}
  </>;
}
function LeafIcon() { return <Feather size={28}/>; }
function BookIcon() { return <Check size={16}/>; }
