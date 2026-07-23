export function assertLocalMutationTarget(baseURL, allowMutation) {
  if (!allowMutation) {
    throw new Error('this script changes local business data; pass --allow-mutation to continue');
  }
  const target = new URL(baseURL);
  if (target.username || target.password) {
    throw new Error('mutation target URL must not contain credentials');
  }
  const loopbackHosts = new Set(['127.0.0.1', 'localhost', '[::1]']);
  if (!loopbackHosts.has(target.hostname)) {
    throw new Error(`mutation target must be a loopback address, got ${target.hostname}`);
  }
  if (target.protocol !== 'http:') {
    throw new Error(`local mutation target must use http, got ${target.protocol}`);
  }
  return target;
}
