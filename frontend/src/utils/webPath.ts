// webPath returns the panel base path the UI is mounted under (e.g. "/a1b2c3"),
// or "" when the panel is served from the domain root.
//
// The value is injected by the backend into index.html as a <meta
// name="x-web-path" content="..."> tag (a <script> would be blocked by the
// page CSP). It is read once at module load; the panel is a pure SPA with no
// SSR, so document is always available when this runs.
export function webPath(): string {
  if (typeof document === 'undefined') return '';
  const meta = document.querySelector('meta[name="x-web-path"]');
  const value = meta?.getAttribute('content');
  if (!value) return '';
  const trimmed = value.trim();
  if (trimmed === '' || trimmed === '/') return '';
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
}
