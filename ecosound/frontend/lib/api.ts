export const API_BASE = (process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080').replace(/\/$/, '');
export const MAX_AUDIO_BYTES = 15 * 1024 * 1024;
export const CONFIRMATION_COPY = 'Personally confirmed — not independently scientifically verified.';
export type Prediction = { id: number; scientific_name: string; common_name: string; raw_species_name: string; confidence: number; start_time: number | null; end_time: number | null; user_confirmed: boolean };
export type Identification = { recording_id: string; audio_url: string; duration_seconds: number; model: string; model_version: string; predictions: Prediction[] };
export type Discovery = { id: number; recording_id: string; prediction_id: number; scientific_name: string; common_name: string; confidence: number; observed_at: string; user_confirmed: boolean; audio_url: string };
export type Quest = { name: string; date: string; timezone: string; completed: boolean; tasks: { name: string; current: number; target: number; completed: boolean }[] };
export class ApiError extends Error { constructor(message: string, public code: string, public status: number) { super(message); this.name = 'ApiError'; } }
export function audioUrl(path: string) { return new URL(path, `${API_BASE}/`).toString(); }
export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  let response: Response;
  try { response = await fetch(`${API_BASE}${path}`, options); }
  catch { throw new ApiError('EcoSound could not reach the server. Check that the backend is running and try again.', 'network_error', 0); }
  let body;
  try { body = await response.json(); }
  catch { throw new ApiError('The server returned an unreadable response. Please try again.', 'invalid_response', response.status); }
  if (!response.ok) throw new ApiError(body?.error?.message || 'Something went wrong. Please try again.', body?.error?.code || 'request_failed', response.status);
  return body as T;
}
export function strongestCandidates(predictions: Prediction[]) {
  const grouped = new Map<string, Prediction>();
  for (const prediction of predictions) {
    const key = prediction.scientific_name || prediction.raw_species_name;
    if (!grouped.has(key) || prediction.confidence > grouped.get(key)!.confidence) grouped.set(key, prediction);
  }
  return [...grouped.values()].sort((a, b) => b.confidence - a.confidence);
}
