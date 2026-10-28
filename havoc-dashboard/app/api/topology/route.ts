import { NextResponse } from 'next/server';
import { fallbackNodes, fallbackEdges } from '@/lib/topology';

export async function GET() {
  const engineUrl = process.env.HAVOC_API_URL || process.env.NEXT_PUBLIC_ENGINE_API || 'http://localhost:8080';

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 4000);

    const headers: Record<string, string> = {};
    if (process.env.HAVOC_API_TOKEN) {
      headers['Authorization'] = `Bearer ${process.env.HAVOC_API_TOKEN}`;
    }

    const res = await fetch(`${engineUrl}/api/topology`, {
      headers,
      signal: controller.signal,
      cache: 'no-store',
    });
    clearTimeout(timeoutId);

    if (!res.ok) {
      return NextResponse.json({
        fallback: true,
        offline: true,
        nodes: fallbackNodes,
        edges: fallbackEdges,
      });
    }

    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    // Engine offline fallback
    return NextResponse.json({
      fallback: true,
      offline: true,
      nodes: fallbackNodes,
      edges: fallbackEdges,
    });
  }
}
