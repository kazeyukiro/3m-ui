import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

interface AuthState {
  token: string | null;
  username: string | null;
  mustChangePassword: boolean;
  isAuthenticated: () => boolean;
  login: (token: string, username: string, mustChange: boolean) => void;
  logout: () => void;
  setMustChangePassword: (v: boolean) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      username: null,
      mustChangePassword: false,

      isAuthenticated: () => !!get().token,

      login: (token, username, mustChange) =>
        set({ token, username, mustChangePassword: mustChange }),

      logout: () =>
        set({ token: null, username: null, mustChangePassword: false }),

      setMustChangePassword: (v) => set({ mustChangePassword: v }),
    }),
    {
      name: '3m-ui-auth',
      storage: createJSONStorage(() => sessionStorage),
    }
  )
);
