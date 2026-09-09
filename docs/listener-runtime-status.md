# Listener runtime status

The node page separates the enabled setting from usability. The main badge
shows connection test passed, connection test failed, checking, not tested, unknown, or disabled.
A listening socket alone never produces an available badge. Clicking a node
opens diagnostics and reuses a still-valid result or starts a test using the exported client endpoint; saving an enabled node
also runs the test. The detail drawer separates socket observations, client
configuration, test-client startup, and authentication plus proxy access.

Lightweight socket polling remains visible-page-only. Polling never launches
test clients. End-to-end results are cached in memory for one minute and are
invalidated when the applied configuration, serving process or exported client
configuration changes. Cache reads re-export the current profile to validate a
hash, covering public endpoint settings and active credentials without retaining
credential-bearing YAML in the cache. A
request failure invalidates the displayed result rather than retaining green.

## API

These routes use the existing authenticated node/listener route groups:

- `GET /api/v1/nodes/runtime-status`: all local node runtime states.
- `POST /api/v1/nodes/:id/check`: one fresh, read-only runtime check.
- `/api/v1/listeners` provides the same aliases.

The list response is an array; the individual check returns one object.
`GET /api/v1/nodes/runtime-status?summary=true` verifies the same sockets but
returns empty `endpoints` arrays. The page uses these small summaries for
polling and fetches full endpoints only for an open drawer or a manual check.
An open drawer refreshes alongside the summaries; both show unknown on request
failure. Omitting `summary=true` preserves the existing full list response.
Individual checks always return full endpoints.

Example full response:

```json
{
  "id": 1,
  "state": "listening",
  "reason": "sockets_bound",
  "checked_at": "2026-09-08T14:00:00Z",
  "endpoints": [
    {"network": "tcp", "address": "0.0.0.0", "port": 10001, "bound": true}
  ]
}
```

States are `disabled`, `listening`, `not_listening`, and `unknown`; checking is
a frontend request state. Reasons are stable machine-readable codes, translated
in the frontend. No database schema or existing listener response is changed.
The endpoint does not return credentials, configuration contents, or process logs.

## Exported client endpoint test

`POST /api/v1/nodes/:id/connection-check` returns a runtime object with an
additional `connection_check` containing state, reason, source, timestamps,
expiry, tested endpoint, successful target, request duration, and diagnostic
steps. Existing runtime routes include a valid cached result when available.
The check uses the existing authentication middleware and accepts no target URL
or configuration from the request body. With `?reuse=true`, it reuses a valid
completed pass/fail result after checking the current profile; without that
option it always runs a fresh test. Opening diagnostics requests reuse, while
saving a node and manually selecting Test connection bypass the cache. Reused
results keep their original check and expiry timestamps.

The server exports existing credentials without creating or changing users,
bindings or defaults. It uses the same client exporter order as sharing, picks
one exported client profile, preserves authentication and TLS/REALITY options,
and keeps its actual server address, public port, SNI and transport settings.
Global public-port settings are used when the node has no override, matching
share export. No loopback endpoint is substituted. A missing configured public
host stops export rather than inventing a localhost target. A valid exported
server and single port are required; profiles depending on another proxy are
reported as unsupported rather than silently changing the route.

An isolated instance of the configured Mihomo binary runs with a private
0700 temporary directory, a 0600 configuration and a Unix controller socket.
It does not reload or stop the serving core, install a system proxy, or enable
TUN. No credentials or subprocess logs are returned. The client and temporary
files are cleaned up after completion or cancellation.

The client requests `https://www.gstatic.com/generate_204` and
`https://cp.cloudflare.com/generate_204` concurrently through the test proxy,
requiring HTTP 204. Each target retains its five-second Mihomo timeout (six
seconds for the controller request). The first success cancels the other
request; failure requires both to fail. Workers are cancelled and joined before
the client process is cleaned up, including when the caller disconnects. There is no direct fallback route. A successful request
proves that this exported profile can authenticate and access that target
through the node from this server. Failures cannot always distinguish protocol
errors from outbound or test-target outages; the UI explains this instead of
inventing an exact handshake failure.

Only one test client runs per panel at a time, with an overall 20-second bound.
Missing credentials, unsupported endpoints/platforms, client startup failure,
concurrent tests, and configuration changes are unknown rather than fabricated
node failures. Test traffic may count toward the selected user's traffic quota.
The tested route now includes the exported endpoint, so a missing port mapping
or blocked ingress can fail the test even when local sockets are healthy.
The request still originates on this server: NAT loopback and routing to its own
public address can differ from external ingress. Success is labeled as a passed
connection test, not proof that all external networks, users or destinations work.

Missing applied listeners stop the connection test before any client stages.
The drawer prioritizes the serving failure over a secondary client limitation
and shows a prominent explanation and next action. Invalid stored configuration
is distinguished from an unsupported test address; the legacy access-SNI issue
has a specific explanation. Certificate hostname hints are kept separate from
inbound fields, and an old generated `sni` matching the node's access SNI/public
host is removed on regeneration so the node is no longer silently skipped.

## Local socket inspection

On Linux, the checker reads the current Mihomo process's socket ownership via
`/proc`. One descriptor scan collects socket inodes and propagates permission
errors. Socket tables are streamed with cancellation checks, retaining only
owned TCP LISTEN and unconnected UDP sockets. A shared index matches protocol,
address, and port against the applied configuration. All ports in
a range must be present. A socket owned by another PID cannot satisfy a check.
UDP relayed inside a TCP protocol does not require a separate UDP socket.

An IPv4 wildcard may be implemented as an IPv6 dual-stack socket. The checker
uses Linux socket diagnostics (`INET_DIAG_SKV6ONLY`), matched to the owned socket
inode, before accepting it as covering IPv4. Diagnostics request only TCP
LISTEN or unconnected UDP states, once per needed protocol and observation.
The nonblocking receive loop honors the inspection deadline. It does not assume
the host's IPv6 default or infer ownership by connecting to the port.

Unsupported operating systems/transports, denied process inspection, or
unavailable address-family diagnostics produce `unknown`. In particular,
Shadowsocks with KCP tunneling is not verified in this first version. No new
capabilities or privileged-container mode are required for the normal child
process inspection; platform security policy can still restrict inspection.

`listening` means the local sockets are present. It does **not** verify protocol
handshakes, credentials, TLS, outbound connectivity, Docker port publishing,
host firewalls, cloud security groups, or reachability from the Internet. The UI
explicitly labels public reachability as unverified.

## Applying configuration

After starting or restarting Mihomo, configuration application waits up to
three seconds for the expected listeners. Expected port ranges are prepared
once and reused across retries. Descriptor/table loops, diagnostic receives,
and retry waits check the shared deadline; individual filesystem calls cannot
be forcibly interrupted. The ordinary status scan has a one-second budget.
Startup, configuration parsing, and rollback are outside these scan budgets.
A supported listener that is still missing returns an error through the existing
create/update flow and triggers configuration rollback. This catches cores that log a bind failure while the
process itself stays alive. Inspection that is unavailable is not treated as
proof of a binding failure; the UI reports unknown instead. If a completed
readiness observation already found a missing socket and a later scan exhausts
the deadline, the earlier failure remains an error rather than being silently
accepted. Applying configuration and rollback remain serialized.

The create/edit form remains open on errors and retains its input. Success
messages distinguish disabled nodes, successful exported-endpoint connection tests, failed
connection tests, and saved nodes whose usability could not be verified. A network timeout does not claim that a
server-side mutation has definitely been rolled back.

## Validation

Unit tests cover PID ownership, TCP versus UDP, incomplete port ranges,
IPv6-only versus dual-stack sockets, unsupported checks, and runtime API errors.
Tests also cover a missing middle port in a 40,001-port range, small summary
responses, request cancellation, malformed socket tables, deadline exhaustion,
and visibility-driven polling. The Linux tests exercise real owned sockets,
TCP/UDP dual-stack versus IPv6-only wildcard bindings, and rollback when a live
core fails to open a listener.

The initial implementation was manually validated with an isolated local server
and disposable database:
desktop and 390px mobile layouts showed disabled and failed nodes separately,
the detail drawer and manual recheck updated the check time, and a failed create
kept the entered name and port with a persistent error. Real-core checks also
passed in a non-root Docker container with all capabilities dropped and a
read-only root filesystem.

To additionally run against a real Mihomo binary without using any existing
panel configuration or database:

```sh
MIHOMO_TEST_BINARY=/opt/mihomo/mihomo go test ./internal/mihomo \
  -run TestRealMihomoListenerReadinessAndPortConflict -v
```

Run from `backend` on Linux. The test creates disposable configuration, checks
a real Shadowsocks TCP/UDP wildcard listener, occupies another port using the
test process, and verifies that applying the conflicting configuration fails
and the previous listener is restored. The test does not test public access.

## Optimization validation (2026-09-09)

The final optimization was validated on the second Oracle instance (Linux
ARM64), in isolated non-root containers with all capabilities dropped and a
read-only root filesystem. No production configuration or database was used.
The existing panel health check remained successful after testing.

- Go 1.25: the complete backend suite passed with `-tags sqlite_modernc`.
- The Mihomo and listener suites also passed with the default SQLite driver.
- The real-core test used a copy of the deployed Mihomo binary and disposable
  configuration; owned TCP/UDP, dual-stack, IPv6-only, and rollback tests passed.
- Frontend unit tests, lint, TypeScript checking, and the production build passed.

A synthetic before/after comparison used 40,001 UDP ports (10000–50000) and
40,001 owned sockets, rebuilding the endpoint range and producing full endpoint
results on each iteration. Both versions ran on the same host with one Go
execution thread, a 0.75 CPU container limit, and three measured iterations.

| Version | Time per operation | Allocated bytes per operation |
| --- | ---: | ---: |
| Original `244b860` | 5.791 s | 10,990,664 |
| Indexed matching | 27.46 ms | 16,793,208 |

The index trades additional temporary allocations for roughly 211 times faster
matching in this large-range case. These are cumulative allocations, not peak
resident memory. This comparison excludes `/proc` scanning, YAML parsing, and
HTTP transfer; it does not establish a production-wide speedup. Separate tests
verify that summary responses stay below 512 bytes for a single 40,001-port
listener while still detecting a missing middle port.

## Local connection validation (2026-09-09)

- The complete backend suite passed. The Mihomo, node, and user packages also
  passed with the Go race detector.
- Frontend unit tests, TypeScript, lint on the changed runtime components, and
  the production build passed. Vite retained its bundle-size warning.
- On Linux ARM64 with Mihomo v1.19.30, disposable Shadowsocks and VLESS + TLS
  listeners passed actual proxied HTTP 204 requests. Wrong passwords/UUIDs
  failed and did not reach the target, verifying there was no direct bypass.
- Cancelling an in-flight request stopped the temporary client and removed its
  temporary credentials. Serving processes and their configurations were intact.
- A disposable local panel was checked in desktop and 390px mobile layouts:
  stopped nodes showed unavailable, disabled nodes stayed disabled, and the
  diagnostic drawer retained the core-stopped reason through polling.

Run the isolated real-client tests from `backend` on Linux:

```sh
MIHOMO_TEST_BINARY=/opt/mihomo/mihomo go test ./internal/mihomo \
  -run TestRealMihomoLocalConnectionCheck -v
```

These tests use a controlled local HTTP target to distinguish credential
failures from external target outages. They do not validate public ingress or
the availability of the two production test URLs.

## Exported endpoint regression coverage

Profile preparation preserves the exported domain/IPv6 address, public port,
credentials, TLS/REALITY and transport options, including implicit SNI defaults.
Node export uses per-node public settings before global access settings and never
invents a localhost fallback. Real Mihomo tests keep a local listener healthy
while pointing the exported profile at a different, unlistened port: the check
must fail without reaching the target or falling back to the local listener.

## Connection latency optimization

Barrier-based tests prove both targets start before either finishes, a failed
target does not hide another target's success, and success/caller cancellation
stop the remaining requests. Cache tests cover expiry, process/config changes,
public-port/SNI/credential edits without an applied-YAML change, original
timestamps on reuse, and forced manual checks.
