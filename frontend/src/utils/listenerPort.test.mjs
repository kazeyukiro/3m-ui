import assert from 'node:assert/strict';
import { collectUsedPorts, randomListenerPort, LISTENER_PORT_MIN, LISTENER_PORT_MAX } from './listenerPort.ts';

// inverted range normalized
{
  const used = collectUsedPorts(['50005-50000']);
  assert.equal(used.has(50000), true);
  assert.equal(used.has(50005), true);
  assert.equal(used.has(50003), true);
}

// list + single
{
  const used = collectUsedPorts(['20000', '30000-30002', ' 40000,40001 ']);
  assert.equal(used.has(20000), true);
  assert.equal(used.has(30001), true);
  assert.equal(used.has(40001), true);
  assert.equal(used.has(19999), false);
}

// outside suggestion range ignored for used set intersection
{
  const used = collectUsedPorts(['443', '80-90']);
  assert.equal(used.size, 0);
}

// exhausted range
{
  const all = [`${LISTENER_PORT_MIN}-${LISTENER_PORT_MAX}`];
  assert.equal(randomListenerPort(all), undefined);
}

// avoids used ports
{
  for (let i = 0; i < 20; i++) {
    const p = randomListenerPort(['20000', '20001']);
    assert.ok(p);
    const n = Number(p);
    assert.ok(n >= LISTENER_PORT_MIN && n <= LISTENER_PORT_MAX);
    assert.notEqual(n, 20000);
    assert.notEqual(n, 20001);
  }
}

// tiny range exact
{
  const p = randomListenerPort(['10000'], 10000, 10001);
  assert.equal(p, '10001');
}

console.log('listenerPort tests passed');
