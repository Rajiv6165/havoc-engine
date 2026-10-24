import { NextResponse } from 'next/server';

export async function GET(request: Request) {
  const engineUrl = process.env.HAVOC_API_URL || process.env.NEXT_PUBLIC_ENGINE_API || 'http://localhost:8080';
  const { searchParams } = new URL(request.url);
  const limit = searchParams.get('limit') || '20';

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 4000);

    const res = await fetch(`${engineUrl}/api/experiments?limit=${limit}`, {
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

export async function POST(request: Request) {
  const engineUrl = process.env.HAVOC_API_URL || process.env.NEXT_PUBLIC_ENGINE_API || 'http://localhost:8080';

  try {
    const body = await request.json();

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 4000);

    const res = await fetch(`${engineUrl}/api/experiments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
      signal: controller.signal,
    });
    clearTimeout(timeoutId);

    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch (err: unknown) {
    const errorMsg = err instanceof Error ? err.message : 'Engine unreachable';
    return NextResponse.json(
      { error: errorMsg, offline: true },
      { status: 503 }
    );
  }
}
