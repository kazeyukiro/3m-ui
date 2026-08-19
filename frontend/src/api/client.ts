import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from '../stores/authStore';

const client = axios.create({
  baseURL: '/api/v1',
  // Mutating operations can legitimately trigger Mihomo validation/restart.
  // Keep a finite timeout, but don't make normal restart/apply operations look
  // like a dead backend after only 15 seconds.
  timeout: 30000,
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
      if (err.code === 'ECONNABORTED' || err.code === 'ETIMEDOUT') {
        return Promise.reject(new Error('Backend request timed out; the operation may still be in progress. Check the current status before retrying.'));
      }
      if (err.request) {
        return Promise.reject(new Error('Backend service unreachable or connection was interrupted'));
      }
      return Promise.reject(new Error(err.message || 'Backend request failed'));
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
    if (status === 408 || status === 504) {
      return Promise.reject(new Error('Backend request timed out; the operation may still be in progress. Check the current status before retrying.'));
    }
    if (status === 429) {
      const retryAfter = err.response.headers?.['retry-after'];
      return Promise.reject(new Error(retryAfter ? `Too many requests; retry after ${retryAfter} seconds` : 'Too many requests; please try again later'));
    }
    if (status >= 500) {
      return Promise.reject(new Error(data?.error || `Backend error (${status})`));
    }
    if (status === 410) {
      return Promise.reject(new Error('Resource expired or gone'));
    }

    const msg = data?.error || `Request failed (${status})`;
    return Promise.reject(new Error(msg));
  }
);

export default client;
