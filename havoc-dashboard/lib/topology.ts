import { Node, Edge } from '@xyflow/react';

export type ServiceStatus = 'healthy' | 'targeted' | 'downstream-error';

export interface TopologyNodeData extends Record<string, unknown> {
  label: string;
  podCount: number;
  status: ServiceStatus;
}

export const fallbackNodes: Node<TopologyNodeData>[] = [
  {
    id: 'api-gateway',
    type: 'serviceNode',
    position: { x: 300, y: 50 },
    data: { label: 'api-gateway', podCount: 3, status: 'healthy' },
  },
  {
    id: 'payment-service',
    type: 'serviceNode',
    position: { x: 100, y: 200 },
    data: { label: 'payment-service', podCount: 2, status: 'healthy' },
  },
  {
    id: 'user-service',
    type: 'serviceNode',
    position: { x: 500, y: 200 },
    data: { label: 'user-service', podCount: 4, status: 'healthy' },
  },
  {
    id: 'database-service',
    type: 'serviceNode',
    position: { x: 100, y: 350 },
    data: { label: 'database-service', podCount: 1, status: 'healthy' },
  },
  {
    id: 'queue-service',
    type: 'serviceNode',
    position: { x: 300, y: 350 },
    data: { label: 'queue-service', podCount: 2, status: 'healthy' },
  },
];

export const fallbackEdges: Edge[] = [
  { id: 'e1', source: 'api-gateway', target: 'payment-service', animated: false },
  { id: 'e2', source: 'api-gateway', target: 'user-service', animated: false },
  { id: 'e3', source: 'payment-service', target: 'database-service', animated: false },
  { id: 'e4', source: 'payment-service', target: 'queue-service', animated: false },
  { id: 'e5', source: 'user-service', target: 'queue-service', animated: false },
];

export const initialNodes = fallbackNodes;
export const initialEdges = fallbackEdges;

export interface TopologyResponse {
  fallback: boolean;
  offline?: boolean;
  nodes: Node<TopologyNodeData>[];
  edges: Edge[];
}

export async function fetchTopology(): Promise<TopologyResponse> {
  try {
    const res = await fetch('/api/topology', { cache: 'no-store' });
    if (!res.ok) {
      return { fallback: true, nodes: fallbackNodes, edges: fallbackEdges };
    }
    const data = await res.json();
    return {
      fallback: Boolean(data.fallback),
      offline: Boolean(data.offline),
      nodes: data.nodes && data.nodes.length > 0 ? data.nodes : fallbackNodes,
      edges: data.edges || fallbackEdges,
    };
  } catch {
    return { fallback: true, offline: true, nodes: fallbackNodes, edges: fallbackEdges };
  }
}
