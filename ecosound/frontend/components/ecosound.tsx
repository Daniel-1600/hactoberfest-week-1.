'use client';
import { useCallback, useEffect, useState } from 'react';
import { ArrowRight, AudioLines, BookOpen, Check, ChevronRight, Compass, Feather, Globe2, Headphones, Leaf, MapPin, Mic, RefreshCw, Sparkles, Sprout, Sun, Trophy, Award } from 'lucide-react';
import { audioUrl, CONFIRMATION_COPY, Discovery, Quest, request } from '../lib/api';
import { synchronize } from '../lib/sync';
import ForestArt from './forest-art';
import Listen from './listen';
import Explore, { Adventure, completeTask } from './explore';
import ActiveAdventure from './active-adventure';
import BiodiversityMap from './biodiversity-map';
import Destinations from './destinations';
import Passport from './passport';
import ContributeObservation from './contribute-observation';
import NatureGuide from './nature-guide';

type View = 'home' | 'listen' | 'collection' | 'explore' | 'adventure' | 'map' | 'destinations' | 'passport';
export default function EcoSound() {
  const [view, setView] = useState<View>('home');
  const [activeAdventure, setActiveAdventure] = useState<Adventure | null>(null);
  const [quest, setQuest] = useState<Quest | null>(null);
  const [discoveries, setDiscoveries] = useState<Discovery[]>([]);
  const [loading, setLoading] = useState(true);
  const [questError, setQuestError] = useState('');
  const [collectionError, setCollectionError] = useState('');
  const [confirming, setConfirming] = useState<number | null>(null);
  const [confirmError, setConfirmError] = useState('');
  const refresh = useCallback(async () => {
    setLoading(true);
    const [q, d] = await Promise.allSettled([request<Quest>('/api/quests/today'), request<{ discoveries: Discovery[] }>('/api/discoveries')]);
    if (q.status === 'fulfilled') { setQuest(q.value); setQuestError(''); } else setQuestError(q.reason.message);
    if (d.status === 'fulfilled') { setDiscoveries(d.value.discoveries); setCollectionError(''); } else setCollectionError(d.reason.message);
    setLoading(false);
  }, []);
  useEffect(() => {
    let mounted = true;
    void Promise.resolve().then(() => { if (mounted) void refresh(); });
    return () => { mounted = false; };
  }, [refresh]);
  useEffect(() => {
    const handleOnline = () => { void synchronize().then(() => refresh()); };
    window.addEventListener('online', handleOnline);
    return () => window.removeEventListener('online', handleOnline);
  }, [refresh]);
  useEffect(() => {
    void Promise.resolve().then(() => { try { const saved = localStorage.getItem('ecosound-active-adventure'); if (saved) setActiveAdventure(JSON.parse(saved) as Adventure); } catch { /* Ignore corrupted local cache. */ } });
  }, []);
  const updateAdventure = useCallback((adventure: Adventure) => { setActiveAdventure(adventure); try { localStorage.setItem('ecosound-active-adventure', JSON.stringify(adventure)); } catch { /* Cache is best effort. */ } }, []);
  const chooseAdventure = (adventure: Adventure) => { updateAdventure(adventure); setView('adventure'); };
  const discoverySaved = useCallback(async (discoveryId: number) => {
    await refresh();
    if (discoveryId && activeAdventure?.status === 'active') {
      const task = activeAdventure.tasks.find(item => !item.completed && item.type === 'audio_discovery') || activeAdventure.tasks.find(item => !item.completed && item.type === 'species_discovery');
      if (task) {
        try { const next = await completeTask(activeAdventure, task, { discovery_id: discoveryId }); if (next) updateAdventure(next); }
        catch { /* The discovery remains saved; task can be retried from the adventure screen. */ }
      }
    }
  }, [activeAdventure, refresh, updateAdventure]);
  const nav: { view: View; label: string; icon: typeof Compass }[] = [{view:'home',label:'Home',icon:Compass},{view:'explore',label:'Explore',icon:Sparkles},{view:'listen',label:'Listen',icon:Mic},{view:'collection',label:'Collection',icon:BookOpen},{view:'map',label:'Map',icon:Globe2},{view:'destinations',label:'Places',icon:MapPin},{view:'passport',label:'Passport',icon:Award}];
  const confirm = async (id: number) => {
    setConfirming(id); setConfirmError('');
    try {
      const updated = await request<Discovery>(`/api/discoveries/${id}/confirm`, { method: 'POST' });
      setDiscoveries(previous => previous.map(item => item.id === id ? updated : item));
    } catch (error) { setConfirmError(error instanceof Error ? error.message : 'Confirmation failed. Try again.'); }
    finally { setConfirming(null); }
  };
  const completed = quest?.tasks.filter(task => task.completed).length || 0;
  return <div className="app-shell">
    <header className="site-header"><button className="brand" onClick={() => setView('home')} aria-label="EcoSound home"><span className="brand-mark"><AudioLines size={24}/><Leaf size={12}/></span><span>eco<span className="brand-light">sound</span><span className="brand-dot">.</span></span></button>
      <nav className="desktop-nav" aria-label="Main navigation">{nav.map(item => <button key={item.view} aria-current={view === item.view || item.view === 'explore' && view === 'adventure' ? 'page' : undefined} className={view === item.view || item.view === 'explore' && view === 'adventure' ? 'active' : ''} onClick={() => setView(item.view)}>{item.view === 'collection' ? 'My collection' : item.label}</button>)}</nav>
      <span className="header-note"><span className="live-dot"/>Your outdoors companion</span>
    </header>
    <main id="main-content" className="main-content">
      {quest?.completed && <div className="quest-complete-banner" role="status"><Trophy size={21}/><span>Quest complete! You explored the natural world today.</span></div>}
      {view === 'home' && <>
        <section className="welcome"><span className="eyebrow"><Sun size={16}/> A LITTLE CURIOSITY. A LOT OF DISCOVERY.</span><h1>Explore the world through sound</h1><p>Slow down, tune in, and meet the wildlife around you.</p></section>
        <section className="hero-grid">
          <div className="hero-copy"><span className="pill"><Leaf size={14}/> FIND YOUR WILD</span><h2>A little closer<br/>to nature.</h2><p>Every birdsong has a story. Capture a call, discover who’s singing, and make your next walk an adventure.</p><button className="primary-button" onClick={() => setView('listen')}><Mic size={18}/> Record Wildlife <ArrowRight size={17}/></button><button className="secondary-button adventure-home-button" onClick={() => setView('explore')}><Sparkles size={16}/> Create an outdoor adventure</button><span className="hero-footnote"><Headphones size={14}/> A quiet moment is all you need.</span></div>
          <div className="hero-art"><ForestArt/><span className="art-tag"><span className="live-dot"/> A whole world worth listening to</span><span className="art-caption">GO OUTSIDE. GET CURIOUS.</span></div>
        </section>
        <div className="section-heading"><div><span className="eyebrow">ONE SMALL ADVENTURE, EVERY DAY</span><h2>Your daily quest</h2></div><span className="subtle-chip"><Sun size={14}/> {quest ? `Today · ${quest.timezone}` : 'Today'}</span></div>
        <section className="quest-card"><div className="quest-intro"><span className="quest-icon"><Compass size={30}/></span><div><span className="eyebrow">DAILY ADVENTURE</span><h3>{quest?.name || 'Morning Bird Quest'}</h3><p>Let the birds lead the way.</p></div></div>
          <div className="quest-tasks">{questError ? <div role="alert" className="inline-error">{questError}<button className="text-button" onClick={() => void refresh()}><RefreshCw size={14}/> Try again</button></div> : loading && !quest ? <p className="muted" role="status">Loading your quest…</p> : quest?.tasks.map(task => <div className="quest-task" key={task.name}><span className={`task-check ${task.completed ? 'complete' : ''}`}>{task.completed ? <Check size={12}/> : <Feather size={12}/>}</span><div><div className="task-label"><span>{task.name}</span><strong>{task.current}/{task.target}</strong></div><div className="progress-track" role="progressbar" aria-label={task.name} aria-valuenow={Math.min(task.current, task.target)} aria-valuemin={0} aria-valuemax={task.target}><span style={{ width: `${Math.min(100, task.current / task.target * 100)}%` }}/></div></div></div>)}</div>
          <div className="quest-action"><span><Trophy size={14}/> {quest?.completed ? 'Quest complete!' : quest ? `${completed}/${quest.tasks.length} goals complete` : 'Your daily adventure'}</span><button className="secondary-button" onClick={() => setView('listen')}>{quest?.completed ? 'Keep exploring' : 'Start quest'}<ArrowRight size={15}/></button></div>
        </section>
        <div className="section-heading"><div><span className="eyebrow">FROM A SOUND TO A STORY</span><h2>Follow your curiosity</h2></div></div>
        <section className="steps-grid">{[{ icon: Mic, title: 'Pause & listen', text: 'Find a quiet spot. Record up to 60 seconds of the sounds around you.', number: '01' }, { icon: Sparkles, title: 'Meet the singer', text: 'BirdNET helps identify bird calls. Listen back and explore the possibilities.', number: '02' }, { icon: BookOpen, title: 'Keep the memory', text: 'Save a discovery to your collection and watch your nature journal grow.', number: '03' }].map(step => <article className="step-card" key={step.number}><div><span className="step-icon"><step.icon size={20}/></span><span className="step-number">{step.number}</span></div><h3>{step.title}</h3><p>{step.text}</p></article>)}</section>
        {discoveries.length > 0 && <section className="recent-discoveries"><div className="section-heading"><div><span className="eyebrow">YOUR LATEST WILD MOMENTS</span><h2>Recent discoveries</h2></div><button className="text-button" onClick={() => setView('collection')}>View all <ArrowRight size={16}/></button></div><div className="recent-grid">{discoveries.slice(0, 3).map(discovery => <article className="recent-card" key={discovery.id}><span className="bird-icon"><Feather size={21}/></span><div><h3>{discovery.common_name || discovery.scientific_name || 'Unidentified bird'}</h3><p className="scientific-name">{discovery.scientific_name || 'Scientific name unavailable'}</p><span className={`status-badge ${discovery.user_confirmed ? 'confirmed' : 'predicted'}`}>{discovery.user_confirmed ? 'Personally confirmed' : 'AI predicted'}</span></div></article>)}</div></section>}
        <section className="journal-teaser"><span className="journal-icon"><Sprout size={25}/></span><div><h3>Your discoveries, all in one place.</h3><p>{discoveries.length ? `${discoveries.length} saved ${discoveries.length === 1 ? 'memory' : 'memories'} from your outdoor adventures.` : 'A personal collection of the wild moments you’ve found.'}</p></div><button className="text-button" onClick={() => setView('collection')}>Open my collection <ChevronRight size={17}/></button></section>
      </>}
      {view === 'listen' && <Listen onSaved={discoverySaved} onCollection={() => setView('collection')}/>}
      {view === 'explore' && <Explore onStart={chooseAdventure} onDestinations={() => setView('destinations')}/>}
      {view === 'adventure' && activeAdventure && <ActiveAdventure initial={activeAdventure} onBack={() => setView('explore')} onRecord={() => setView('listen')} onUpdate={updateAdventure}/>}
      {view === 'adventure' && !activeAdventure && <Explore onStart={chooseAdventure} onDestinations={() => setView('destinations')}/>}
      {view === 'map' && <BiodiversityMap/>}
      {view === 'destinations' && <Destinations onStart={chooseAdventure}/>}
      {view === 'passport' && <Passport/>}
      {view === 'collection' && <>
        <section className="welcome"><span className="eyebrow"><BookOpen size={16}/> YOUR NATURE JOURNAL</span><h1>Little moments. Wild memories.</h1><p>Every discovery is a reason to listen a little closer.</p></section>
        <div className="section-heading"><h2>My collection <span className="count-badge">{discoveries.length}</span></h2><button className="text-button" disabled={loading} onClick={() => void refresh()}><RefreshCw size={15} className={loading ? 'spin' : ''}/> Refresh</button></div>
        {collectionError && <p className="error-box" role="alert">{collectionError}</p>}{confirmError && <p className="error-box" role="alert">{confirmError}</p>}
        {loading && !discoveries.length ? <p className="loading-box" role="status">Opening your nature journal…</p> : collectionError && !discoveries.length ? <p className="loading-box">Your journal is unavailable right now. Refresh to try again.</p> : !discoveries.length ? <div className="empty-state"><span className="empty-icon"><Feather size={36}/></span><h3>Your first discovery is out there.</h3><p>Record a bird call and save an identification to begin your collection.</p><button className="primary-button" onClick={() => setView('listen')}><Mic size={17}/> Find my first discovery <ArrowRight size={16}/></button></div> : <div className="collection-grid">{discoveries.map(discovery => <article className="discovery-card" key={discovery.id}><div className="discovery-top"><span className="bird-icon"><Feather size={24}/></span><span className={`status-badge ${discovery.user_confirmed ? 'confirmed' : 'predicted'}`}>{discovery.user_confirmed ? 'Personally confirmed' : 'AI predicted'}</span></div><h3>{discovery.common_name || discovery.scientific_name || 'Unidentified bird'}</h3><p className="scientific-name">{discovery.scientific_name}</p><span className="discovery-date">{new Date(discovery.observed_at).toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' })}</span><div className="score-line"><Sparkles size={14}/> AI identification score <strong>{Math.round(discovery.confidence * 100)}%</strong></div><p className="score-caveat">An AI suggestion, not guaranteed accuracy.</p><audio controls preload="none" src={audioUrl(discovery.audio_url)} aria-label={`Play ${discovery.common_name} recording`}/><NatureGuide scientificName={discovery.scientific_name} discoveryId={discovery.id}/><div className="confirmation">{discovery.user_confirmed ? <p><Check size={15}/>{CONFIRMATION_COPY}</p> : <><p>Recognize this call yourself?</p><button className="secondary-button" disabled={confirming !== null} onClick={() => void confirm(discovery.id)}>{confirming === discovery.id ? 'Confirming…' : 'I personally confirm this'}<Check size={15}/></button></>}<ContributeObservation discoveryID={discovery.id}/></div></article>)}</div>}
      </>}
      <footer className="footer"><span><Leaf size={14}/> Made for curious minds & open skies.</span><span>AI helps you explore. Nature has the final word.</span></footer>
    </main>
    <nav className="mobile-nav" aria-label="Mobile navigation">{nav.map(item => <button key={item.view} className={view === item.view || item.view === 'explore' && view === 'adventure' ? 'active' : ''} aria-current={view === item.view || item.view === 'explore' && view === 'adventure' ? 'page' : undefined} onClick={() => setView(item.view)}><item.icon size={19}/>{item.label}</button>)}</nav>
  </div>;
}
