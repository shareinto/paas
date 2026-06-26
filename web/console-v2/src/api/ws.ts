// src/api/ws.ts

export interface WSMessage {
  type: string;
  app_id: string;
  payload?: unknown;
}

type MessageHandler = (msg: WSMessage) => void;
type ReconnectHandler = () => void;

class PaaSWebSocket {
  private ws: WebSocket | null = null;
  private url: string;
  private subscribedApps = new Set<string>();
  private handlers: MessageHandler[] = [];
  private reconnectHandlers: ReconnectHandler[] = [];
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private closed = false;

  constructor() {
    const base = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';
    this.url = base.replace(/^http/, 'ws') + '/api/console-v2/ws';
  }

  connect() {
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) return;
    this.closed = false;
    try {
      this.ws = new WebSocket(this.url);
    } catch {
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      this.reconnectDelay = 1000;
      for (const appId of this.subscribedApps) {
        this.send({ type: 'subscribe', app_id: appId });
      }
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        for (const handler of this.handlers) {
          handler(msg);
        }
      } catch { /* ignore parse errors */ }
    };

    this.ws.onclose = () => {
      if (!this.closed) this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  disconnect() {
    this.closed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }

  subscribe(appId: string) {
    this.subscribedApps.add(appId);
    this.send({ type: 'subscribe', app_id: appId });
  }

  unsubscribe(appId: string) {
    this.subscribedApps.delete(appId);
    this.send({ type: 'unsubscribe', app_id: appId });
  }

  onMessage(handler: MessageHandler): () => void {
    this.handlers.push(handler);
    return () => {
      this.handlers = this.handlers.filter(h => h !== handler);
    };
  }

  onReconnect(handler: ReconnectHandler): () => void {
    this.reconnectHandlers.push(handler);
    return () => {
      this.reconnectHandlers = this.reconnectHandlers.filter(h => h !== handler);
    };
  }

  private send(msg: { type: string; app_id: string }) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  private scheduleReconnect() {
    if (this.closed || this.reconnectTimer) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
      for (const handler of this.reconnectHandlers) {
        handler();
      }
    }, this.reconnectDelay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
  }
}

export const paasWS = new PaaSWebSocket();
