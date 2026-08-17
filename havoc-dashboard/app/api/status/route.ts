import { NextResponse } from 'next/server';

export async function GET() {
  const mockStatus = [
    { name: 'payment-service', podCount: 5, status: 'healthy' },
    { name: 'auth-service', podCount: 3, status: 'degraded' },
    { name: 'checkout-service', podCount: 0, status: 'critical' },
    { name: 'inventory-service', podCount: 8, status: 'healthy' },
  ];
  return NextResponse.json(mockStatus);
}
