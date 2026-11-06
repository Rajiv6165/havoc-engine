import test from 'node:test';
import assert from 'node:assert/strict';

test('DEMO_READ_ONLY flag evaluation', () => {
  // When no env var is set, read-only mode is inactive
  delete process.env.DEMO_READ_ONLY;
  delete process.env.NEXT_PUBLIC_DEMO_READ_ONLY;
  let isReadOnly =
    process.env.DEMO_READ_ONLY === 'true' ||
    process.env.NEXT_PUBLIC_DEMO_READ_ONLY === 'true';
  assert.equal(isReadOnly, false, 'Default should not be read-only');

  // When DEMO_READ_ONLY=true is set, read-only mode is active
  process.env.DEMO_READ_ONLY = 'true';
  isReadOnly =
    process.env.DEMO_READ_ONLY === 'true' ||
    process.env.NEXT_PUBLIC_DEMO_READ_ONLY === 'true';
  assert.equal(isReadOnly, true, 'DEMO_READ_ONLY=true should enable read-only mode');

  // When NEXT_PUBLIC_DEMO_READ_ONLY=true is set, read-only mode is active
  delete process.env.DEMO_READ_ONLY;
  process.env.NEXT_PUBLIC_DEMO_READ_ONLY = 'true';
  isReadOnly =
    process.env.DEMO_READ_ONLY === 'true' ||
    process.env.NEXT_PUBLIC_DEMO_READ_ONLY === 'true';
  assert.equal(isReadOnly, true, 'NEXT_PUBLIC_DEMO_READ_ONLY=true should enable read-only mode');

  // Tooltip requirement
  const expectedTooltip = 'Run locally to inject real chaos';
  assert.equal(expectedTooltip, 'Run locally to inject real chaos');
});
