'use client';
import { useEffect, useState } from 'react';
import { ArrowRight, Compass, Leaf, MapPin, Route } from 'lucide-react';
import { request } from '../lib/api';
import type { Adventure } from './explore';

type Destination = { id: string; name: string; description: string; type: string; region: string; country: string; website?: string; status: string };
type Trail = { id: string; name: string; description: string; distance_km?: number; estimated_duration_minutes?: number; difficulty: string; geometry_geojson?: string; accessibility_notes?: string; status: string };
type Experience = { id: string; title: string; description: string; duration_minutes: number; difficulty: string; type: string; status: string; trail_id?: string };
export default function Destinations({ onStart }: { onStart: (adventure: Adventure) => void }) {
  const [destinations, setDestinations] = useState<Destination[]>([]);
  const [selected, setSelected] = useState<Destination | null>(null);
  const [trails, setTrails] = useState<Trail[]>([]);
  const [experiences, setExperiences] = useState<Experience[]>([]);
  const [error, setError] = useState('');
  const [starting, setStarting] = useState('');
  useEffect(() => { void request<{ destinations: Destination[] }>('/api/destinations').then(x => { setDestinations(x.destinations); if (x.destinations.length) setSelected(x.destinations[0]); }).catch(e => setError(e.message)); }, []);
  useEffect(() => { if (!selected) return; let live = true; Promise.all([request<{ trails: Trail[] }>(`/api/destinations/${selected.id}/trails`), request<{ experiences: Experience[] }>(`/api/destinations/${selected.id}/experiences`)]).then(([t,e]) => { if (live) { setTrails(t.trails); setExperiences(e.experiences); } }).catch(e => { if (live) setError(e.message); }); return () => { live = false; }; }, [selected]);
  const startExperience = async (experience: Experience) => {
    if (!selected || !navigator.onLine) { setError('Connect to generate an outdoor mission.'); return; }
    setStarting(experience.id); setError('');
    try {
      const payload = { duration_minutes: Math.min(60, Math.max(10, experience.duration_minutes)), interest: experience.type.includes('bird') ? 'birdwatching' : experience.type.includes('mindfulness') ? 'mindfulness' : experience.type.includes('education') ? 'environmental_education' : 'wildlife', environment: selected.status === 'verified' ? 'approved_nature_destination' : selected.type === 'forest' ? 'forest' : 'park', difficulty: ['beginner','intermediate','experienced'].includes(experience.difficulty) ? experience.difficulty : 'beginner', ...(selected.status === 'verified' ? { destination_id: selected.id } : {}) };
      const adventure = await request<Adventure>('/api/adventures/generate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) }); onStart(adventure);
    } catch (e) { setError(e instanceof Error ? e.message : 'Could not start this outdoor mission.'); }
    finally { setStarting(''); }
  };
  return <section className="feature-page"><header className="welcome"><span className="eyebrow"><Compass size={16}/> NATURE DESTINATIONS</span><h1>Find a place to listen</h1><p>Explore approved destination experiences and verified trail information where available.</p></header>
    {error && <p role="alert" className="error-box">{error}</p>}
    {!destinations.length ? <div className="empty-state"><span className="empty-icon"><MapPin size={30}/></span><h3>Destinations are being prepared.</h3><p>EcoSound only displays destination and route information approved for publication.</p></div> : <div className="destination-layout"><aside className="destination-list">{destinations.map(d => <button key={d.id} className={`destination-choice ${selected?.id === d.id ? 'selected' : ''}`} onClick={() => setSelected(d)}><MapPin size={17}/><span><strong>{d.name}</strong><small>{d.region}, {d.country}</small></span><ArrowRight size={15}/></button>)}</aside><div className="destination-details">{selected && <><article className="feature-card destination-intro"><span className="feature-icon"><Leaf size={23}/></span><div><span className="eyebrow">{selected.type.replace('_',' ')} · {selected.status}</span><h2>{selected.name}</h2><p>{selected.description}</p>{selected.website && <a href={selected.website} target="_blank" rel="noreferrer">Visit destination website</a>}</div></article><div className="section-heading"><div><span className="eyebrow">APPROVED CONTENT</span><h2>Outdoor experiences</h2></div></div>{experiences.length ? <div className="feature-list">{experiences.map(item => <article className="feature-card experience-card" key={item.id}><span className="feature-icon"><Compass size={20}/></span><div><span className="status-badge predicted">{item.status === 'concept' ? 'Concept demonstration' : 'Approved experience'}</span><h3>{item.title}</h3><p>{item.description}</p><p className="source-note">Adventure guidance is curated by the destination and is used by the generator.</p><small>{item.duration_minutes} minutes · {item.difficulty}</small><button className="secondary-button experience-start" disabled={starting !== ''} onClick={() => void startExperience(item)}>{starting === item.id ? 'Preparing mission…' : selected.status === 'concept' ? 'Try route-free listening mission' : 'Start this destination mission'} <ArrowRight size={15}/></button></div></article>)}</div> : <p className="source-note">No published experiences for this destination.</p>}<div className="section-heading"><div><span className="eyebrow">ROUTE INFORMATION</span><h2>Verified trails</h2></div></div>{trails.length ? <div className="feature-list">{trails.map(t => <article className="feature-card" key={t.id}><Route size={18}/><div><h3>{t.name}</h3><p>{t.description}</p><small>{t.distance_km ? `${t.distance_km} km · ` : ''}{t.estimated_duration_minutes ? `${t.estimated_duration_minutes} min · ` : ''}{t.difficulty}</small>{t.accessibility_notes && <p>{t.accessibility_notes}</p>}</div></article>)}</div> : <p className="source-note">No verified trail data is published here. Adventures remain route-free; EcoSound does not generate trails.</p>}</>}</div></div>}
  </section>;
}
