// eslint-disable-next-line @typescript-eslint/no-explicit-any
type MessageHandler = (data: any) => void;

export class HavocWebSocket {
  private ws: WebSocket | null = null;
  private url: string;
  private onMessageHandlers: Set<MessageHandler> = new Set();
  private onStatusChangeHandlers: Set<(status: 'connected' | 'disconnected' | 'connecting') => void> = new Set();
  private reconnectTimer: NodeJS.Timeout | null = null;

  constructor(url: string) {
    this.url = url;
  }

  connect() {
    this.updateStatus('connecting');
    try {
      this.ws = new WebSocket(this.url);

      this.ws.onopen = () => {
        this.updateStatus('connected');
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer);
          this.reconnectTimer = null;
        }
      };

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          this.onMessageHandlers.forEach(handler => handler(data));
        } catch {
          this.onMessageHandlers.forEach(handler => handler({ message: event.data, timestamp: new Date().toISOString() }));
        }
      };

      this.ws.onclose = () => {
        this.updateStatus('disconnected');
        this.scheduleReconnect();
      };

      this.ws.onerror = () => {
        this.updateStatus('disconnected');
        this.ws?.close();
      };
    } catch {
      this.updateStatus('disconnected');
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect() {
    if (!this.reconnectTimer) {
      this.reconnectTimer = setTimeout(() => {
        this.reconnectTimer = null;
        this.connect();
      }, 5000);
    }
  }

  private updateStatus(status: 'connected' | 'disconnected' | 'connecting') {
    this.onStatusChangeHandlers.forEach(handler => handler(status));
  }

  onMessage(handler: MessageHandler) {
    this.onMessageHandlers.add(handler);
    return () => {
      this.onMessageHandlers.delete(handler);
    };
  }

  onStatusChange(handler: (status: 'connected' | 'disconnected' | 'connecting') => void) {
    this.onStatusChangeHandlers.add(handler);
    return () => {
      this.onStatusChangeHandlers.delete(handler);
    };
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

export const wsClient = typeof window !== 'undefined' ? new HavocWebSocket('ws://localhost:9091') : null;
