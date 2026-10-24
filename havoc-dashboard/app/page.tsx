'use client';
import { useEffect, useState, useCallback } from 'react';
import StatusCard, { ServiceStatus } from '@/components/StatusCard';
import ExperimentButton from '@/components/ExperimentButton';
import LiveLogPanel from '@/components/LiveLogPanel';
import BlastRadiusGraph from '@/components/BlastRadiusGraph';

export default function Home() {
  const [services, setServices] = useState<ServiceStatus[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isOffline, setIsOffline] = useState(false);

  const fetchStatus = useCallback(() => {
    setIsLoading(true);
    fetch('/api/status')
      .then(async (res) => {
        if (!res.ok) {
          setIsOffline(true);
          setServices([]);
          setIsLoading(false);
          return;
        }
        const data = await res.json();
        if (data.offline) {
          setIsOffline(true);
          setServices([]);
        } else {
          setIsOffline(false);
          setServices(Array.isArray(data) ? data : []);
        }
        setIsLoading(false);
      })
      .catch((err) => {
        console.error('Failed to fetch status', err);
        setIsOffline(true);
        setServices([]);
        setIsLoading(false);
      });
  }, []);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  return (
    <div className="min-h-screen bg-zinc-950 p-6 md:p-8 lg:p-10 text-zinc-100">
      <div className="max-w-7xl mx-auto space-y-6">
        <header className="border-b border-zinc-800 pb-6 flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
              <svg className="w-7 h-7 text-red-500" fill="currentColor" viewBox="0 0 24 24">
                <path d="M12 2L2 22h20L12 2zm0 4.5l6.5 13h-13L12 6.5zM11 10v5h2v-5h-2zm0 6v2h2v-2h-2z" />
              </svg>
              Havoc Engine
            </h1>
            <p className="text-zinc-400 mt-1 text-sm font-medium">Chaos Engineering Control Plane</p>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-zinc-900 border border-zinc-800 text-xs font-medium text-zinc-400">
              <div
                className={`w-2 h-2 rounded-full ${
                  isOffline
                    ? 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.8)]'
                    : 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.8)]'
                }`}
              />
              Engine:{' '}
              <span className={isOffline ? 'text-red-400 font-semibold' : 'text-green-400 font-semibold'}>
                {isOffline ? 'Offline' : 'Online'}
              </span>
            </div>
            <div className="px-3 py-1.5 rounded-full bg-zinc-900 border border-zinc-800 text-xs font-medium text-zinc-400">
              Environment: <span className="text-zinc-200">Kubernetes</span>
            </div>
          </div>
        </header>

        {isOffline && (
          <div className="p-4 rounded-xl border border-red-500/30 bg-red-500/10 text-red-300 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 animate-in fade-in duration-300">
            <div className="flex items-start gap-3">
              <div className="w-2.5 h-2.5 rounded-full bg-red-500 mt-1 shadow-[0_0_8px_rgba(239,68,68,0.8)] animate-pulse shrink-0" />
              <div>
                <h3 className="font-semibold text-red-200 text-sm">Engine Offline</h3>
                <p className="text-xs text-red-300/80 mt-0.5">
                  Havoc Engine API is unreachable. Start the backend with{' '}
                  <code className="bg-red-950/60 px-1 py-0.5 rounded text-red-200 font-mono">havoc-engine serve</code>{' '}
                  to access live cluster deployments and real-time chaos experiments.
                </p>
              </div>
            </div>
            <button
              onClick={fetchStatus}
              className="text-xs px-3 py-1.5 rounded-lg bg-red-500/20 hover:bg-red-500/30 text-red-200 border border-red-500/40 font-medium shrink-0 transition-colors"
            >
              Retry Connection
            </button>
          </div>
        )}

        <main className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div className="lg:col-span-8 space-y-8">
            <section>
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-lg font-semibold flex items-center gap-2 tracking-tight">
                  <svg className="w-5 h-5 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                    />
                  </svg>
                  System Status
                </h2>
                {!isOffline && (
                  <span className="text-xs text-zinc-500 font-mono">
                    {services.length} {services.length === 1 ? 'deployment' : 'deployments'} active
                  </span>
                )}
              </div>

              {isLoading ? (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  {[1, 2, 3, 4].map((i) => (
                    <div key={i} className="animate-pulse bg-zinc-900/50 rounded-xl h-36 border border-zinc-800" />
                  ))}
                </div>
              ) : isOffline ? (
                <div className="p-8 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20 text-center">
                  <svg
                    className="w-10 h-10 mx-auto mb-3 text-zinc-600"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={1.5}
                      d="M18.364 5.636a9 9 0 010 12.728m0 0l-2.829-2.829m2.829 2.829L21 21M15.536 8.464a5 5 0 010 7.072m0 0l-2.829-2.829m-4.243 4.243a9 9 0 01-2.828-6.364m0 0a9 9 0 012.828-6.364m6.364 6.364a5 5 0 00-7.071 0"
                    />
                  </svg>
                  <p className="text-zinc-400 font-medium text-sm">Cluster Status Unavailable</p>
                  <p className="text-zinc-600 text-xs mt-1">
                    Live deployment telemetry will appear once the Havoc Engine connects.
                  </p>
                </div>
              ) : services.length === 0 ? (
                <div className="p-8 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20 text-center">
                  <p className="text-zinc-400 font-medium text-sm">No Deployments Found</p>
                  <p className="text-zinc-600 text-xs mt-1">
                    Apply Kubernetes manifests in target namespace(s) or check target_namespaces in config.yaml.
                  </p>
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  {services.map((svc) => (
                    <StatusCard key={svc.name} {...svc} />
                  ))}
                </div>
              )}
            </section>

            <section>
              <h2 className="text-lg font-semibold mb-4 flex items-center gap-2 tracking-tight">
                <svg className="w-5 h-5 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M13 10V3L4 14h7v7l9-11h-7z"
                  />
                </svg>
                Chaos Controls
              </h2>
              <div className="bg-zinc-900/50 border border-zinc-800 rounded-xl p-6 backdrop-blur-sm">
                <div className="flex flex-wrap gap-3">
                  <ExperimentButton label="Kill Pod" action="kill-pod" variant="danger" />
                  <ExperimentButton label="Inject Latency" action="inject-latency" variant="warning" />
                  <ExperimentButton label="Spike CPU" action="spike-cpu" variant="warning" />
                  <ExperimentButton label="Memory Leak" action="memory-leak" variant="warning" />
                </div>
                <div className="mt-5 pt-5 border-t border-zinc-800/80">
                  <p className="text-xs text-zinc-400 font-medium flex items-start gap-2">
                    <svg
                      className="w-4 h-4 text-yellow-500 shrink-0"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                    Warning: Triggering experiments will actively affect target workloads. The safety runner will
                    enforce blast radius limits and auto-abort on excessive error rates.
                  </p>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-lg font-semibold mb-4 flex items-center gap-2 tracking-tight">
                <svg className="w-5 h-5 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
                  />
                </svg>
                Blast Radius
              </h2>
              <BlastRadiusGraph />
            </section>
          </div>

          <div className="lg:col-span-4 h-[600px] lg:h-[calc(100vh-140px)] sticky top-8">
            <LiveLogPanel />
          </div>
        </main>
      </div>
    </div>
  );
}
