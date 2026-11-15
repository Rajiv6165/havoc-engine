import { NextResponse } from 'next/server';

export async function GET(request: Request) {
  const engineUrl = process.env.HAVOC_API_URL || process.env.NEXT_PUBLIC_ENGINE_API || 'http://localhost:8080';
  const { searchParams } = new URL(request.url);
  const a = searchParams.get('a');
  const b = searchParams.get('b');
  const action = searchParams.get('action');

  if (!a || !b) {
    return NextResponse.json(
      { error: "both 'a' and 'b' query parameters are required" },
      { status: 400 }
    );
  }

  let query = `a=${encodeURIComponent(a)}&b=${encodeURIComponent(b)}`;
  if (action) {
    query += `&action=${encodeURIComponent(action)}`;
  }

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 4000);
    const headers: Record<string, string> = {};
    if (process.env.HAVOC_API_TOKEN) {
      headers['Authorization'] = `Bearer ${process.env.HAVOC_API_TOKEN}`;
    }

    const res = await fetch(`${engineUrl}/api/compare?${query}`, {
      headers,
      signal: controller.signal,
      cache: 'no-store',
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
