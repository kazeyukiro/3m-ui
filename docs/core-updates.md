# Updating the Mihomo core

Administrators can use **Core → Check for updates** to select an official stable
or Pre Mihomo release, or **Restore previous version** to return to the saved binary.
This updates Mihomo independently of the panel. Updates are manual; checking
for releases never installs one. The most recent 30 GitHub releases are queried,
with drafts and artifacts without a SHA-256 digest excluded.
Linux amd64 (baseline v1) and arm64 are supported.

The updater downloads only from `MetaCubeX/mihomo` on GitHub, verifies the release
asset's GitHub-provided SHA-256 digest, checks its executable version, and runs
its configuration test against the current configuration. These steps happen
before stopping the existing core. Downloads are limited to 64 MiB compressed
and 128 MiB decompressed, with request and validation timeouts.

For a running core, the updater verifies the existing listeners, stops the old
process, starts the candidate, and verifies the new process and known local
listening sockets. A failed switch restores the prior executable and running
state. If the core was stopped, installation keeps it stopped; only executable
and configuration validation are performed. Public connectivity, firewall rules,
and protocol handshakes are not tested by an update. Unsupported listener
transports are not covered by local socket inspection. Older releases may reject
newer configuration fields; configuration validation then rejects the update
before the running core is stopped.

## Persistent selection and recovery

The bundled `mihomo.binary` remains an immutable fallback. Managed binaries are
stored in `cores/` beside `mihomo.config`, normally
`/var/lib/3m-ui/mihomo/cores/`. The directory holds:

- `mihomo-<sha256>`: executable files for the active and previous versions.
- `selection.json`: schema 1, with active and previous version/checksum records.
- `job.json`: the most recent update result or interrupted job.
- `update.lock`: an advisory exclusive lock for updates and deployment tooling.

The initial bundled core is copied into this directory before the first switch,
so the previous version survives later panel/image upgrades too. The selected
version is recorded atomically only after successful validation and runtime
checks. Restarting during an unfinished switch therefore selects the previously
committed version. A corrupted or missing selected binary is reported rather
than silently resetting to the image's bundled version. Restore the directory
from backup to recover damaged selection metadata.

The normal Docker data volume already includes this directory. No Docker socket,
root permissions, or writable image filesystem are needed. Preserve that volume
when recreating containers. Native installations must keep this directory
writable and executable by the panel service; the supplied systemd unit already
permits writes in the data directory. A `noexec` data mount prevents core updates.
Do not share a data directory between multiple panel instances.

Panel upgrades must support selection schema 1 and preserve the selected binary.
`distribution/core-update.json` declares the storage contract to deployment tools.
Before replacing a panel, a deployment tool can hold `flock(LOCK_EX)` on
`cores/update.lock` through its backup, switch, and runtime checks. The in-panel
updater returns a conflict while this lock is held. Back up the entire core
configuration directory, including both binaries and selection metadata. Verify
the active core version and local listener states after deployment, separately
from the panel HTTP health check.

## API

All endpoints require the existing administrator authentication.

| Method | Endpoint | Result |
| --- | --- | --- |
| GET | `/api/v1/mihomo/releases` | Recent stable and Pre releases with usable platform artifacts and release notes |
| GET | `/api/v1/mihomo/update` | Capability, persistence/rollback state, and latest job |
| POST | `/api/v1/mihomo/update` | JSON `{"version":"v1.19.30"}`; returns HTTP 202 with the accepted job |
| POST | `/api/v1/mihomo/update/rollback` | Returns HTTP 202 with a job restoring the previous version |

POST accepts a version only, never an arbitrary download URL or executable path.
A concurrent update or deployment lock returns HTTP 409. Poll GET `/mihomo/update`
until the job is `succeeded` or `failed`; closing the page does not cancel it.
Failed jobs include their failure stage, error, and `rolled_back` result.
Start/stop/restart and configuration mutations are rejected during an update,
so queued actions cannot unexpectedly apply after the switch completes.
User and listener-binding changes that already committed to the database are
reconciled after either update success or failure by generating fresh configuration
from the latest database. Their API response can still report the temporary busy
error. Manual rollback does not require the current core to be healthy; it validates
the saved target and checks its listeners after starting it.

## Official Pre builds

The dropdown labels the official `Prerelease-Alpha` channel as **Pre** and shows
the concrete `alpha-<commit>` build. Stable remains the default; Pre requires an
explicit selection. Other prerelease channels and alternate Go builds are excluded.
Release lists are cached for one minute. Installation resolves the rolling tag
again and rejects a changed build instead of silently installing a different one.
The archive checksum and executable version must match the resolved metadata.
The selected build and content hash are persisted, so restart and rollback retain
that exact executable even after the upstream Pre release changes.

POST accepts the displayed build, for example `{"version":"alpha-dca26db"}`,
not the rolling tag. If the build has been replaced, check for updates again.
Pre versions may reject existing configuration; the usual validation and recovery
checks also apply to switches between stable and Pre.
