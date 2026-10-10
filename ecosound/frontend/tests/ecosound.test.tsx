import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import EcoSound from '../components/ecosound';
import Listen from '../components/listen';
import { CONFIRMATION_COPY, strongestCandidates } from '../lib/api';
import { useRecorder } from '../lib/use-recorder';

const prediction = { id: 12, scientific_name: 'Turdus merula', common_name: 'Common Blackbird', raw_species_name: 'Turdus merula_Common Blackbird', confidence: .87, start_time: 0, end_time: 3, user_confirmed: false };
const otherPrediction = { ...prediction, id: 13, scientific_name: 'Erithacus rubecula', common_name: 'European Robin', confidence: .65 };
const identification = { recording_id: 'recording-1', audio_url: '/api/recordings/recording-1/audio', duration_seconds: 12, model: 'birdnet', model_version: '3.0', predictions: [prediction, { ...prediction, id: 10, confidence: .55 }, otherPrediction] };
const discovery = { id: 1, recording_id: 'recording-1', prediction_id: 12, scientific_name: prediction.scientific_name, common_name: prediction.common_name, confidence: .87, observed_at: '2026-10-10T06:00:00Z', user_confirmed: false, audio_url: identification.audio_url };
const quest = (current = 0) => ({ name: 'Morning Bird Quest', date: '2026-10-10', timezone: 'Africa/Nairobi', completed: false, tasks: [{ name: 'Record three distinct calls', current, target: 3, completed: false }, { name: 'Discover two distinct species', current, target: 2, completed: false }] });
function response(body: unknown, status = 200) { return Promise.resolve({ ok: status < 400, status, json: async () => body } as Response); }
function upload() { fireEvent.change(screen.getByLabelText('Upload audio'), { target: { files: [new File(['real test bytes'], 'call.wav', { type: 'audio/wav' })] } }); }
beforeEach(() => {
  URL.createObjectURL = vi.fn(() => 'blob:actual-recording');
  URL.revokeObjectURL = vi.fn();
});

describe('real API workflow', () => {
  it('shows the server completion banner and caps accessible progress values', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => url.endsWith('/api/quests/today') ? response({ ...quest(4), completed: true, tasks: quest(4).tasks.map(task => ({ ...task, completed: true })) }) : response({ discoveries: [] })));
    render(<EcoSound/>);
    expect(await screen.findByText('Quest complete! You explored the natural world today.')).toBeDefined();
    expect(screen.getByRole('progressbar', { name: 'Record three distinct calls' }).getAttribute('aria-valuenow')).toBe('3');
    expect(screen.getByRole('progressbar', { name: 'Discover two distinct species' }).getAttribute('aria-valuenow')).toBe('2');
  });

  it('handles absent timing and label metadata without inventing a species', async () => {
    vi.stubGlobal('fetch', vi.fn(() => response({ ...identification, predictions: [{ ...prediction, common_name: '', scientific_name: '', raw_species_name: 'source-label', start_time: null, end_time: null }] })));
    render(<Listen onSaved={async () => {}} onCollection={() => {}}/>);
    upload(); fireEvent.click(screen.getByRole('button', { name: 'Identify this sound' }));
    expect(await screen.findByText('source-label')).toBeDefined();
    expect(screen.getByText('Call timing unavailable')).toBeDefined();
  });

  it('uses server quest state, saves the selected prediction, refreshes collection and quest, and permits only one save', async () => {
    let saved = false;
    const fetchMock = vi.fn((url: string, options?: RequestInit) => {
      if (url.endsWith('/api/quests/today')) return response(quest(saved ? 1 : 0));
      if (url.endsWith('/api/identify')) {
        expect(options?.body).toBeInstanceOf(FormData);
        expect((options!.body as FormData).get('audio')).toBeInstanceOf(File);
        return response(identification);
      }
      if (url.endsWith('/api/discoveries') && options?.method === 'POST') {
        expect(JSON.parse(options.body as string)).toEqual({ prediction_id: 12 });
        saved = true; return response(discovery, 201);
      }
      return response({ discoveries: saved ? [discovery] : [] });
    });
    vi.stubGlobal('fetch', fetchMock);
    render(<EcoSound/>);
    expect(await screen.findByText('0/3')).toBeDefined();
    fireEvent.click(screen.getByRole('button', { name: /Start quest/ }));
    upload(); fireEvent.click(screen.getByRole('button', { name: 'Identify this sound' }));
    await screen.findByText('Common Blackbird');
    expect(screen.getAllByRole('button', { name: 'Save discovery' })).toHaveLength(2);
    expect(screen.getByText('87%')).toBeDefined();
    fireEvent.click(screen.getAllByRole('button', { name: 'Save discovery' })[0]);
    await screen.findByText('Your wild moment is in your collection.');
    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => url.endsWith('/api/quests/today'))).toHaveLength(2));
    for (const button of screen.getAllByRole('button', { name: 'Recording saved' })) expect((button as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole('button', { name: 'EcoSound home' }));
    expect(await screen.findByText('1/3')).toBeDefined();
    expect(screen.getByText('1/2')).toBeDefined();
  });

  it('shows duplicate errors and disables further candidate saves for that recording', async () => {
    const refreshed = vi.fn(async () => {});
    vi.stubGlobal('fetch', vi.fn((url: string) => url.endsWith('/api/identify') ? response(identification) : response({ error: { code: 'duplicate_discovery', message: 'Already saved' } }, 409)));
    render(<Listen onSaved={refreshed} onCollection={() => {}}/>);
    upload(); fireEvent.click(screen.getByRole('button', { name: 'Identify this sound' }));
    await screen.findByText('Common Blackbird');
    fireEvent.click(screen.getAllByRole('button', { name: 'Save discovery' })[0]);
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'A discovery from this recording is already in your collection. Each recording can be saved once.');
    expect(refreshed).toHaveBeenCalledOnce();
    expect(screen.getAllByRole('button', { name: 'Recording saved' }).every(button => (button as HTMLButtonElement).disabled)).toBe(true);
  });

  it('distinguishes successful no detection from inference failure', async () => {
    const fetchMock = vi.fn().mockImplementationOnce(() => response({ ...identification, predictions: [] })).mockImplementationOnce(() => response({ error: { code: 'model_unavailable', message: 'BirdNET is unavailable. Install the model and try again.' } }, 503));
    vi.stubGlobal('fetch', fetchMock);
    render(<Listen onSaved={async () => {}} onCollection={() => {}}/>);
    upload(); fireEvent.click(screen.getByRole('button', { name: 'Identify this sound' }));
    await screen.findByText('No bird calls identified this time.');
    expect(screen.queryByRole('button', { name: 'Save discovery' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Identify this sound' }));
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'BirdNET is unavailable. Install the model and try again.');
    expect(screen.queryByText('No bird calls identified this time.')).toBeNull();
  });

  it('plays stored audio and shows the exact personal confirmation wording', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      if (url.endsWith('/confirm')) return response({ ...discovery, user_confirmed: true });
      if (url.endsWith('/api/quests/today')) return response(quest(1));
      return response({ discoveries: [discovery] });
    }));
    render(<EcoSound/>);
    await screen.findByText('1/3');
    fireEvent.click(screen.getByRole('button', { name: 'My collection' }));
    expect(screen.getByLabelText('Play Common Blackbird recording').getAttribute('src')).toBe('http://localhost:8080/api/recordings/recording-1/audio');
    fireEvent.click(screen.getByRole('button', { name: 'I personally confirm this' }));
    expect(await screen.findByText(CONFIRMATION_COPY)).toBeDefined();
  });
});

class MockRecorder {
  static isTypeSupported() { return true; }
  state = 'inactive'; mimeType = 'audio/webm';
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  onerror: (() => void) | null = null;
  start() { this.state = 'recording'; }
  stop() { this.state = 'inactive'; this.ondataavailable?.({ data: new Blob(['captured microphone audio'], { type: 'audio/webm' }) }); this.onstop?.(); }
}

describe('microphone lifecycle', () => {
  it('explains denied permission and keeps the upload alternative available', async () => {
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: { getUserMedia: vi.fn(async () => { throw new DOMException('Denied', 'NotAllowedError'); }) } });
    vi.stubGlobal('MediaRecorder', MockRecorder);
    render(<Listen onSaved={async () => {}} onCollection={() => {}}/>);
    fireEvent.click(screen.getByRole('button', { name: 'Start recording' }));
    expect((await screen.findByRole('alert')).textContent).toContain('Microphone permission was denied.');
    expect((screen.getByLabelText('Upload audio') as HTMLInputElement).disabled).toBe(false);
    expect(screen.queryByRole('button', { name: 'Stop recording' })).toBeNull();
  });

  it('offers audio upload when browser recording is unsupported', async () => {
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: undefined });
    vi.stubGlobal('MediaRecorder', undefined);
    render(<Listen onSaved={async () => {}} onCollection={() => {}}/>);
    fireEvent.click(screen.getByRole('button', { name: 'Start recording' }));
    expect((await screen.findByRole('alert')).textContent).toContain('Recording is unavailable in this browser.');
    expect((screen.getByLabelText('Upload audio') as HTMLInputElement).disabled).toBe(false);
  });

  it('requests permission only after a click, stops tracks, and offers actual captured audio playback', async () => {
    const stopTrack = vi.fn();
    const permission = vi.fn(async () => ({ getTracks: () => [{ stop: stopTrack }] }));
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: { getUserMedia: permission } });
    vi.stubGlobal('MediaRecorder', MockRecorder);
    let now = 0; vi.spyOn(Date, 'now').mockImplementation(() => now);
    render(<Listen onSaved={async () => {}} onCollection={() => {}}/>);
    expect(permission).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Start recording' }));
    await screen.findByRole('button', { name: 'Stop recording' });
    expect(permission).toHaveBeenCalledWith({ audio: true });
    now = 1200; fireEvent.click(screen.getByRole('button', { name: 'Stop recording' }));
    const audio = await screen.findByLabelText('Play your recorded audio');
    await waitFor(() => expect(audio.getAttribute('src')).toBe('blob:actual-recording'));
    expect(stopTrack).toHaveBeenCalledOnce();
    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.objectContaining({ name: 'ecosound-recording.webm', size: 25 }));
  });

  it('releases a microphone granted after the recorder unmounts', async () => {
    const stopTrack = vi.fn();
    let grant!: (stream: MediaStream) => void;
    const permission = vi.fn(() => new Promise<MediaStream>(resolve => { grant = resolve; }));
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: { getUserMedia: permission } });
    vi.stubGlobal('MediaRecorder', MockRecorder);
    const view = render(<Listen onSaved={async () => {}} onCollection={() => {}}/>);
    fireEvent.click(screen.getByRole('button', { name: 'Start recording' }));
    view.unmount();
    await act(async () => grant({ getTracks: () => [{ stop: stopTrack }] } as unknown as MediaStream));
    expect(stopTrack).toHaveBeenCalledOnce();
  });

  it('automatically stops at the recording limit', async () => {
    vi.useFakeTimers();
    const stopTrack = vi.fn();
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: { getUserMedia: vi.fn(async () => ({ getTracks: () => [{ stop: stopTrack }] })) } });
    vi.stubGlobal('MediaRecorder', MockRecorder);
    const { result } = renderHook(() => useRecorder());
    await act(async () => result.current.start());
    expect(result.current.status).toBe('recording');
    await act(async () => vi.advanceTimersByTime(60_000));
    expect(result.current.status).toBe('idle');
    expect(stopTrack).toHaveBeenCalledOnce();
    expect(result.current.file?.size).toBeGreaterThan(0);
  });
});

it('groups the highest score for display without inventing prediction identifiers', () => {
  expect(strongestCandidates(identification.predictions).map(item => item.id)).toEqual([12, 13]);
});
