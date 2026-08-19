import { useAuthStore } from '../stores/authStore';

export function useAuth() {
  const store = useAuthStore();
  return {
    isAuthenticated: store.isAuthenticated(),
    username: store.username,
    mustChangePassword: store.mustChangePassword,
    login: store.login,
    logout: store.logout,
  };
}
