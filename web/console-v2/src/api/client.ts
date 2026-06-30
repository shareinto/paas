export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const DEFAULT_ACTOR_ID = import.meta.env.VITE_ACTOR_ID || 'usr_admin';

export type PageResult<T> = {
  items: T[];
  total?: number;
  page?: number;
  page_size?: number;
};

export class APIError extends Error {
  code: string;
  status?: number;

  constructor(code: string, message: string, status?: number) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

/**
 * Returns true when the app should use real backend APIs instead of mock data.
 * True when either VITE_API_BASE_URL is set (direct backend) or VITE_USE_REAL_API is set (proxy mode).
 */
export function hasAPIBaseURL() {
  return API_BASE_URL.trim() !== '' || import.meta.env.VITE_USE_REAL_API === 'true';
}

export function actorId() {
  return window.localStorage.getItem('paas_actor_id') || DEFAULT_ACTOR_ID;
}

export function actorQuery() {
  return `actor_id=${encodeURIComponent(actorId())}`;
}

export function actorBody() {
  return { type: 'user', id: actorId() };
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = window.localStorage.getItem('paas_token') || '';
  const isFormData = options.body instanceof FormData;
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers
    }
  });

  const text = await response.text();
  const payload = parseJSONPayload(text);
  if (!response.ok) {
    const error = payload?.error;
    throw new APIError(error?.code || 'request_failed', error?.message || `请求失败 (HTTP ${response.status})`, response.status);
  }
  return payload as T;
}

export async function requestText(path: string, options: RequestInit = {}): Promise<string> {
  const token = window.localStorage.getItem('paas_token') || '';
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers
    }
  });

  const text = await response.text();
  if (!response.ok) {
    const payload = parseJSONPayload(text);
    const error = payload?.error;
    throw new APIError(error?.code || 'request_failed', error?.message || text || '请求处理失败', response.status);
  }
  return text;
}

export type SSEEvent = { event: string; data: string };

export function streamSSE(path: string, onEvent: (event: SSEEvent) => void, onError?: (error: Error) => void) {
  const controller = new AbortController();
  const token = window.localStorage.getItem('paas_token') || '';

  const emitBlock = (block: string) => {
    const lines = block.split(/\r?\n/);
    let event = 'message';
    const data: string[] = [];
    for (const line of lines) {
      if (line.startsWith('event:')) event = line.slice('event:'.length).trim();
      if (line.startsWith('data:')) data.push(line.slice('data:'.length).replace(/^\s/, ''));
    }
    if (data.length > 0) onEvent({ event, data: data.join('\n') });
  };

  const run = async () => {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      signal: controller.signal
    });

    if (!response.ok) {
      throw new APIError('request_failed', '日志连接失败', response.status);
    }

    if (!response.body) {
      const text = await response.text();
      text.split(/\n\n/).filter(Boolean).forEach(emitBlock);
      return;
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const blocks = buffer.split(/\n\n/);
      buffer = blocks.pop() || '';
      blocks.filter(Boolean).forEach(emitBlock);
    }
    buffer += decoder.decode();
    if (buffer.trim()) emitBlock(buffer);
  };

  run().catch((error) => {
    if (controller.signal.aborted) return;
    onError?.(error instanceof Error ? error : new Error('日志连接失败'));
  });

  return () => controller.abort();
}

function parseJSONPayload(text: string) {
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}
