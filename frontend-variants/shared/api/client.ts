import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from '../stores/authStore';

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
  headers: { Accept: 'application/json' },
});

// Request interceptor: inject JWT
client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = useAuthStore.getState().token;
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: handle auth errors globally
client.interceptors.response.use(
  (res) => res,
  (err: AxiosError<{ error?: string; code?: string }>) => {
    if (!err.response) {
      return Promise.reject(new Error('Backend service unreachable'));
    }
    const { status, data } = err.response;

    if (status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
      return Promise.reject(new Error('Session expired'));
    }
    if (status === 403 && data?.code === 'PASSWORD_CHANGE_REQUIRED') {
      if (window.location.pathname !== '/change-password') {
        useAuthStore.getState().setMustChangePassword(true);
        window.location.href = '/change-password';
      }
      return Promise.reject(new Error('Password change required'));
    }
    if (status === 410) {
      return Promise.reject(new Error('Resource expired or gone'));
    }

    const msg = data?.error || `Request failed (${status})`;
    return Promise.reject(new Error(msg));
  }
);

export default client;
