'use client';
import React, { useEffect, useState, useRef } from 'react';
import { wsClient } from '@/lib/websocket';

interface LogMessage {
  id: string;
  timestamp: string;
  level: 'info' | 'warn' | 'error';
  message: string;
}

export default function LiveLogPanel() {
  const [logs, setLogs] = useState<LogMessage[]>([]);
  const [status, setStatus] = useState<'connected' | 'disconnected' | 'connecting'>('disconnected');
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!wsClient) return;

    wsClient.connect();

    const unsubscribeStatus = wsClient.onStatusChange((newStatus) => {
      setStatus(newStatus);
    });

    const unsubscribeMessage = wsClient.onMessage((data) => {
      const newLog: LogMessage = {
        id: Math.random().toString(36).substring(2, 11),
        timestamp: data.timestamp || new Date().toISOString(),
        level: data.level || 'info',
        message: data.message || (typeof data === 'string' ? data : JSON.stringify(data)),
      };
      setLogs((prev) => [...prev.slice(-99), newLog]); // Keep last 100 logs
    });

    return () => {
      unsubscribeStatus();
      unsubscribeMessage();
      wsClient?.disconnect();
    };
  }, []);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  const statusColors = {
    connected: 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]',
    disconnected: 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.6)]',
    connecting: 'bg-yellow-500 shadow-[0_0_8px_rgba(234,179,8,0.6)]',
  };

  return (
    <div className="flex flex-col h-full rounded-xl border border-zinc-800 bg-zinc-950 overflow-hidden">
      <div className="flex justify-between items-center px-4 py-3 border-b border-zinc-800 bg-zinc-900/50">
        <h3 className="font-semibold text-zinc-100 text-sm flex items-center gap-2">
          <svg className="w-4 h-4 text-zinc-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h7" /></svg>
          Live Experiment Logs
        </h3>
        <div className="flex items-center gap-2 text-xs font-medium text-zinc-400">
          <div className={`w-2 h-2 rounded-full ${statusColors[status]} ${status === 'connecting' ? 'animate-pulse' : ''}`} />
          {status.charAt(0).toUpperCase() + status.slice(1)}
        </div>
      </div>
      <div 
        ref={scrollRef}
        className="flex-1 overflow-y-auto p-4 space-y-1 font-mono text-xs sm:text-sm"
      >
        {logs.length === 0 ? (
          <div className="text-zinc-600 italic">Waiting for events...</div>
        ) : (
          logs.map((log) => (
            <div key={log.id} className="flex items-start gap-3 hover:bg-zinc-900/50 px-2 py-1 rounded transition-colors">
              <span className="text-zinc-500 shrink-0">
                {new Date(log.timestamp).toLocaleTimeString(undefined, { hour12: false })}
              </span>
              <span className={`shrink-0 w-12 ${
                log.level === 'error' ? 'text-red-400' : 
                log.level === 'warn' ? 'text-yellow-400' : 
                'text-blue-400'
              }`}>
                [{log.level.toUpperCase()}]
              </span>
              <span className="text-zinc-300 break-all">{log.message}</span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
