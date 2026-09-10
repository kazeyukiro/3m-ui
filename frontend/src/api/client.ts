import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from '../stores/authStore';

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
});

client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = useAuthStore.getState().token;
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

/** True when the browser/axios aborted the request (navigation, polling, explicit cancel). */
export function isCanceledError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false;
  const e = err as { code?: string; name?: string; message?: string };
  if (e.code === 'ERR_CANCELED' || e.name === 'CanceledError' || e.name === 'AbortError') return true;
  if (typeof e.message === 'string' && /cancel|aborted/i.test(e.message)) return true;
  if (axios.isCancel?.(err)) return true;
  return false;
}

function apiErrorMessage(err: AxiosError<{ error?: string; code?: string; message?: string }>): string {
  const data = err.response?.data;
  if (data && typeof data === 'object') {
    if (typeof data.error === 'string' && data.error.trim()) return data.error;
    if (typeof data.message === 'string' && data.message.trim()) return data.message;
  }
  if (err.message) return err.message;
  return 'Request failed';
}

client.interceptors.response.use(
  (res) => res,
  (err: AxiosError<{ error?: string; code?: string; message?: string }>) => {
    // Aborted polls / navigation must not surface as "backend unreachable".
    if (isCanceledError(err) || err.code === 'ERR_CANCELED') {
      const cancelErr = new Error('Request canceled');
      cancelErr.name = 'CanceledError';
      (cancelErr as Error & { code?: string }).code = 'ERR_CANCELED';
      return Promise.reject(cancelErr);
    }

    if (!err.response) {
      if (err.code === 'ECONNABORTED' || err.code === 'ETIMEDOUT') {
        return Promise.reject(
          new Error(
            'Backend request timed out; the operation may still be in progress. Check the current status before retrying.',
          ),
        );
      }
      if (err.request) {
        // Network Error, connection reset, offline, or proxy dropped the response.
        const detail =
          typeof err.message === 'string' && err.message && err.message !== 'Network Error'
            ? ` (${err.message})`
            : '';
        return Promise.reject(
          new Error(
            `Cannot reach the panel API at /api/v1${detail}. Confirm 3m-ui is running (systemctl status 3m-ui), the panel port is open, and you are not blocking the request with a proxy or mixed-content policy.`,
          ),
        );
      }
      return Promise.reject(new Error(err.message || 'Backend request failed'));
    }
    const { status, data } = err.response;

    if (status === 401) {
      const path = window.location.pathname;
      if (path !== '/login') {
        useAuthStore.getState().logout();
        window.location.href = '/login';
      }
      return Promise.reject(new Error(apiErrorMessage(err) || 'Session expired'));
    }
    if (status === 403 && data?.code === 'PASSWORD_CHANGE_REQUIRED') {
      useAuthStore.getState().setMustChangePassword(true);
      if (window.location.pathname !== '/change-password') {
        window.location.href = '/change-password';
      }
      return Promise.reject(new Error('Password change required'));
    }
    if (status === 408 || status === 504) {
      return Promise.reject(
        new Error('Backend request timed out; the operation may still be in progress. Check the current status before retrying.'),
      );
    }
    if (status === 429) {
      const retryAfter = err.response.headers?.['retry-after'];
      return Promise.reject(
        new Error(retryAfter ? `Too many requests; retry after ${retryAfter} seconds` : 'Too many requests; please try again later'),
      );
    }
    if (status >= 500) {
      return Promise.reject(new Error(data?.error || `Backend error (${status})`));
    }
    if (status === 410) {
      return Promise.reject(new Error(data?.error || 'Resource gone'));
    }
    return Promise.reject(new Error(apiErrorMessage(err)));
  },
);

export default client;
