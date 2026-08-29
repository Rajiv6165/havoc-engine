import React from 'react';
import { Handle, Position, NodeProps, Node } from '@xyflow/react';
import { TopologyNodeData } from '@/lib/topology';

export default function ServiceNode({ data }: NodeProps<Node<TopologyNodeData, 'serviceNode'>>) {
  const { label, podCount, status } = data;

  let ringClass = 'border-zinc-700 hover:border-zinc-500';
  let bgClass = 'bg-zinc-900/80';
  let glowClass = '';

  if (status === 'targeted') {
    ringClass = 'border-red-500';
    bgClass = 'bg-zinc-900';
    glowClass = 'animate-pulse shadow-[0_0_15px_rgba(239,68,68,0.6)]';
  } else if (status === 'downstream-error') {
    ringClass = 'border-orange-500';
    bgClass = 'bg-zinc-900';
    glowClass = 'animate-pulse shadow-[0_0_15px_rgba(249,115,22,0.6)]';
  }

  return (
    <div className={`px-4 py-3 shadow-md rounded-lg border-2 transition-all duration-300 backdrop-blur-md ${ringClass} ${bgClass} ${glowClass}`}>
      <Handle type="target" position={Position.Top} className="w-2 h-2 !bg-zinc-500" />
      <div className="flex flex-col items-center">
        <div className="font-semibold text-zinc-100 text-sm tracking-tight">{label}</div>
        <div className="text-zinc-400 text-xs mt-1 font-medium flex items-center gap-1">
          <div className={`w-1.5 h-1.5 rounded-full ${status === 'healthy' ? 'bg-green-500' : status === 'targeted' ? 'bg-red-500' : 'bg-orange-500'}`} />
          {podCount} {podCount === 1 ? 'Pod' : 'Pods'}
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} className="w-2 h-2 !bg-zinc-500" />
    </div>
  );
}
