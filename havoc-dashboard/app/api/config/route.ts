import { NextResponse } from 'next/server';

export async function GET() {
  const readOnly =
    process.env.DEMO_READ_ONLY === 'true' ||
    process.env.NEXT_PUBLIC_DEMO_READ_ONLY === 'true';

  return NextResponse.json({
    readOnly,
  });
}

