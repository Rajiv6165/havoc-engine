'use client';
import React, { useEffect, useMemo, useCallback, useState } from 'react';
import {
  ReactFlow,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  Node,
  Edge,
  BackgroundVariant,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { wsClient } from '@/lib/websocket';
import { fallbackNodes, fallbackEdges, fetchTopology, TopologyNodeData } from '@/lib/topology';
import ServiceNode from './ServiceNode';

export default function BlastRadiusGraph() {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node<TopologyNodeData>>(fallbackNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>(fallbackEdges);
  const [isFallback, setIsFallback] = useState(true);
  const [isLoading, setIsLoading] = useState(true);

  const nodeTypes = useMemo(() => ({ serviceNode: ServiceNode }), []);

  const loadTopology = useCallback(async () => {
    setIsLoading(true);
    const topo = await fetchTopology();
    setNodes(topo.nodes);
    setEdges(topo.edges);
    setIsFallback(topo.fallback);
    setIsLoading(false);
  }, [setNodes, setEdges]);

  useEffect(() => {
    loadTopology();
  }, [loadTopology]);

  const updateNodeStatus = useCallback((serviceName: string, status: TopologyNodeData['status']) => {
    setNodes((nds) =>
      nds.map((n) => {
        const label = String(n.data.label || n.id).toLowerCase();
        const target = serviceName.toLowerCase();
        if (label === target || label.includes(target) || target.includes(label)) {
          return { ...n, data: { ...n.data, status } };
        }
        return n;
      })
    );
  }, [setNodes]);

  const setAnimatedEdgesToNode = useCallback((targetNodeId: string, animated: boolean) => {
    setEdges((eds) =>
      eds.map((e) => {
        if (e.target === targetNodeId) {
          return {
            ...e,
            animated,
            style: animated ? { stroke: '#f97316', strokeWidth: 2 } : { stroke: '#52525b', strokeWidth: 1 },
          };
        }
        return e;
      })
    );
  }, [setEdges]);

  useEffect(() => {
    if (!wsClient) return;

    const unsubscribeMessage = wsClient.onMessage((data) => {
      const msg = typeof data.message === 'string' ? data.message.toLowerCase() : JSON.stringify(data).toLowerCase();
      const level = data.level || 'info';
      const eventType = data.type || '';
      const serviceProp = (data.service || '').toLowerCase();

      // Check for reset / resolved events
      if (
        eventType === 'completed' ||
        eventType === 'recovered' ||
        msg.includes('experiment ended') ||
        msg.includes('resolved')
      ) {
        setNodes((nds) => nds.map((n) => ({ ...n, data: { ...n.data, status: 'healthy' } })));
        setEdges((eds) => eds.map((e) => ({ ...e, animated: false, style: { stroke: '#52525b', strokeWidth: 1 } })));
        return;
      }

      // Identify targeted services
      setNodes((currentNodes) => {
        const matched = currentNodes.filter((n) => {
          const label = String(n.data.label || n.id).toLowerCase();
          return (
            (serviceProp && (serviceProp === label || label.includes(serviceProp) || serviceProp.includes(label))) ||
            msg.includes(label)
          );
        });

        matched.forEach((matchedNode) => {
          if (
            eventType === 'target_selected' ||
            eventType === 'pod_killed' ||
            level === 'warn'
          ) {
            updateNodeStatus(matchedNode.data.label, 'targeted');
          } else if (
            eventType === 'aborted' ||
            eventType === 'blocked' ||
            level === 'error'
          ) {
            updateNodeStatus(matchedNode.data.label, 'downstream-error');
            setAnimatedEdgesToNode(matchedNode.id, true);
          }
        });

        return currentNodes;
      });
    });

    return () => {
      unsubscribeMessage();
    };
  }, [updateNodeStatus, setAnimatedEdgesToNode, setNodes, setEdges]);

  return (
    <div className="h-[500px] w-full bg-zinc-950/50 rounded-xl border border-zinc-800 overflow-hidden relative flex flex-col">
      <div className="absolute top-3 right-3 z-10 flex items-center gap-2">
        {isLoading ? (
          <span className="text-[11px] px-2.5 py-1 rounded-full bg-zinc-900 border border-zinc-800 text-zinc-400">
            Loading Topology...
          </span>
        ) : isFallback ? (
          <span className="text-[11px] px-2.5 py-1 rounded-full bg-amber-500/10 border border-amber-500/30 text-amber-300 font-medium flex items-center gap-1.5 shadow-sm">
            <span className="w-1.5 h-1.5 rounded-full bg-amber-400" />
            Fallback Mock Topology (Engine Offline)
          </span>
        ) : (
          <span className="text-[11px] px-2.5 py-1 rounded-full bg-green-500/10 border border-green-500/30 text-green-300 font-medium flex items-center gap-1.5 shadow-sm">
            <span className="w-1.5 h-1.5 rounded-full bg-green-400 shadow-[0_0_6px_rgba(74,222,128,0.8)]" />
            Live Kubernetes Topology
          </span>
        )}
      </div>

      <div className="flex-1 w-full h-full">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          fitView
          className="bg-zinc-950"
        >
          <Background variant={BackgroundVariant.Dots} gap={16} size={1} color="#3f3f46" />
          <Controls className="!bg-zinc-900 !border-zinc-800 !text-zinc-300 [&>button]:!border-zinc-800 hover:[&>button]:!bg-zinc-800" />
        </ReactFlow>
      </div>
    </div>
  );
}
