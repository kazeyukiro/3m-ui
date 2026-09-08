import client from './client';
import { useAuthStore } from '../stores/authStore';

export interface LoginInput {
  username: string;
  password: string;
}

export interface LoginResult {
  token: string;
  username: string;
  must_change_password: boolean;
  expires_at?: string;
  role?: string;
}

export async function login(input: LoginInput): Promise<LoginResult> {
  const { data } = await client.post<LoginResult>('/auth/login', input);
  if (!data?.token) {
    throw new Error('Login response missing token');
  }
  useAuthStore.getState().login(data.token, data.username || input.username, !!data.must_change_password);
  return data;
}

export async function changePassword(current: string, next: string) {
  const { data } = await client.post<{
    status?: string;
    message?: string;
    token?: string;
    username?: string;
    must_change_password?: boolean;
    relogin?: boolean;
  }>('/auth/password', {
    current_password: current,
    new_password: next,
  });
  if (data?.token) {
    useAuthStore.getState().login(data.token, data.username || useAuthStore.getState().username || '', false);
  } else {
    // Older backends invalidate the session without returning a token.
    useAuthStore.getState().logout();
    throw new Error(data?.message || 'Password changed; please log in again with the new password');
  }
  return data;
}

export async function fetchMe() {
  const { data } = await client.get('/auth/me');
  return data;
}
