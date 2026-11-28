import assert from 'node:assert/strict';
import test from 'node:test';
import { cleanFormValue } from './formValues.ts';

test('clearing DNS fields omits optional values while preserving explicit false, policies and rule order', () => {
  const input = {
    enable: false, listen: '', ipv6: false, useHosts: false, fakeIPTTL: 0,
    nameserver: ['https://dns.example/dns-query#DIRECT&ecs=1.1.1.0/24', ''],
    fallback: [], fallbackFilter: { geoip: false, geoipCode: '' },
    nameserverPolicy: { '+.lan': 'system', 'rule-set:cn': ['223.5.5.5'], '': [] },
    proxyServerNameserverPolicy: {},
    fakeIPFilter: ['DOMAIN,example.com,real-ip', 'MATCH,fake-ip'],
  };
  assert.deepEqual(cleanFormValue(input), {
    enable: false, ipv6: false, useHosts: false, fakeIPTTL: 0,
    nameserver: ['https://dns.example/dns-query#DIRECT&ecs=1.1.1.0/24'],
    fallbackFilter: { geoip: false },
    nameserverPolicy: { '+.lan': 'system', 'rule-set:cn': ['223.5.5.5'] },
    fakeIPFilter: ['DOMAIN,example.com,real-ip', 'MATCH,fake-ip'],
  });
  assert.equal(input.listen, '');
});
