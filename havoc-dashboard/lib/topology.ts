export type ServiceStatus = 'healthy' | 'targeted' | 'downstream-error';

export interface TopologyNodeData extends Record<string, unknown> {
  label: string;
  podCount: number;
  status: ServiceStatus;
}

export const initialNodes = [
  {
    id: 'api-gateway',
    type: 'serviceNode',
    position: { x: 300, y: 50 },
    data: { label: 'api-gateway', podCount: 3, status: 'healthy' as ServiceStatus },
  },
  {
    id: 'payment-service',
    type: 'serviceNode',
    position: { x: 100, y: 200 },
    data: { label: 'payment-service', podCount: 2, status: 'healthy' as ServiceStatus },
  },
  {
    id: 'user-service',
    type: 'serviceNode',
    position: { x: 500, y: 200 },
    data: { label: 'user-service', podCount: 4, status: 'healthy' as ServiceStatus },
  },
  {
    id: 'database-service',
    type: 'serviceNode',
    position: { x: 100, y: 350 },
    data: { label: 'database-service', podCount: 1, status: 'healthy' as ServiceStatus },
  },
  {
    id: 'queue-service',
    type: 'serviceNode',
    position: { x: 300, y: 350 },
    data: { label: 'queue-service', podCount: 2, status: 'healthy' as ServiceStatus },
  }
];

export const initialEdges = [
  { id: 'e1', source: 'api-gateway', target: 'payment-service', animated: false },
  { id: 'e2', source: 'api-gateway', target: 'user-service', animated: false },
  { id: 'e3', source: 'payment-service', target: 'database-service', animated: false },
  { id: 'e4', source: 'payment-service', target: 'queue-service', animated: false },
  { id: 'e5', source: 'user-service', target: 'queue-service', animated: false },
];
