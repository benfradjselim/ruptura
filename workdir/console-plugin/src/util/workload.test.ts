import { failuresOf, parseWorkload, shortPod, visibleFailures, workloadKey } from './workload';
import { Failure } from '../types';

const failure = (workload: string): Failure => ({
  level: 'workload',
  target: workload,
  workload,
  failureMode: 'crash_loop',
  kind: 'detected',
  state: 'critical',
});

describe('parseWorkload', () => {
  it.each([
    ['demo-shop/Deployment/leak-orders', { namespace: 'demo-shop', kind: 'Deployment', name: 'leak-orders' }],
    ['demo-shop/leak-orders', null],
    ['a/b/c/d', null],
    ['/Deployment/x', null],
  ])('%s', (key, want) => {
    expect(parseWorkload(key)).toEqual(want);
  });
});

describe('workloadKey', () => {
  it('matches the engine key of a console object', () => {
    expect(workloadKey({ kind: 'StatefulSet', metadata: { name: 'db', namespace: 'shop' } })).toBe('shop/StatefulSet/db');
  });

  it('tolerates an object without metadata', () => {
    expect(workloadKey({ kind: 'Deployment' })).toBe('/Deployment/');
  });
});

describe('failuresOf', () => {
  it('keeps the failures of one workload only', () => {
    const got = failuresOf([failure('a/Deployment/x'), failure('a/Deployment/y'), failure('a/Deployment/x')], 'a/Deployment/x');
    expect(got).toHaveLength(2);
  });
});

describe('visibleFailures', () => {
  const all = [failure('shop/Deployment/a'), failure('secret/Deployment/b'), failure('shop/StatefulSet/c')];

  it('keeps the namespaces the user may see', () => {
    expect(visibleFailures(all, ['shop']).map((f) => f.workload)).toEqual(['shop/Deployment/a', 'shop/StatefulSet/c']);
  });

  it('shows nothing while the namespaces are unknown', () => {
    expect(visibleFailures(all, null)).toEqual([]);
  });

  it('shows nothing to a user without namespaces', () => {
    expect(visibleFailures(all, [])).toEqual([]);
  });

  it('drops a failure with a malformed key', () => {
    expect(visibleFailures([failure('shop')], ['shop'])).toEqual([]);
  });
});

describe('shortPod', () => {
  it('drops the namespace', () => {
    expect(shortPod('shop/api-1')).toBe('api-1');
    expect(shortPod('api-1')).toBe('api-1');
  });
});
