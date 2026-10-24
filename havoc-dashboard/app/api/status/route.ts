import { NextResponse } from 'next/server';

export async function GET() {
  const engineUrl = process.env.HAVOC_API_URL || process.env.NEXT_PUBLIC_ENGINE_API || 'http://localhost:8080';

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 4000);

    const res = await fetch(`${engineUrl}/api/status`, {
      signal: controller.signal,
      cache: 'no-store',
    });
    clearTimeout(timeoutId);

    if (!res.ok) {
      return NextResponse.json(
        { error: `Engine returned status ${res.status}`, offline: true },
        { status: res.status }
      );
    }

    const data = await res.json();
    return NextResponse.json(data);
  } catch (err: unknown) {
    const errorMsg = err instanceof Error ? err.message : 'Engine unreachable';
    return NextResponse.json(
      { error: errorMsg, offline: true },
      { status: 503 }
    );
  }
}
