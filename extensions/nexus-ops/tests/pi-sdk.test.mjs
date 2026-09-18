import assert from 'node:assert/strict';
import test from 'node:test';
import { createAgentSession, SessionManager } from '@earendil-works/pi-coding-agent';

test('Pi SDK session exports are available without model credentials', () => {
  assert.equal(typeof createAgentSession, 'function');
  assert.equal(typeof SessionManager.inMemory, 'function');
});
