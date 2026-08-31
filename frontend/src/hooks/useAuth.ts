import { useEffect } from 'react';
import { useAuthStore } from '../stores/authStore';
import { isTokenExpired } from '../utils/jwt';
import { fetchMe } from '../api/auth';

/**
 * Auth convenience hook.
 *
 * On mount (and whenever the token changes) it:
 *   1. Proactively checks the JWT `exp` claim and logs the user out if the
 *      token has already expired, so the user lands on /login instead of
 *      triggering a 401 on the next API call.
 *   2. Calls GET /auth/me to refresh server-side state. Today this is used
 *      to pick up `must_change_password` flips that happened outside the
 *      current tab (e.g. an admin forced a password change in another tab,
 *      or the backend flipped the flag). Without this call the store could
 *      keep `mustChangePassword=false` and the user would never be routed
 *      to the change-password screen until a full reload (R3-F4).
 *
 * NOTE: this does NOT make sessionStorage-stored tokens XSS-safe. The
 * durable fix is httpOnly + SameSite=Strict cookies issued by the backend,
 * which requires backend API changes tracked separately.
 */
export function useAuth() {
  const store = useAuthStore();

  useEffect(() => {
    // Bail out early when there is no token — fetchMe would just 401.
    if (!store.token) {
      return;
    }
    if (isTokenExpired(store.token)) {
      store.logout();
      return;
    }
    // Refresh server-side auth flags. A failure (e.g. 401 from a since-revoked
    // token, or a transient network error) is intentionally swallowed: the JWT
    // expiry check above is the authoritative logout trigger, and we don't want
    // a flaky /auth/me to bounce a still-valid session.
    //
    // We read mustChangePassword/setMustChangePassword from the live store via
    // getState() inside the async callback (instead of closing over `store`)
    // so that (a) we don't capture a stale value and (b) the effect deps don't
    // need to include mustChangePassword — which would re-trigger fetchMe on
    // every flag flip and cause a redundant request right after we just set it.
    let cancelled = false;
    fetchMe()
      .then((me: { must_change_password?: boolean }) => {
        if (cancelled) return;
        if (me && me.must_change_password) {
          const live = useAuthStore.getState();
          if (!live.mustChangePassword) {
            live.setMustChangePassword(true);
          }
        }
      })
      .catch(() => {
        /* see comment above */
      });
    return () => {
      cancelled = true;
    };
  }, [store.token, store.logout]);

  return {
    isAuthenticated: store.isAuthenticated(),
    username: store.username,
    mustChangePassword: store.mustChangePassword,
    login: store.login,
    logout: store.logout,
  };
}
