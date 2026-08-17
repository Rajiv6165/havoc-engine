import React from 'react';

export type Status = 'healthy' | 'degraded' | 'critical';

export interface ServiceStatus {
  name: string;
  podCount: number;
  status: Status;
}

export default function StatusCard({ name, podCount, status }: ServiceStatus) {
  const statusColors = {
    healthy: 'bg-green-500/10 text-green-400 border-green-500/20',
    degraded: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
    critical: 'bg-red-500/10 text-red-400 border-red-500/20',
  };

  const dotColors = {
    healthy: 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]',
    degraded: 'bg-yellow-500 shadow-[0_0_8px_rgba(234,179,8,0.6)]',
    critical: 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.6)]',
  };

  return (
    <div className="flex flex-col p-5 rounded-xl border border-zinc-800 bg-zinc-900/50 hover:bg-zinc-800/50 transition-colors backdrop-blur-sm">
      <div className="flex justify-between items-start mb-6">
        <h3 className="font-semibold text-zinc-100 tracking-tight">{name}</h3>
        <div className={`px-2.5 py-1 rounded-full border text-xs font-medium flex items-center gap-2 ${statusColors[status]}`}>
          <div className={`w-2 h-2 rounded-full ${dotColors[status]}`} />
          {status.charAt(0).toUpperCase() + status.slice(1)}
        </div>
      </div>
      <div className="mt-auto">
        <p className="text-xs text-zinc-400 font-medium uppercase tracking-wider">Pods Running</p>
        <p className="text-3xl font-mono font-medium mt-1 text-zinc-200">{podCount}</p>
      </div>
    </div>
  );
}
