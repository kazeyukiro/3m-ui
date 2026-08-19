import client from './client';
import { useAuthStore } from '../stores/authStore';

export interface LoginInput {
  username: string;
  password: string;
}

export async function login(input: LoginInput) {
  const { data } = await client.post<{
    token: string;
    username: string;
    must_change_password: boolean;
  }>('/auth/login', input);
  useAuthStore.getState().login(data.token, data.username, data.must_change_password);
  return data;
}

export async function changePassword(current: string, next: string) {
  const { data } = await client.post('/auth/password', {
    current_password: current,
    new_password: next,
  });
  useAuthStore.getState().setMustChangePassword(false);
  return data;
}

export async function fetchMe() {
  const { data } = await client.get('/auth/me');
  return data;
}
