'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { MAX_AUDIO_BYTES } from './api';

export function useRecorder() {
  const [status, setStatus] = useState<'idle' | 'permission' | 'recording'>('idle');
  const [seconds, setSeconds] = useState(0);
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState('');
  const recorder = useRef<MediaRecorder | null>(null);
  const stream = useRef<MediaStream | null>(null);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null);
  const active = useRef(true);
  const release = useCallback(() => {
    if (timer.current) clearInterval(timer.current);
    timer.current = null;
    stream.current?.getTracks().forEach(track => track.stop());
    stream.current = null;
  }, []);
  const stop = useCallback(() => {
    if (recorder.current?.state === 'recording') recorder.current.stop();
    release();
  }, [release]);
  useEffect(() => {
    active.current = true;
    return () => { active.current = false; stop(); };
  }, [stop]);
  const start = async () => {
    setError('');
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined') {
      setError('Recording is unavailable in this browser. Open EcoSound on HTTPS or localhost, or upload an audio file below.'); return;
    }
    setStatus('permission');
    try {
      const acquired = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (!active.current) { acquired.getTracks().forEach(track => track.stop()); return; }
      stream.current = acquired;
      const mimeType = ['audio/webm;codecs=opus', 'audio/mp4', 'audio/ogg;codecs=opus'].find(type => MediaRecorder.isTypeSupported(type));
      const nextRecorder = new MediaRecorder(acquired, mimeType ? { mimeType } : undefined);
      recorder.current = nextRecorder;
      const chunks: Blob[] = [];
      let bytes = 0;
      let tooLarge = false;
      const started = Date.now();
      nextRecorder.ondataavailable = event => {
        if (event.data.size) { chunks.push(event.data); bytes += event.data.size; }
        if (bytes > MAX_AUDIO_BYTES) { tooLarge = true; stop(); }
      };
      nextRecorder.onerror = () => {
        stop();
        if (active.current) { setError('The microphone recording failed. Please try again or upload audio.'); setStatus('idle'); }
      };
      nextRecorder.onstop = () => {
        release();
        if (!active.current) return;
        setStatus('idle');
        if (tooLarge) { setError('Your recording exceeded 15 MiB. Try a shorter recording.'); return; }
        if (Date.now() - started < 1000 || bytes === 0) { setError('Record at least one second of audio, then try again.'); return; }
        const type = nextRecorder.mimeType || 'audio/webm';
        const extension = type.includes('mp4') ? 'm4a' : type.includes('ogg') ? 'ogg' : 'webm';
        setFile(new File(chunks, `ecosound-recording.${extension}`, { type }));
      };
      setFile(null); setSeconds(0); nextRecorder.start(250); setStatus('recording');
      timer.current = setInterval(() => {
        const elapsed = Math.min(60, Math.floor((Date.now() - started) / 1000));
        if (active.current) setSeconds(elapsed);
        if (elapsed >= 60) stop();
      }, 200);
    } catch (cause) {
      release();
      if (!active.current) return;
      setStatus('idle');
      setError(cause instanceof DOMException && cause.name === 'NotAllowedError'
        ? 'Microphone permission was denied. Allow microphone access in your browser settings or upload audio below.'
        : 'We could not open your microphone. Check that it is connected and available, or upload audio below.');
    }
  };
  return { status, seconds, file, error, start, stop, setFile, setError };
}
