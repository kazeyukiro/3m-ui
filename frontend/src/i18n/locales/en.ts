import type { TranslationKeys } from '../types';

const en: TranslationKeys = {
  nav: {
    dashboard: 'Dashboard', listeners: 'Nodes', users: 'Users',
    core: 'Core', logs: 'Logs', config: 'Config', settings: 'Settings', logout: 'Logout',
  },
  common: {
    save: 'Save', cancel: 'Cancel', delete: 'Delete', edit: 'Edit', create: 'Create',
    confirm: 'Confirm', close: 'Close', copy: 'Copy', download: 'Download', refresh: 'Refresh',
    loading: 'Loading…', empty: 'No data', search: 'Search', actions: 'Actions', status: 'Status',
    enabled: 'Enabled', disabled: 'Disabled', name: 'Name', type: 'Type', port: 'Port',
    protocol: 'Protocol', address: 'Address', password: 'Password', username: 'Username',
    submit: 'Submit', success: 'Success', error: 'Error',
    confirmDelete: 'Are you sure to delete?', back: 'Back', generate: 'Generate', validate: 'Validate',
  },
  login: {
    title: '3M-UI', subtitle: 'Mihomo Core Management Console', username: 'Username', password: 'Password',
    button: 'Login', welcomeBack: 'Welcome back', failed: 'Login failed',
  },
  password: {
    title: 'Change Password', current: 'Current Password', new: 'New Password', confirm: 'Confirm New Password',
    button: 'Change Password', success: 'Password updated', failed: 'Failed to change password',
    mismatch: 'Passwords do not match',
  },
  dashboard: {
    title: 'Dashboard', subtitle: 'System & Core Overview', status: 'Core Status', running: 'Running',
    stoppedStatus: 'Stopped', version: 'Version', pid: 'PID', uptime: 'Uptime', listeners: 'Node Stats',
    total: 'Total', enabled: 'Enabled', disabled: 'Disabled', system: 'System Resources', cpu: 'CPU', memory: 'Memory',
    disk: 'Disk', network: 'Network', upload: 'Upload', download: 'Download', traffic: 'Traffic Stats',
  },
  listeners: {
    title: 'Nodes', create: 'Create Node', edit: 'Edit Node', name: 'Name', protocol: 'Protocol',
    port: 'Port', bindAddress: 'Bind Address', status: 'Status', actions: 'Actions',
    config: 'Protocol Config', udp: 'UDP', tls: 'TLS', enabled: 'Enabled', disabled: 'Disabled',
    deleteConfirm: 'Delete this node?', created: 'Created', updated: 'Updated', deleted: 'Deleted',
    reloaded: 'Reloaded', urisTitle: 'Subscription URIs', copyURI: 'Copy URI',
    portHint: 'Single port, range (8080-8090), or comma-separated list',
    usersHint: 'Create credentials under Users and bind them to this node; only official Listener fields are configured here.',
    sectionProtocol: 'Protocol', sectionTransport: 'Transport', sectionTLS: 'TLS',
    cipher: 'Cipher', psk: 'PSK', version: 'Version', obfsMode: 'Obfs Mode', obfsHost: 'Obfs Host',
    alterId: 'alterId', flow: 'Flow', wsPath: 'WS Path', grpcServiceName: 'gRPC Service Name',
    decryption: 'Decryption', passwordOptionalHint: 'Leave empty if credentials are bound under Users',
    passwordOptionalPlaceholder: 'Optional; injected from bound users',
    alterIdHint: 'Default alterId for bound VMess users',
    flowHint: 'Default flow for bound VLESS users (e.g. xtls-rprx-vision)',
    certificate: 'Certificate', privateKey: 'Private Key', clientAuthType: 'Client Auth Type',
    clientAuthCert: 'Client Auth Cert', echKey: 'ECH Key', allowInsecure: 'Allow Insecure',
    tokenPlaceholder: 'Leave empty to use bound users (TUIC v5)',
    mekyaKcpSection: 'Mekya KCP',
  },
  core: {
    title: 'Core Management', status: 'Status', start: 'Start', stop: 'Stop', restart: 'Restart',
    version: 'Version', path: 'Binary Path', configPath: 'Config Path', running: 'Running',
    stopped: 'Stopped', startSuccess: 'Core started', stopSuccess: 'Core stopped',
    restartSuccess: 'Core restarted',
  },
  logs: {
    title: 'Runtime Logs', clear: 'Clear', autoRefresh: 'Auto Refresh', level: 'Level',
    empty: 'No logs yet',
  },
  config: {
    title: 'Config Engine', visual: 'Visual', yaml: 'YAML', proxies: 'Proxies',
    addProxy: 'Add Proxy', editProxy: 'Edit Proxy', deleteProxy: 'Delete Proxy',
    deleteConfirm: 'Delete this proxy?', proxyName: 'Name', proxyType: 'Type',
    proxyServer: 'Server', proxyPort: 'Port', proxyPassword: 'Password', proxyUUID: 'UUID',
    yamlPreview: 'YAML Preview', validate: 'Validate', generate: 'Generate',
    generateSuccess: 'Config generated', validateSuccess: 'Config is valid',
    validateFailed: 'Config validation failed',
  },
  users: {
    title: 'User Management', create: 'Create User', edit: 'Edit User', username: 'Username',
    password: 'Password', enabled: 'Enabled', deleteConfirm: 'Delete this user?',
    created: 'Created', updated: 'Updated', deleted: 'Deleted',
    bind: 'Bind', bindNodes: 'Bind Nodes', bindHint: 'Select inbound nodes this user may use. Credentials are injected into the matching listeners when Mihomo config is generated.',
    selectNodes: 'Select nodes', bindSuccess: 'Node bindings updated',
  },
  settings: {
    title: 'Settings', subtitle: 'Preferences & Account', language: 'Language', theme: 'Theme',
    light: 'Light', dark: 'Dark', system: 'System', passwordTitle: 'Account Security',
    passwordDescription: 'Change your password regularly to improve security', changePassword: 'Change Password',
    about: 'About 3M-UI', version: 'Version',
  },
};

export default en;
