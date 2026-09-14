package cluster

// SSRF validation for cluster base_url targets lives in
// internal/netutil (AssertHostAllowed / AssertIPAllowed / SafeDialContext /
// NewSafeHTTPClient). This file previously held a cluster-local copy of that
// logic; it has been centralized so every server-side fetcher (cluster sync,
// external-subscription merge, …) shares one dial-time-validated policy and
// cannot drift apart.
