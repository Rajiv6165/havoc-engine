'use client';
import React, { useState } from 'react';

interface ExperimentButtonProps {
  label: string;
  action: string;
  variant?: 'danger' | 'warning' | 'default';
  disabled?: boolean;
  tooltip?: string;
}

export default function ExperimentButton({
  label,
  action,
  variant = 'default',
  disabled = false,
  tooltip,
}: ExperimentButtonProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [status, setStatus] = useState<'idle' | 'success' | 'error'>('idle');

  const triggerExperiment = async () => {
    if (disabled) return;
    setIsLoading(true);
    setStatus('idle');
    try {
      const res = await fetch('/api/experiments', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action }),
      });
      if (res.ok) {
        setStatus('success');
        setTimeout(() => setStatus('idle'), 2000);
      } else {
        setStatus('error');
        setTimeout(() => setStatus('idle'), 3000);
      }
    } catch {
      setStatus('error');
      setTimeout(() => setStatus('idle'), 3000);
    } finally {
      setIsLoading(false);
    }
  };

  const baseClasses =
    'px-4 py-2 rounded-lg font-medium transition-all flex items-center justify-center gap-2 text-sm shadow-sm';
  const variants = {
    danger:
      'bg-red-500/10 text-red-400 border border-red-500/30 hover:bg-red-500/20 focus:ring-2 focus:ring-red-500/50 outline-none',
    warning:
      'bg-yellow-500/10 text-yellow-400 border border-yellow-500/30 hover:bg-yellow-500/20 focus:ring-2 focus:ring-yellow-500/50 outline-none',
    default:
      'bg-zinc-800 text-zinc-200 border border-zinc-700 hover:bg-zinc-700 focus:ring-2 focus:ring-zinc-500/50 outline-none',
  };

  const isDisabled = disabled || isLoading;
  const effectiveTooltip = disabled
    ? tooltip || 'Run locally to inject real chaos'
    : undefined;

  return (
    <div className="relative group inline-block">
      <button
        onClick={triggerExperiment}
        disabled={isDisabled}
        title={effectiveTooltip}
        className={`${baseClasses} ${variants[variant]} ${
          disabled
            ? 'opacity-40 cursor-not-allowed hover:bg-transparent'
            : isLoading
            ? 'opacity-50 cursor-not-allowed'
            : ''
        }`}
      >
        {isLoading ? (
          <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
              fill="none"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
        ) : null}
        {status === 'success' && !isLoading
          ? 'Triggered!'
          : status === 'error' && !isLoading
          ? 'Failed (Offline)'
          : label}
      </button>

      {disabled && (
        <div className="pointer-events-none absolute bottom-full left-1/2 -translate-x-1/2 mb-2 hidden group-hover:flex items-center gap-1.5 w-max max-w-xs px-2.5 py-1 text-xs text-zinc-200 bg-zinc-900 rounded-md shadow-xl border border-zinc-700 z-50 animate-in fade-in zoom-in-95 duration-150">
          <svg
            className="w-3.5 h-3.5 text-amber-400 shrink-0"
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
          <span>{effectiveTooltip}</span>
        </div>
      )}
    </div>
  );
}
