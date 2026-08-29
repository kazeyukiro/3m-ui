/**
 * JWT helpers.
 *
 * NOTE on security: the auth token is currently stored in `sessionStorage`
 * (see stores/authStore.ts) which makes it stealable via XSS. The proper
 * fix is to migrate to an httpOnly + SameSite=Strict cookie set by the
 * backend, but that requires backend API changes (the backend currently
 * expects `Authorization: Bearer <jwt>` and does not yet issue / accept
 * cookies). That migration is tracked separately and out of scope for this
 * agent.
 *
 * What this module does provide is *expiry awareness*: rather than waiting
 * for a 401 from the backend (which means the user already sees a broken
 * request), we proactively detect an expired JWT on app boot / route
 * change and clear the session.
 */

/**
 * Returns true if the supplied JWT is missing, malformed, or past its
 * `exp` claim. Returns false if the token is present, well-formed, and
 * either has no `exp` claim or has not yet expired.
 *
 * Defensive: any decode / parse error is treated as "expired" so the
 * caller will log the user out rather than sending a bad token.
 */
export function isTokenExpired(token: string | null | undefined): boolean {
  if (!token) return true;
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return true;
    // JWT uses base64url without padding; restore padding for atob.
    const b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4);
    const payload = JSON.parse(atob(padded));
    if (!payload.exp) return false;
    // exp is seconds since epoch; Date.now() is milliseconds.
    return Date.now() >= payload.exp * 1000;
  } catch {
    return true;
  }
}
