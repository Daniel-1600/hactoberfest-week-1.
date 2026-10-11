'use client';
import { useEffect, useState } from 'react';
import { Globe2, LoaderCircle, MapPin, RefreshCw } from 'lucide-react';
import { request } from '../lib/api';

type Observation = { id: string; scientific_name: string; common_name: string; observed_at: string; latitude?: number; longitude?: number; verification_status: string; location_generalized: boolean; destination_id?: string };
type MapObject = { remove: () => void; addControl: (control: unknown) => void; on: (name: string, callback: () => void) => void; loaded: () => boolean; once: (name: string, callback: () => void) => void; addSource: (id: string, source: unknown) => void; addLayer: (layer: unknown) => void; getSource: (id: string) => { setData: (data: unknown) => void } | undefined; fitBounds: (bounds: [[number, number], [number, number]], options?: object) => void };
type MapLibrary = { Map: new (options: object) => MapObject; NavigationControl: new () => unknown };
declare global { interface Window { maplibregl?: MapLibrary } }

const scripts = [
  'https://unpkg.com/maplibre-gl@6.13.0/dist/maplibre-gl.js',
];
function loadMapLibrary(): Promise<MapLibrary> {
  if (window.maplibregl) return Promise.resolve(window.maplibregl);
  if (!document.querySelector('link[data-ecosound-map]')) { const link = document.createElement('link'); link.rel = 'stylesheet'; link.href = 'https://unpkg.com/maplibre-gl@6.13.0/dist/maplibre-gl.css'; link.dataset.ecosoundMap = 'true'; document.head.append(link); }
  return new Promise((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>('script[data-ecosound-map]');
    const script = existing || document.createElement('script');
    const done = () => window.maplibregl ? resolve(window.maplibregl) : reject(new Error('MapLibre did not load.'));
    script.addEventListener('load', done, { once: true }); script.addEventListener('error', () => reject(new Error('Map display is unavailable.')), { once: true });
    if (!existing) { script.src = scripts[0]; script.dataset.ecosoundMap = 'true'; document.body.append(script); }
  });
}

export default function BiodiversityMap() {
  const [items, setItems] = useState<Observation[]>([]);
  const [species, setSpecies] = useState('');
  const [status, setStatus] = useState('');
  const [since, setSince] = useState('');
  const [until, setUntil] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [mapError, setMapError] = useState('');
  const [destinations, setDestinations] = useState<{ id: string; name: string }[]>([]);
  const [destination, setDestination] = useState('');
  const [mapHost, setMapHost] = useState<HTMLDivElement | null>(null);
  const [refreshIndex, setRefreshIndex] = useState(0);
  useEffect(() => { void request<{ destinations: { id: string; name: string }[] }>('/api/destinations').then(v => setDestinations(v.destinations)).catch(() => undefined); }, []);
  useEffect(() => {
    let alive = true;
    const load = async () => { setBusy(true); setError(''); const params = new URLSearchParams(); if (species.trim()) params.set('species', species.trim()); if (status) params.set('status', status); if (destination) params.set('destination', destination); if (since) params.set('since', new Date(`${since}T00:00:00`).toISOString()); if (until) params.set('until', new Date(`${until}T23:59:59.999`).toISOString());
      try { const result = await request<{ observations: Observation[] }>(`/api/map/observations${params.size ? `?${params}` : ''}`); if (alive) setItems(result.observations); }
      catch (e) { if (alive) setError(e instanceof Error ? e.message : 'Observations are unavailable.'); } finally { if (alive) setBusy(false); }
    }; void load(); return () => { alive = false; };
  }, [species, status, destination, since, until, refreshIndex]);
  useEffect(() => {
    if (!mapHost || !items.length) return;
    let map: MapObject | null = null; let alive = true;
    const setup = async () => {
      try {
        const lib = await loadMapLibrary(); if (!alive) return;
        map = new lib.Map({ container: mapHost, style: 'https://tiles.openfreemap.org/styles/bright', center: [36.817, -1.286], zoom: 5 });
        map.addControl(new lib.NavigationControl());
        const draw = () => {
          if (!map) return;
          const features: { type: 'Feature'; geometry: { type: 'Point'; coordinates: [number, number] }; properties: { label: string } }[] = items.filter(x => x.latitude != null && x.longitude != null).map(x => ({ type: 'Feature', geometry: { type: 'Point', coordinates: [x.longitude as number, x.latitude as number] }, properties: { label: `${x.common_name || x.scientific_name} · ${x.verification_status.replace('_',' ')}` } }));
          const data = { type: 'FeatureCollection', features };
          if (map.getSource('observations')) map.getSource('observations')?.setData(data);
          else { map.addSource('observations', { type: 'geojson', data }); map.addLayer({ id: 'observations', type: 'circle', source: 'observations', paint: { 'circle-radius': 8, 'circle-color': '#236546', 'circle-stroke-color': '#fffaf0', 'circle-stroke-width': 2 } }); }
          if (features.length === 1) { const p = features[0]?.geometry.coordinates; if (p) map.fitBounds([[p[0] - .02, p[1] - .02], [p[0] + .02, p[1] + .02]], { padding: 40, maxZoom: 12 }); }
        };
        map.on('load', draw); if (map.loaded()) draw();
      } catch (e) { if (alive) setMapError(e instanceof Error ? e.message : 'The map could not be displayed.'); }
    }; void setup(); return () => { alive = false; map?.remove(); };
  }, [mapHost, items]);
  return <section className="feature-page"><header className="welcome"><span className="eyebrow"><Globe2 size={16}/> COMMUNITY OBSERVATIONS</span><h1>Listen to the living map</h1><p>Approximate locations shared by explorers. AI identifications are suggestions, not verified records.</p></header>
    <div className="feature-card map-filters"><label>Species scientific name<input value={species} onChange={e => setSpecies(e.target.value)} placeholder="Filter by species"/></label><label>Verification label<select value={status} onChange={e => setStatus(e.target.value)}><option value="">Any public status</option><option value="ai_predicted">AI predicted</option><option value="user_confirmed">Personally confirmed</option><option value="expert_verified">Expert verified</option></select></label><label>Destination<select value={destination} onChange={e => setDestination(e.target.value)}><option value="">All areas</option>{destinations.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}</select></label><label>From<input type="date" value={since} onChange={e => setSince(e.target.value)}/></label><label>To<input type="date" value={until} onChange={e => setUntil(e.target.value)}/></label><button className="secondary-button" onClick={() => setRefreshIndex(n => n + 1)}><RefreshCw size={15}/> Refresh</button></div>
    {error && <p role="alert" className="error-box">{error}</p>}{busy && <p role="status" className="source-note"><LoaderCircle size={15} className="spin"/> Loading public observations…</p>}
    {!busy && !items.length ? <div className="empty-state"><span className="empty-icon"><MapPin size={31}/></span><h3>No observations in this view yet.</h3><p>When someone chooses to share a discovery, this map will show only its generalized location. EcoSound never fabricates map data.</p></div> : items.length > 0 && <><div ref={setMapHost} className="map-canvas" aria-label="Map of approximate wildlife observations"/>{mapError && <p className="source-note" role="status">{mapError} The privacy-safe observation list is still available below.</p>}<div className="feature-list map-observations">{items.map(item => <article className="feature-card" key={item.id}><MapPin size={17}/><div><strong>{item.common_name || item.scientific_name}</strong><small><em>{item.scientific_name}</em> · {new Date(item.observed_at).toLocaleDateString()}</small></div><span className="status-badge predicted">{item.verification_status.replace('_',' ')}{item.location_generalized ? ' · approximate' : ''}</span></article>)}</div></>}
    <p className="source-note">Map tiles: OpenFreeMap with OpenStreetMap attribution. MapLibre GL JS is MIT licensed. Map tiles require internet; observation APIs expose generalized coordinates only.</p>
  </section>;
}
