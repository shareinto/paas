import { useSession } from '../app/store';

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';

export type PageResult<T> = { items: T[]; total: number; page: number; page_size: number };

export class APIError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
  }
}

export function hasAPIBaseURL() {
  return API_BASE_URL.trim() !== '';
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = useSession.getState().token;
  const isFormData = options.body instanceof FormData;
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers
    }
  });
  if (response.status === 401) {
    useSession.getState().clear();
    throw new APIError('unauthenticated', '会话已过期，请重新登录');
  }
  const text = await response.text();
  const payload = text ? JSON.parse(text) : undefined;
  if (!response.ok) {
    const error = payload?.error;
    throw new APIError(error?.code || 'request_failed', error?.message || '请求处理失败');
  }
  return payload as T;
}

export async function requestText(path: string, options: RequestInit = {}): Promise<string> {
  const token = useSession.getState().token;
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers
    }
  });
  const text = await response.text();
  if (!response.ok) {
    throw new APIError('request_failed', '请求处理失败');
  }
  return text;
}

export type SSEEvent = { event: string; data: string };

export function streamSSE(path: string, onEvent: (event: SSEEvent) => void, onError?: (error: Error) => void) {
  const controller = new AbortController();
  const token = useSession.getState().token;
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
    if (response.status === 401) {
      useSession.getState().clear();
      throw new APIError('unauthenticated', '会话已过期，请重新登录');
    }
    if (!response.ok) {
      throw new APIError('request_failed', '请求处理失败');
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
