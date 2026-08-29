'use client';
import React, { useEffect, useMemo, useCallback } from 'react';
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
import { initialNodes, initialEdges, TopologyNodeData } from '@/lib/topology';
import ServiceNode from './ServiceNode';

export default function BlastRadiusGraph() {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node<TopologyNodeData>>(initialNodes as Node<TopologyNodeData>[]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>(initialEdges as Edge[]);

  const nodeTypes = useMemo(() => ({ serviceNode: ServiceNode }), []);

  const updateNodeStatus = useCallback((serviceName: string, status: TopologyNodeData['status']) => {
    setNodes((nds) =>
      nds.map((n) => {
        if (n.data.label === serviceName) {
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
          // Add custom animated styling for spreading errors
          return { 
            ...e, 
            animated, 
            style: animated ? { stroke: '#f97316', strokeWidth: 2 } : { stroke: '#52525b', strokeWidth: 1 } 
          };
        }
        return e;
      })
    );
  }, [setEdges]);

  useEffect(() => {
    if (!wsClient) return;
    
    // We don't call wsClient.connect() here, it's called in LiveLogPanel. 
    // They share the same client.
    
    const unsubscribeMessage = wsClient.onMessage((data) => {
      const msg = typeof data.message === 'string' ? data.message.toLowerCase() : JSON.stringify(data).toLowerCase();
      const level = data.level || 'info';

      // Find which services are mentioned
      const mentionedServices = initialNodes
        .map(n => n.data.label)
        .filter(label => msg.includes(label.toLowerCase()));

      mentionedServices.forEach(service => {
        if (level === 'warn') {
          updateNodeStatus(service, 'targeted');
          // Node is targeted directly, edges aren't necessarily spreading failure yet
        } else if (level === 'error') {
          updateNodeStatus(service, 'downstream-error');
          // The error reached this node, animate edges leading TO it to show spread
          const nodeId = initialNodes.find(n => n.data.label === service)?.id;
          if (nodeId) {
            setAnimatedEdgesToNode(nodeId, true);
          }
        }
      });
      
      // Basic reset logic if we see a specific reset message (optional)
      if (msg.includes('experiment ended') || msg.includes('resolved')) {
        setNodes((nds) => nds.map(n => ({ ...n, data: { ...n.data, status: 'healthy' } })));
        setEdges((eds) => eds.map(e => ({ ...e, animated: false, style: { stroke: '#52525b', strokeWidth: 1 } })));
      }
    });

    return () => {
      unsubscribeMessage();
    };
  }, [updateNodeStatus, setAnimatedEdgesToNode, setNodes, setEdges]);

  return (
    <div className="h-[500px] w-full bg-zinc-950/50 rounded-xl border border-zinc-800 overflow-hidden relative">
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
  );
}
