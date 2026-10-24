'use client';
import React, { useEffect, useState, useCallback } from 'react';
import { wsClient } from '@/lib/websocket';

export interface ExperimentRecord {
  ID: string;
  ExperimentType: string;
  TargetNamespace: string;
  RecoveryTimeMs: number | null;
  ErrorRatePercent: number | null;
  SafetyIntervened: boolean;
  ResilienceScore: number;
  PostmortemReport: string | null;
  CreatedAt: string;
}

export default function ExperimentHistoryPanel() {
  const [experiments, setExperiments] = useState<ExperimentRecord[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const fetchHistory = useCallback(async () => {
    setIsLoading(true);
    try {
      const res = await fetch('/api/experiments?limit=10', { cache: 'no-store' });
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data)) {
          setExperiments(data);
        } else {
          setExperiments([]);
        }
      } else {
        setExperiments([]);
      }
    } catch {
      setExperiments([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchHistory();

    if (!wsClient) return;
    const unsubscribe = wsClient.onMessage((data) => {
      const eventType = data.type || '';
      const msg = typeof data.message === 'string' ? data.message.toLowerCase() : '';
      if (eventType === 'completed' || msg.includes('experiment ended')) {
        // Refresh history when an experiment finishes
        fetchHistory();
      }
    });

    return () => {
      unsubscribe();
    };
  }, [fetchHistory]);

  const toggleExpand = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  };

  const getScoreColor = (score: number) => {
    if (score >= 80) return 'bg-green-500/10 text-green-400 border-green-500/30';
    if (score >= 60) return 'bg-yellow-500/10 text-yellow-400 border-yellow-500/30';
    return 'bg-red-500/10 text-red-400 border-red-500/30';
  };

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-6 backdrop-blur-sm space-y-4">
      <div className="flex items-center justify-between pb-3 border-b border-zinc-800">
        <div>
          <h2 className="text-lg font-semibold flex items-center gap-2 tracking-tight text-zinc-100">
            <svg className="w-5 h-5 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Experiment History & Resilience Scores
          </h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Audit log of chaos experiments, recovery metrics, and AI postmortems
          </p>
        </div>
        <button
          onClick={fetchHistory}
          disabled={isLoading}
          className="text-xs px-2.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 border border-zinc-700 font-medium transition-colors flex items-center gap-1.5"
        >
          <svg className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </button>
      </div>

      {isLoading && experiments.length === 0 ? (
        <div className="space-y-3">
          {[1, 2].map((i) => (
            <div key={i} className="animate-pulse bg-zinc-900/80 rounded-lg h-20 border border-zinc-800" />
          ))}
        </div>
      ) : experiments.length === 0 ? (
        <div className="p-8 rounded-lg border border-dashed border-zinc-800 text-center">
          <p className="text-zinc-400 text-sm font-medium">No Experiment History Recorded</p>
          <p className="text-zinc-600 text-xs mt-1">
            Trigger a chaos action above or run experiments via CLI to view scores and AI postmortem reports.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {experiments.map((exp) => {
            const isExpanded = expandedId === exp.ID;
            return (
              <div
                key={exp.ID}
                className="rounded-lg border border-zinc-800 bg-zinc-950/60 p-4 transition-all hover:border-zinc-700"
              >
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="px-2 py-0.5 rounded text-xs font-mono font-medium bg-zinc-800 text-zinc-200 border border-zinc-700">
                        {exp.ExperimentType}
                      </span>
                      <span className="text-xs text-zinc-400 font-mono">
                        ns: <span className="text-zinc-300">{exp.TargetNamespace || 'default'}</span>
                      </span>
                      <span className="text-xs text-zinc-500">
                        {exp.CreatedAt ? new Date(exp.CreatedAt).toLocaleTimeString() : ''}
                      </span>
                    </div>
                    <div className="flex items-center gap-4 text-xs text-zinc-400 pt-1 font-mono">
                      <span>
                        Recovery:{' '}
                        <span className="text-zinc-200">
                          {exp.RecoveryTimeMs !== null ? `${exp.RecoveryTimeMs}ms` : 'N/A'}
                        </span>
                      </span>
                      <span>
                        Error Rate:{' '}
                        <span className="text-zinc-200">
                          {exp.ErrorRatePercent !== null ? `${exp.ErrorRatePercent.toFixed(1)}%` : '0%'}
                        </span>
                      </span>
                      {exp.SafetyIntervened && (
                        <span className="px-1.5 py-0.5 rounded text-[10px] bg-amber-500/10 text-amber-300 border border-amber-500/30">
                          Safety Intervened
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-3 self-end sm:self-center">
                    <div className={`px-3 py-1.5 rounded-lg border text-xs font-mono font-semibold ${getScoreColor(exp.ResilienceScore)}`}>
                      Score: {exp.ResilienceScore}/100
                    </div>
                    {exp.PostmortemReport && (
                      <button
                        onClick={() => toggleExpand(exp.ID)}
                        className="text-xs px-2.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 border border-zinc-700 transition-colors font-medium flex items-center gap-1"
                      >
                        {isExpanded ? 'Hide Report' : 'AI Postmortem'}
                        <svg
                          className={`w-3.5 h-3.5 transition-transform ${isExpanded ? 'rotate-180' : ''}`}
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </button>
                    )}
                  </div>
                </div>

                {isExpanded && exp.PostmortemReport && (
                  <div className="mt-4 pt-4 border-t border-zinc-800/80 animate-in fade-in duration-200">
                    <div className="flex items-center gap-1.5 text-xs text-purple-400 font-semibold mb-2">
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                      </svg>
                      Claude AI Postmortem Analysis
                    </div>
                    <pre className="text-xs text-zinc-300 bg-zinc-900/90 p-4 rounded-lg font-mono whitespace-pre-wrap border border-zinc-800 leading-relaxed overflow-x-auto">
                      {exp.PostmortemReport}
                    </pre>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
