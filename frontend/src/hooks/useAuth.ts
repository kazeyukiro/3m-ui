import { useEffect } from 'react';
import { useAuthStore } from '../stores/authStore';
import { isTokenExpired } from '../utils/jwt';

/**
 * Auth convenience hook.
 *
 * On mount (and whenever the token changes) it proactively checks the JWT
 * `exp` claim and logs the user out if the token has already expired, so
 * the user lands on /login instead of triggering a 401 on the next API
 * call.
 *
 * NOTE: this does NOT make sessionStorage-stored tokens XSS-safe. The
 * durable fix is httpOnly + SameSite=Strict cookies issued by the backend,
 * which requires backend API changes tracked separately.
 */
export function useAuth() {
  const store = useAuthStore();

  useEffect(() => {
    if (store.token && isTokenExpired(store.token)) {
      store.logout();
    }
  }, [store.token, store.logout]);

  return {
    isAuthenticated: store.isAuthenticated(),
    username: store.username,
    mustChangePassword: store.mustChangePassword,
    login: store.login,
    logout: store.logout,
  };
}
