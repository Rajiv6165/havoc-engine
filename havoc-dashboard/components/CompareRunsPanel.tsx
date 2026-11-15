'use client';
import React, { useEffect, useState, useCallback } from 'react';
import { ExperimentRecord } from './ExperimentHistoryPanel';

interface LabelStats {
  label: string;
  runCount: number;
  avgScore: number;
  avgRecoveryMs: number;
  p95RecoveryMs?: number;
  safetyInterventions: number;
}

interface ComparisonDelta {
  scoreDelta: number;
  recoveryMsDelta: number;
  p95RecoveryMsDelta?: number;
  safetyInterventionsDelta: number;
}

interface ComparisonResult {
  labelA: LabelStats;
  labelB: LabelStats;
  difference: ComparisonDelta;
  warning?: string;
  hasSufficientData: boolean;
  error?: string;
}

export default function CompareRunsPanel() {
  const [labels, setLabels] = useState<string[]>([]);
  const [labelA, setLabelA] = useState<string>('');
  const [labelB, setLabelB] = useState<string>('');
  const [comparison, setComparison] = useState<ComparisonResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);

  // Discover labels from experiments
  const refreshLabels = useCallback(async () => {
    try {
      const res = await fetch('/api/experiments?limit=50', { cache: 'no-store' });
      if (res.ok) {
        const data: ExperimentRecord[] = await res.json();
        if (Array.isArray(data)) {
          const unique = Array.from(
            new Set(
              data
                .filter((e) => !e.Seeded && Boolean(e.Label && e.Label.trim()))
                .map((e) => e.Label as string)
            )
          );
          setLabels(unique);

          // Select defaults if not chosen
          if (unique.length >= 2) {
            setLabelA((prev) => (prev && unique.includes(prev) ? prev : (unique.includes('baseline') ? 'baseline' : unique[0])));
            setLabelB((prev) => (prev && unique.includes(prev) ? prev : (unique.includes('hardened') ? 'hardened' : unique[1])));
          } else if (unique.length === 1) {
            setLabelA((prev) => (prev || unique[0]));
          }
        }
      }
    } catch {
      // Ignore background label fetch errors
    }
  }, []);

  useEffect(() => {
    refreshLabels();
  }, [refreshLabels]);

  // Fetch comparison when labelA and labelB are selected
  useEffect(() => {
    if (!labelA || !labelB || labelA === labelB) {
      setComparison(null);
      return;
    }

    let isMounted = true;
    setIsLoading(true);
    setFetchError(null);

    fetch(`/api/compare?a=${encodeURIComponent(labelA)}&b=${encodeURIComponent(labelB)}`, {
      cache: 'no-store',
    })
      .then(async (res) => {
        const data = await res.json();
        if (!isMounted) return;
        if (!res.ok) {
          setFetchError(data.error || `HTTP ${res.status}`);
          setComparison(null);
        } else {
          setComparison(data);
          setFetchError(null);
        }
        setIsLoading(false);
      })
      .catch((err: unknown) => {
        if (!isMounted) return;
        const msg = err instanceof Error ? err.message : 'Failed to fetch comparison';
        setFetchError(msg);
        setComparison(null);
        setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [labelA, labelB]);

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-6 backdrop-blur-sm">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div>
          <h2 className="text-lg font-semibold flex items-center gap-2 tracking-tight">
            <svg className="w-5 h-5 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
              />
            </svg>
            Compare Runs
          </h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Statistical resilience comparison across deployment architectures
          </p>
        </div>

        {/* Label Dropdowns */}
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-1.5 text-xs">
            <span className="text-zinc-500 font-medium">A:</span>
            <select
              value={labelA}
              onChange={(e) => setLabelA(e.target.value)}
              className="bg-zinc-950 border border-zinc-800 rounded-lg px-2.5 py-1.5 text-xs font-mono text-zinc-200 focus:outline-none focus:border-zinc-600"
            >
              <option value="">Select label A</option>
              {labels.map((l) => (
                <option key={`a-${l}`} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </div>

          <span className="text-zinc-600 text-xs">vs</span>

          <div className="flex items-center gap-1.5 text-xs">
            <span className="text-zinc-500 font-medium">B:</span>
            <select
              value={labelB}
              onChange={(e) => setLabelB(e.target.value)}
              className="bg-zinc-950 border border-zinc-800 rounded-lg px-2.5 py-1.5 text-xs font-mono text-zinc-200 focus:outline-none focus:border-zinc-600"
            >
              <option value="">Select label B</option>
              {labels.map((l) => (
                <option key={`b-${l}`} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* State 1: Insufficient labels discovered */}
      {labels.length < 2 && (
        <div className="p-6 rounded-lg border border-dashed border-zinc-800 text-center bg-zinc-950/30">
          <svg className="w-8 h-8 text-zinc-600 mx-auto mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
          </svg>
          <p className="text-sm font-medium text-zinc-300">Need At Least Two Labeled Experiment Sets</p>
          <p className="text-xs text-zinc-500 mt-1 max-w-md mx-auto">
            Run experiments with <code className="text-zinc-400">--label=baseline</code> and <code className="text-zinc-400">--label=hardened</code> to populate real comparison data.
          </p>
        </div>
      )}

      {/* State 2: Error fetching comparison */}
      {labels.length >= 2 && fetchError && (
        <div className="p-4 rounded-lg bg-red-500/10 border border-red-500/30 text-red-300 text-xs">
          {fetchError}
        </div>
      )}

      {/* State 3: Loading */}
      {isLoading && (
        <div className="p-8 text-center text-zinc-500 text-xs font-mono">
          Calculating statistical comparison...
        </div>
      )}

      {/* State 4: Same labels selected */}
      {!isLoading && !fetchError && labelA && labelB && labelA === labelB && (
        <div className="p-6 rounded-lg border border-zinc-800 text-center text-zinc-500 text-xs font-mono">
          Please select two different labels to compare.
        </div>
      )}

      {/* State 5: Too few runs (< 3 runs per label) */}
      {!isLoading && !fetchError && comparison && !comparison.hasSufficientData && (
        <div className="p-6 rounded-lg border border-dashed border-amber-500/30 bg-amber-500/5 text-center">
          <svg className="w-8 h-8 text-amber-400 mx-auto mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <p className="text-sm font-medium text-amber-300">Insufficient Sample Size</p>
          <p className="text-xs text-amber-400/80 mt-1 max-w-md mx-auto">
            {comparison.warning || `Fewer than 3 real runs found per label. At least 3 runs per variant are required for reliable comparison.`}
          </p>
          <div className="mt-3 flex justify-center gap-4 text-xs font-mono text-zinc-400">
            <span>
              {comparison.labelA.label}: <strong className="text-zinc-200">{comparison.labelA.runCount}</strong> runs
            </span>
            <span>•</span>
            <span>
              {comparison.labelB.label}: <strong className="text-zinc-200">{comparison.labelB.runCount}</strong> runs
            </span>
          </div>
        </div>
      )}

      {/* State 6: Full comparison data */}
      {!isLoading && !fetchError && comparison && comparison.hasSufficientData && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            {/* Resilience Score Card */}
            <div className="p-4 rounded-lg bg-zinc-950 border border-zinc-800">
              <span className="text-xs text-zinc-400 font-medium">Avg Resilience Score</span>
              <div className="mt-2 flex items-baseline justify-between">
                <div>
                  <span className="text-xs text-zinc-500 font-mono">{comparison.labelA.label}:</span>{' '}
                  <span className="text-lg font-bold font-mono text-zinc-300">{comparison.labelA.avgScore}</span>
                </div>
                <div>
                  <span className="text-xs text-zinc-500 font-mono">{comparison.labelB.label}:</span>{' '}
                  <span className="text-lg font-bold font-mono text-white">{comparison.labelB.avgScore}</span>
                </div>
              </div>
              <div className="mt-2 pt-2 border-t border-zinc-800/80 flex justify-between items-center text-xs">
                <span className="text-zinc-500">Difference:</span>
                <span
                  className={`font-mono font-semibold ${
                    comparison.difference.scoreDelta > 0
                      ? 'text-emerald-400'
                      : comparison.difference.scoreDelta < 0
                      ? 'text-red-400'
                      : 'text-zinc-400'
                  }`}
                >
                  {comparison.difference.scoreDelta > 0 ? `+${comparison.difference.scoreDelta}` : comparison.difference.scoreDelta}
                </span>
              </div>
            </div>

            {/* Recovery Time Card */}
            <div className="p-4 rounded-lg bg-zinc-950 border border-zinc-800">
              <span className="text-xs text-zinc-400 font-medium">Avg Recovery Time</span>
              <div className="mt-2 flex items-baseline justify-between">
                <div>
                  <span className="text-xs text-zinc-500 font-mono">{comparison.labelA.label}:</span>{' '}
                  <span className="text-lg font-bold font-mono text-zinc-300">{comparison.labelA.avgRecoveryMs}ms</span>
                </div>
                <div>
                  <span className="text-xs text-zinc-500 font-mono">{comparison.labelB.label}:</span>{' '}
                  <span className="text-lg font-bold font-mono text-white">{comparison.labelB.avgRecoveryMs}ms</span>
                </div>
              </div>
              <div className="mt-2 pt-2 border-t border-zinc-800/80 flex justify-between items-center text-xs">
                <span className="text-zinc-500">Difference:</span>
                <span
                  className={`font-mono font-semibold ${
                    comparison.difference.recoveryMsDelta < 0
                      ? 'text-emerald-400'
                      : comparison.difference.recoveryMsDelta > 0
                      ? 'text-red-400'
                      : 'text-zinc-400'
                  }`}
                >
                  {comparison.difference.recoveryMsDelta > 0 ? `+${comparison.difference.recoveryMsDelta}ms` : `${comparison.difference.recoveryMsDelta}ms`}
                </span>
              </div>
            </div>

            {/* P95 Recovery / Interventions Card */}
            <div className="p-4 rounded-lg bg-zinc-950 border border-zinc-800">
              <span className="text-xs text-zinc-400 font-medium">P95 Recovery / Safety</span>
              <div className="mt-2 flex items-baseline justify-between text-xs font-mono">
                <div>
                  <span className="text-zinc-500">{comparison.labelA.label}:</span>{' '}
                  <span className="text-zinc-300">
                    {comparison.labelA.p95RecoveryMs != null ? `${comparison.labelA.p95RecoveryMs}ms` : 'N/A'}
                  </span>
                  <span className="text-zinc-600 block text-[11px]">
                    {comparison.labelA.safetyInterventions} safety stops
                  </span>
                </div>
                <div className="text-right">
                  <span className="text-zinc-500">{comparison.labelB.label}:</span>{' '}
                  <span className="text-white">
                    {comparison.labelB.p95RecoveryMs != null ? `${comparison.labelB.p95RecoveryMs}ms` : 'N/A'}
                  </span>
                  <span className="text-zinc-600 block text-[11px]">
                    {comparison.labelB.safetyInterventions} safety stops
                  </span>
                </div>
              </div>
              <div className="mt-2 pt-2 border-t border-zinc-800/80 flex justify-between items-center text-xs">
                <span className="text-zinc-500">Runs Analyzed:</span>
                <span className="font-mono text-zinc-400">
                  {comparison.labelA.runCount} vs {comparison.labelB.runCount}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
