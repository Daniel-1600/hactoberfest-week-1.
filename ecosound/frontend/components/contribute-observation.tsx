'use client';
import { useState } from 'react';
import { Globe2, LoaderCircle, MapPin } from 'lucide-react';
import { ApiError, request } from '../lib/api';
import { enqueueOperation } from '../lib/offline-store';

const CONSENT_VERSION = 'ecosound-observation-v1';
export default function ContributeObservation({ discoveryID }: { discoveryID: number }) {
  const [open, setOpen] = useState(false);
  const [consent, setConsent] = useState(false);
  const [includeLocation, setIncludeLocation] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const contribute = async () => {
    if (!consent) { setMessage('Please review and accept the sharing consent first.'); return; }
    setBusy(true); setMessage('');
    let queued: Record<string, unknown> | null = null;
    try {
      const submit = async (location?: { latitude: number; longitude: number; accuracy: number }) => {
        const payload = { discovery_id: discoveryID, consent: true, consent_version: CONSENT_VERSION, visibility: 'public', location_source: location ? 'device' : 'none', ...(location ? { latitude: location.latitude, longitude: location.longitude, location_accuracy_meters: location.accuracy } : {}) };
        queued = payload;
        if (!navigator.onLine) throw new ApiError('offline', 'network_error', 0);
        return request('/api/observations', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': crypto.randomUUID() }, body: JSON.stringify(payload) });
      };
      if (includeLocation && navigator.geolocation) {
        const location = await new Promise<GeolocationPosition>((resolve, reject) => navigator.geolocation.getCurrentPosition(resolve, reject, { enableHighAccuracy: false, timeout: 12000, maximumAge: 60000 }));
        await submit({ latitude: location.coords.latitude, longitude: location.coords.longitude, accuracy: location.coords.accuracy });
      } else await submit();
      setMessage('Observation shared. Any map location is generalized; your audio remains private.'); setOpen(false);
    } catch (e) {
      if (e instanceof ApiError && e.status === 0 && queued) {
        try { await enqueueOperation('create_observation', queued); setMessage('Consented observation queued on this device. It will sync when connected; its location remains private until the server generalizes it.'); setOpen(false); }
        catch { setMessage('The connection failed and this browser could not queue the observation. No observation was shared.'); }
      } else setMessage(e instanceof Error ? e.message : 'Could not share this observation.');
    }
    finally { setBusy(false); }
  };
  return <div className="contribute-wrap"><button className="text-button" onClick={() => setOpen(value => !value)}><Globe2 size={15}/> Contribute observation</button>{open && <div className="contribute-panel"><p>Share this saved observation with the community? Your recording stays private. If you include location, exact coordinates are stored privately on this server and generalized for the public map; sensitive or uncurated species stay off the map.</p><label className="consent-check"><input type="checkbox" checked={consent} onChange={e => setConsent(e.target.checked)}/> I agree to share this wildlife observation publicly under EcoSound’s current consent terms.</label><label className="consent-check"><input type="checkbox" checked={includeLocation} onChange={e => setIncludeLocation(e.target.checked)}/> Include current location (optional; browser location permission is requested only when sharing).</label><button className="secondary-button" disabled={busy || !consent} onClick={() => void contribute()}>{busy ? <LoaderCircle className="spin" size={15}/> : includeLocation ? <MapPin size={15}/> : <Globe2 size={15}/>} Share observation</button></div>}{message && <small role="status" className="share-status">{message}</small>}</div>;
}
