'use client';
import React, { useState } from 'react';

interface ExperimentButtonProps {
  label: string;
  action: string;
  variant?: 'danger' | 'warning' | 'default';
}

export default function ExperimentButton({ label, action, variant = 'default' }: ExperimentButtonProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [status, setStatus] = useState<'idle' | 'success' | 'error'>('idle');

  const triggerExperiment = async () => {
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
      }
    } catch {
      setStatus('error');
    } finally {
      setIsLoading(false);
    }
  };

  const baseClasses = "px-4 py-2 rounded-lg font-medium transition-all flex items-center justify-center gap-2 text-sm shadow-sm";
  const variants = {
    danger: "bg-red-500/10 text-red-400 border border-red-500/30 hover:bg-red-500/20 focus:ring-2 focus:ring-red-500/50 outline-none",
    warning: "bg-yellow-500/10 text-yellow-400 border border-yellow-500/30 hover:bg-yellow-500/20 focus:ring-2 focus:ring-yellow-500/50 outline-none",
    default: "bg-zinc-800 text-zinc-200 border border-zinc-700 hover:bg-zinc-700 focus:ring-2 focus:ring-zinc-500/50 outline-none",
  };

  return (
    <button
      onClick={triggerExperiment}
      disabled={isLoading}
      className={`${baseClasses} ${variants[variant]} ${isLoading ? 'opacity-50 cursor-not-allowed' : ''}`}
    >
      {isLoading ? (
        <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      ) : null}
      {status === 'success' && !isLoading ? 'Triggered!' : label}
    </button>
  );
}
