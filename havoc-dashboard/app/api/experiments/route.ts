import { NextResponse } from 'next/server';

export async function POST(request: Request) {
  try {
    const body = await request.json();
    const { action } = body;
    console.log(`[API] Experiment triggered: ${action}`);
    
    // Simulate some network delay
    await new Promise((resolve) => setTimeout(resolve, 500));
    
    return NextResponse.json({ success: true, message: `Experiment ${action} started` });
  } catch {
    return NextResponse.json({ success: false, message: 'Invalid request' }, { status: 400 });
  }
}
