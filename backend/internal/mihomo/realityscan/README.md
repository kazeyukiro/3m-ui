# REALITY target selection

## Reviewed catalog

`candidates.json` replaces the unreviewed 100-host text list. There is no target
count requirement. Every entry includes its sources, rationale, service kind,
`auto_select` decision, review time and vantage point, observed hosting signals,
known risks, decision explanation, and the last probe's evidence.

The initial 2026-09-07 review contains **11 entries, 3 enabled**:

| Host | Automatic selection | Review basis |
| --- | --- | --- |
| www.libreoffice.org | Yes | Project homepage inspected; repeated local probes passed |
| www.debian.org | Yes | Project homepage inspected; displayed ISO download uses cdimage.debian.org; repeated probes passed |
| www.videolan.org | Yes | Project homepage inspected; displayed binary downloads use get.videolan.org; repeated probes passed |
| www.gimp.org | No | Fastly DNS alias |
| www.esa.int | No | cdnmix.net DNS alias; delivery behavior unreviewed |
| www.cisco.com | No | akadns.net alias; conservative delivery-network DNS exclusion |
| www.python.org | No | Fastly DNS alias |
| www.caltech.edu | No | Cloudflare DNS/response signals |
| one-piece.com | No | CloudFront response headers, despite CNAME equaling the hostname |
| www.fom-international.com | No | X-Served-By plus X-Timer cache signal |
| www.r-project.org | No | Alias to a CRAN archive host; bulk-content exposure unreviewed |

The three enabled entries were selected through **our own homepage review and
network probes**, not copied from a published list of proven-safe REALITY sites.
Their sources are the actual project websites and the upstream selection
criteria. Historical suggestions are attributed individually where applicable:

- https://github.com/XTLS/REALITY/blob/main/README.md
- https://www.v2ray-agent.com/archives/1689439383686
- https://x.com/v2rayagent/status/2093709520556208466 (risk context, not an allowlist)

Reviews were made from a local development machine. Its egress geographic region
was not independently verified, and the results are **not production VPS or
multi-region measurements**. Candidate records say this explicitly. Known CDN
signals cause conservative exclusion from automatic selection; they do not
prove that a particular endpoint permits exploitable cross-SNI forwarding.
Missing signals do not prove dedicated hosting. Even a normal homepage can serve
large content or be used to consume fallback bandwidth. This remains an explicit
risk in enabled records; homepage inspection does not certify the entire site.

## Selection rules

1. Prefer normal, stable HTTPS websites whose serving role can be reviewed. Do
   not approve based on brand, university status, popularity or hostname count.
2. Do not automatically approve bulk-download/update services or unreviewed
   shared endpoints. More hostnames behind one shared network add little diversity.
3. Require a complete review and a qualifying stored probe before setting
   `auto_select: true`. Catalog validation rejects missing provenance, invalid
   hostnames, incomplete reviews and incompatible enabled entries.
4. Fresh probes on the receiving node are still required. Review data never
   substitutes for live compatibility checks.
5. Retire entries by setting `auto_select: false` and explaining the reason.
   Disabled records stay available for maintenance audits, but never enter the
   automatic scan pool. Catalog changes never modify saved listeners.

## Runtime behavior

- Load only enabled catalog entries, shuffle and sample without replacement.
- Up to 10 hosts per batch and 30 per scan; a smaller pool is probed only once.
- Maximum 5 concurrent probes, 5 seconds per probe, 35 seconds per scan.
- Within a probe, dial at most 2 validated IPs concurrently, interleaving IPv4
  and IPv6 with a 200ms fallback delay and a 1-second limit per TCP attempt.
  A successful connection cancels the remaining attempts; the probe's total
  timeout still includes DNS, TLS and HTTP checks.
- Stop after a batch produces at least 3 eligible targets across all batches,
  or when the pool is exhausted. One or two qualified results can still be used.
- Check the DNS canonical name for known delivery-network suffixes. Also reject
  Cloudflare, CloudFront, Akamai and Fastly-style header signals. These checks
  are deliberately conservative and not a comprehensive CDN classifier.
- Require trusted matching certificate, TLS 1.3, h2, X25519 or X25519MLKEM768,
  successful nonredirecting HTTP HEAD `/`, and no detected delivery-network signal.
- Read at most 4 KiB from `/cdn-cgi/trace`; never follow redirects. Bound headers
  and disable environment HTTP proxies. Validate DNS addresses and dial the
  checked address directly; private and reserved ranges are blocked.
- Keep handshakes within `min(1500, max(300, fastest * 3))` milliseconds and select
  randomly within that window. Return observations and the check timestamp.
- If no candidate qualifies (including an empty enabled pool), `selected` is null.
  Never fall back to Microsoft or a disabled candidate.

The authenticated `POST /api/v1/listeners/reality/scan` endpoint accepts no custom
targets and permits one scan per handler at a time. `/nodes/reality/scan` is its
route-group alias. Use the endpoint on the actual node panel; remote callers may
use the cluster proxy. The local form does not probe a separate remote server.

Only the new-listener form automatically requests a scan. Explicit rescan/change
actions can replace the target and SNI; editing fields cancels outstanding results.
Manually editing a proposed destination clears SNI and server names that still
match that scan's automatic values; independently customized names are preserved.
Empty names are derived from the new destination on save and in exported links.
Saving, editing, cloning, restarting and upgrading never automatically rotate a
target. An explicit destination remains required, including for API callers.

Fallback throttling remains a separate policy. The upstream README warns that
fixed throttling parameters can themselves be a detectable pattern. This scanner
neither changes throttling nor tests arbitrary-SNI forwarding or traffic exhaustion.

## API compatibility and example

Creating or updating a REALITY listener now requires an explicit, nonempty
`reality-config.dest`. Requests that previously relied on the Microsoft default
or a destination inferred from SNI now return HTTP 400. Saving does not run a
scan. Missing credentials are still generated on save, and existing saved
destinations are not automatically changed.

To use automatic selection from an API client:

1. Call `POST /api/v1/listeners/reality/scan` (alias
   `POST /api/v1/nodes/reality/scan`) on the panel that will run the listener,
   with `Authorization: Bearer <jwt>` and no request body.
2. If `selected` is null, retry or choose a target manually. Otherwise, copy
   `selected.target` into `reality-config.dest` inside the listener's decoded
   configuration. The scan only returns a proposal; it does not save a listener.
3. JSON-encode that configuration into the request's **string** `config` field
   and submit it to `POST /api/v1/listeners` (alias `POST /api/v1/nodes`).

Example creation body; replace `chosen.example.com:443` with the selected target
or an operator-reviewed destination, and choose an available listening port:

```json
{
  "name": "reality-node",
  "protocol": "vless",
  "port": "10443",
  "enabled": true,
  "config": "{\"reality-config\":{\"dest\":\"chosen.example.com:443\"}}"
}
```

The example hostname is a placeholder, not a candidate. Empty server names are
derived from the destination. For updates through `PUT /api/v1/listeners/{id}`
or `PUT /api/v1/nodes/{id}`, retain the saved destination in the submitted
configuration unless deliberately changing it, and preserve the other listener
fields and credentials. Update requests replace the listener; they are not
partial patches. When deliberately changing the destination, also update any
nonempty `reality-config.server-names` and `access_sni` that refer to the old
target; API saves preserve those explicitly supplied names.

## Maintenance

Run from the backend directory on the network you want to document:

```sh
go run ./cmd/reality-target-audit -vantage 'Describe the actual server/network' > candidate-review.json
```

The CLI checks all catalog entries, including disabled ones, sequentially with
bounded per-target timeouts. It prints observations only. It does not change the
catalog, promote candidates, write to a database, or update listeners. Review the
report, investigate hosting/content risks, then deliberately update the candidate
record and rebuild. The automatic API cannot probe disabled entries.

Repeat reviews when availability, hosting, certificates, redirects or risk reports
change. Expanding the list requires a source and an actual review, not a quota.
Stored reviews are historical evidence, not a promise of ongoing availability.

## Verification

```sh
go test -race ./internal/mihomo/realityscan ./internal/listener ./internal/router ./cmd/reality-target-audit
```

Automated tests use local TLS servers and synthetic probes, not public websites.
They verify provenance/approval gates, exclusion of disabled entries, small and
empty pools, batching, concurrency, cancellation, address guards, compatibility
and CDN signals, required destination validation and save stability. Algorithm
tests use synthetic hostnames so there is no need to retain real candidates just
to meet a test's size assumption.
