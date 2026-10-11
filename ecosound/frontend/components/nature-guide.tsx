'use client';
import { useState } from 'react';
import { BookOpen, LoaderCircle, Leaf, ShieldCheck } from 'lucide-react';
import { request } from '../lib/api';

type Record = { scientific_name: string; common_name: string; habitat: string; diet: string; ecological_role: string; conservation_information: string; ethical_guidance: string; source_name: string; source_url: string; source_license: string; explanation?: string; outdoor_activities?: string[]; disclaimer?: string };
export default function NatureGuide({ scientificName, discoveryId }: { scientificName: string; discoveryId?: number }) {
  const [record, setRecord] = useState<Record | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const learn = async () => {
    setBusy(true); setError('');
    try {
      const source = await request<Record>(`/api/species/${encodeURIComponent(scientificName)}`);
      if (discoveryId) {
        try { setRecord(await request<Record>(`/api/discoveries/${discoveryId}/explain`, { method: 'POST' })); }
        catch { setRecord(source); setError('Curated facts are available. The local Qwen explanation service is not available right now.'); }
      } else setRecord(source);
    } catch (e) { setError(e instanceof Error ? e.message : 'No curated information is available for this species yet.'); }
    finally { setBusy(false); }
  };
  return <div className="guide-wrap"><button className="text-button guide-button" disabled={busy} onClick={() => void learn()}>{busy ? <LoaderCircle size={15} className="spin"/> : <BookOpen size={15}/>} Learn about this species</button>{error && <p className="source-note" role="status">{error}</p>}{record && <section className="species-guide"><div className="guide-heading"><Leaf size={18}/><div><strong>{record.common_name}</strong><em>{record.scientific_name}</em></div></div>{record.disclaimer && <p className="source-note">{record.disclaimer}</p>}{record.explanation && <p className="guide-explanation">{record.explanation}</p>}{record.outdoor_activities?.length ? <div className="guide-activities"><strong>Try outdoors</strong><ul>{record.outdoor_activities.map((activity, index) => <li key={`${index}-${activity}`}>{activity}</li>)}</ul></div> : null}<dl><dt>Habitat</dt><dd>{record.habitat}</dd><dt>Diet</dt><dd>{record.diet}</dd><dt>Ecological role</dt><dd>{record.ecological_role}</dd><dt>Conservation</dt><dd>{record.conservation_information}</dd></dl><p className="ethical-note"><ShieldCheck size={15}/>{record.ethical_guidance}</p><a href={record.source_url} target="_blank" rel="noreferrer">Source: {record.source_name} · {record.source_license}</a><small>AI-written text, when shown, summarizes this cited record and may omit context. A model prediction is not scientific verification.</small></section>}</div>;
}
