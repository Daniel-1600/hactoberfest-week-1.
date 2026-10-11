'use client';
import { useState } from 'react';
import { AudioLines, LoaderCircle, Sparkles } from 'lucide-react';
import { request } from '../lib/api';

type Result = { provider: string; task: string; species_identification: boolean; descriptions?: { start_time?: number; end_time?: number; text: string }[]; disclaimer: string };
export default function NatureLMPanel({ recordingID }: { recordingID: string }) {
  const [result, setResult] = useState<Result | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const describe = async () => {
    setBusy(true); setError('');
    try { setResult(await request<Result>(`/api/recordings/${encodeURIComponent(recordingID)}/naturelm`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ task: 'audio_description' }) })); }
    catch (e) { setError(e instanceof Error ? e.message : 'NatureLM is unavailable.'); }
    finally { setBusy(false); }
  };
  return <div className="naturelm-panel"><button className="text-button" disabled={busy} onClick={() => void describe()}>{busy ? <LoaderCircle size={15} className="spin"/> : <AudioLines size={15}/>} Experimental audio description</button><small>Optional NatureLM-audio analysis; not a verified identification.</small>{error && <p role="status" className="source-note">{error}</p>}{result && <div className="naturelm-result"><p><Sparkles size={15}/>{result.disclaimer}</p>{result.descriptions?.map((item,index) => <blockquote key={`${index}-${item.text}`}>{item.start_time != null && item.end_time != null ? `${item.start_time.toFixed(1)}–${item.end_time.toFixed(1)} sec · ` : ''}{item.text}</blockquote>)}</div>}</div>;
}
