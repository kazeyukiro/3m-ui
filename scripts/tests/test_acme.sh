#!/usr/bin/env bash
# Tests for the acme.sh helper in ../3m-ui.
#
# acme.sh is never executed for real here: a stub is put in $ACME_HOME and
# every invocation is recorded to $ACME_LOG, so the tests assert on the exact
# flags the menu builds. Flags are checked because acme.sh itself rejects
# values it does not know, and a wrong one only surfaces minutes later during
# an actual issuance.
#
# Usage: scripts/tests/test_acme.sh
set -u

here=$(cd "$(dirname "$0")" && pwd)
target=$here/../3m-ui
[ -f "$target" ] || { echo "cannot find $target"; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# Everything above the top-level command dispatch is pure definitions, so it
# can be sourced as a library. Locate the dispatch instead of hardcoding it.
dispatch=$(awk '/^main\(\) \{/ { print NR; exit }' "$target")
[ -n "$dispatch" ] || { echo "cannot locate command dispatch"; exit 1; }
lib=$work/lib.sh
sed -n "1,$((dispatch - 1))p" "$target" > "$lib"

stub=$work/acme-home
mkdir -p "$stub"
cat > "$stub/acme.sh" <<'EOS'
#!/bin/sh
printf '%s\n' "$*" >> "$ACME_LOG"
exit ${ACME_EXIT:-0}
EOS
chmod +x "$stub/acme.sh"

export ACME_HOME=$stub
export ACME_LOG=$work/acme.log
# The script derives CONFIG_DIR from ROOT (CONFIG_DIR="$ROOT/etc/3m-ui") and
# overwrites any preset value, so redirect the tree via THREE_M_UI_ROOT.
export THREE_M_UI_ROOT=$work/root
export THREE_M_UI_LANG=zh
: > "$ACME_LOG"

pass=0
fail=0

ok() { printf 'PASS | %s\n' "$1"; pass=$((pass + 1)); }
no() { printf 'FAIL | %s\n' "$1"; printf '       %s\n' "$2"; fail=$((fail + 1)); }

# expect NAME NEEDLE  — last recorded acme.sh invocation must contain NEEDLE
expect() {
    last=$(tail -n 1 "$ACME_LOG" 2>/dev/null)
    if [ -z "$last" ]; then
        no "$1" "acme.sh was never invoked"
    elif printf '%s' "$last" | grep -qF -- "$2"; then
        ok "$1"
    else
        no "$1" "want substring: $2 | got: $last"
    fi
}

# expect_absent NAME NEEDLE — last invocation must NOT contain NEEDLE
expect_absent() {
    last=$(tail -n 1 "$ACME_LOG" 2>/dev/null)
    if [ -z "$last" ]; then
        no "$1" "acme.sh was never invoked"
    elif printf '%s' "$last" | grep -qF -- "$2"; then
        no "$1" "must not contain: $2 | got: $last"
    else
        ok "$1"
    fi
}

# call INPUT CMD... — run a library function with INPUT on stdin
call() {
    input=$1
    shift
    # Trailing newline: without it the final read hits EOF-with-data.
    printf '%b\n' "$input" | bash -c ". $lib >/dev/null 2>&1; $* >/dev/null 2>&1"
}

# not_called NAME INPUT CMD... — the function must refuse before invoking acme.sh
not_called() {
    name=$1
    input=$2
    shift 2
    : > "$ACME_LOG"
    call "$input" "$@"
    if [ -s "$ACME_LOG" ]; then
        no "$name" "acme.sh was invoked with: $(cat "$ACME_LOG")"
    else
        ok "$name"
    fi
}

echo "== acme.sh helper: command construction =="

# CA menu: 1 = letsencrypt. standalone flow: confirm, httpport, then the
# optional flags (staging, keylength, dnssleep, days, debug, ocsp, chain, eab).
: > "$ACME_LOG"
call '1\ny\n\nn\n\n\n\n\nn\n\n\n\n' 'acme_issue standalone example.com'
expect "standalone: --issue --standalone" '--issue -d example.com --standalone'
expect "standalone: --server letsencrypt" '--server letsencrypt'

: > "$ACME_LOG"
call '1\ndns_cf\nn\n\n\n\n\nn\n\n\n' 'acme_issue dns example.com'
expect "dns: --dns hook" '--dns dns_cf'

: > "$ACME_LOG"
call '4\ndns_cf\nn\n\n\n\n\nn\n\n\n' 'acme_issue dns example.com'
expect "dns: CA menu item 4 = ssl.com" '--server sslcom'

: > "$ACME_LOG"
call '1\ndns_cf\nn\nec-256\n30\n\n\nn\n\n\n' 'acme_issue dns example.com'
expect "dns: --keylength ec-256" '--keylength ec-256'
expect "dns: --dnssleep 30" '--dnssleep 30'

: > "$ACME_LOG"
call '1\n/var/www/html\nn\n\n\n\n\nn\n\n\n' 'acme_issue webroot example.com'
expect "webroot: --webroot" '--webroot /var/www/html'
expect_absent "webroot: no --standalone" '--standalone'

: > "$ACME_LOG"
call '1\n8443\nn\n\n\n\n\nn\n\n\n' 'acme_issue alpn example.com'
expect "alpn: --alpn" '--alpn'
expect "alpn: --tlsport" '--tlsport 8443'

: > "$ACME_LOG"
call '1\nn\n\n\n\n\nn\n\n\n' 'acme_issue nginx example.com'
expect "nginx: --nginx" '--nginx'

: > "$ACME_LOG"
call '1\nn\n\n\n\n\nn\n\n\n' 'acme_issue apache example.com'
expect "apache: --apache" '--apache'

: > "$ACME_LOG"
call '1\ny\n8080\nn\n\n\n\n\nn\n\n\n' 'acme_issue standalone example.com'
expect "standalone: --httpport" '--httpport 8080'

# Every optional flag at once.
: > "$ACME_LOG"
call '1\ny\n\ny\n\n\n\n\ny\nISRG Root X1\nkid123\nhmac456' 'acme_issue standalone example.com'
expect "advanced: --staging" '--staging'
expect "advanced: --ocsp-must-staple" '--ocsp-must-staple'
expect "advanced: --preferred-chain keeps spaces" '--preferred-chain ISRG Root X1 --eab-kid kid123'
expect "advanced: --eab-hmac-key" '--eab-hmac-key hmac456'

: > "$ACME_LOG"
call '1\ny\n\nn\n\n\n\n2\nn\n\n\n' 'acme_issue standalone example.com'
expect "advanced: --debug 2" '--debug 2'

echo "== input validation (must refuse before calling acme.sh) =="

not_called "reject keylength 1024" '1\ny\n\nn\n1024\n' 'acme_issue standalone example.com'
not_called "reject keylength bogus" '1\ny\n\nn\nnot-a-size\n' 'acme_issue standalone example.com'
not_called "reject debug 9" '1\ny\n\nn\n\n\n\n9\n' 'acme_issue standalone example.com'
not_called "reject --days abc" '1\ny\n\nn\n\nabc\n' 'acme_issue standalone example.com'
not_called "reject --dnssleep abc" '1\ny\n\nn\n\nabc\n' 'acme_issue dns example.com'
not_called "reject --httpport abc" '1\ny\nabc\n' 'acme_issue standalone example.com'
not_called "reject --tlsport abc" '1\nabc\n' 'acme_issue alpn example.com'
not_called "reject half-filled EAB (kid only)" '1\ny\n\nn\n\n\n\n\nn\n\nkid123\n' 'acme_issue standalone example.com'
not_called "cancel when domain is empty" '\n' 'acme_issue standalone'
not_called "cancel when webroot is empty" '1\n\n' 'acme_issue webroot example.com'

echo "== IP certificates (RFC 8738) =="

# acme.sh issues an IP certificate by passing the address straight to -d
# (wiki/ipcert), so http-01/tls-alpn-01 modes must accept it as-is.
: > "$ACME_LOG"
call '1\ny\n\nn\n\n\n\n\nn\n\n\n\n' 'acme_issue standalone 1.2.3.4'
expect "IP: -d keeps the address" '--issue -d 1.2.3.4 --standalone'

: > "$ACME_LOG"
call '1\n/var/www/html\nn\n\n\n\n\nn\n\n\n' 'acme_issue webroot 203.0.113.7'
expect "IP: webroot accepts it too" '--issue -d 203.0.113.7 --webroot'

: > "$ACME_LOG"
call '1\n8443\nn\n\n\n\n\nn\n\n\n' 'acme_issue alpn 2001:db8::1'
expect "IP: IPv6 over TLS-ALPN" '--issue -d 2001:db8::1 --alpn'

# DNS-01 cannot validate an IP identifier; refuse before asking for API keys.
not_called "IP: DNS validation refused (IPv4)" '1\n' 'acme_issue dns 1.2.3.4'
not_called "IP: DNS validation refused (IPv6)" '1\n' 'acme_issue dns 2001:db8::1'

# A hostname keeps working.
: > "$ACME_LOG"
call '1\ndns_cf\nn\n\n\n\n\nn\n\n\n' 'acme_issue dns example.com'
expect "domain: DNS still allowed" '--issue -d example.com --dns dns_cf'

echo "== certificate management =="

: > "$ACME_LOG"
call '' 'acme_renew example.com'
expect "renew: --renew" '--renew -d example.com'
expect_absent "renew: no --ecc for RSA certs" '--ecc'

mkdir -p "$stub/example.com_ecc"
: > "$ACME_LOG"
call '' 'acme_renew example.com'
expect "renew: --ecc for ECC certs" '--renew -d example.com --ecc'
rmdir "$stub/example.com_ecc"

: > "$ACME_LOG"
call 'n\n' 'acme_renew_all'
expect "renew-all: --renew-all" '--renew-all'
expect_absent "renew-all: no -se by default" '-se'

: > "$ACME_LOG"
call 'y\n' 'acme_renew_all'
expect "renew-all: -se when asked" '--renew-all -se'

: > "$ACME_LOG"
call 'example.com\ny\n1\n' 'acme_revoke'
expect "revoke: --revoke" '--revoke -d example.com'
expect "revoke: --revoke-reason" '--revoke-reason 1'

not_called "reject revoke-reason 99" 'example.com\ny\n99\n' 'acme_revoke'

: > "$ACME_LOG"
call 'example.com\nsecret\n' 'acme_export_pfx'
expect "pfx: --to-pkcs12" '--to-pkcs12 -d example.com'
expect "pfx: --password" '--password secret'

: > "$ACME_LOG"
call 'example.com\n\n' 'acme_export_pfx'
expect_absent "pfx: no --password when blank" '--password'

: > "$ACME_LOG"
call 'example.com\ny\n' 'acme_remove'
expect "remove: --remove" '--remove -d example.com'
expect_absent "remove: --remove takes no --ecc" '--ecc'

: > "$ACME_LOG"
call '1\n' 'acme_set_default_ca'
expect "set-default-ca" '--set-default-ca --server letsencrypt'

: > "$ACME_LOG"
call '6\n' 'acme_set_default_ca'
expect "set-default-ca: googletest short name" '--set-default-ca --server googletest'

# acme_deploy refuses when acme.sh has no cert directory for the domain.
: > "$ACME_LOG"
call 'example.com\n' 'acme_deploy example.com'
if [ -s "$ACME_LOG" ]; then
    no "deploy: refuses without a cert directory" "invoked with: $(cat "$ACME_LOG")"
else
    ok "deploy: refuses without a cert directory"
fi

mkdir -p "$stub/example.com"
: > "$ACME_LOG"
call 'example.com\n' 'acme_deploy example.com'
expect "deploy: --install-cert" '--install-cert -d example.com'
expect "deploy: --fullchain-file" '--fullchain-file'
rmdir "$stub/example.com"

echo "== private key discovery =="

# _acme_key_path locates the key behind "view private key" in cert info.
# acme.sh stores ECC certificates in <domain>_ecc/ and issues ECC by default,
# so checking only <domain>/ left every default certificate without a key.
mkdir -p "$stub/example.com_ecc"
: > "$stub/example.com_ecc/example.com.key"
: > "$ACME_LOG"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path example.com" 2>&1)
if [ "$got" = "$stub/example.com_ecc/example.com.key" ]; then
    ok "key: ECC cert key found in <domain>_ecc"
else
    no "key: ECC cert key found in <domain>_ecc" "got: $got"
fi
rm -rf "$stub/example.com_ecc"

# RSA certificates keep the classic layout.
mkdir -p "$stub/rsa.example.com"
: > "$stub/rsa.example.com/rsa.example.com.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path rsa.example.com" 2>&1)
if [ "$got" = "$stub/rsa.example.com/rsa.example.com.key" ]; then
    ok "key: RSA cert key found in <domain>"
else
    no "key: RSA cert key found in <domain>" "got: $got"
fi
rm -rf "$stub/rsa.example.com"

# When both layouts exist the ECC copy must not shadow the RSA one... but a
# key deployed into the panel wins over both: it is the key actually in use.
# CERT_DIR derives from THREE_M_UI_ROOT: $ROOT/etc/3m-ui/certs.
certdir=$work/root/etc/3m-ui/certs
mkdir -p "$stub/both.example.com" "$stub/both.example.com_ecc" "$certdir"
: > "$stub/both.example.com/both.example.com.key"
: > "$stub/both.example.com_ecc/both.example.com.key"
: > "$certdir/both.example.com.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path both.example.com" 2>&1)
if [ "$got" = "$certdir/both.example.com.key" ]; then
    ok "key: deployed key in CERT_DIR wins"
else
    no "key: deployed key in CERT_DIR wins" "got: $got"
fi
rm -rf "$stub/both.example.com" "$stub/both.example.com_ecc" "$certdir"

# No key anywhere must fail loudly rather than print a stale --list column.
: > "$ACME_LOG"
bash -c ". $lib >/dev/null 2>&1; _acme_key_path nowhere.example.com" >/dev/null 2>&1
if [ $? -ne 0 ]; then
    ok "key: fails when no key exists"
else
    no "key: fails when no key exists" "returned success without a key file"
fi

# Discovery is filesystem-only: acme.sh itself must not be invoked, so a
# missing key can never be papered over by a bogus --list fallback.
if [ -s "$ACME_LOG" ]; then
    no "key: no acme.sh invocation during discovery" "invoked with: $(cat "$ACME_LOG")"
else
    ok "key: no acme.sh invocation during discovery"
fi

# acme.sh builds the directory from the domain string verbatim, so the two
# layouts cover every certificate type it can issue: wildcards keep the
# leading "*." (also proves the paths stay glob-safe), IDN names keep their
# UTF-8 form instead of punycode, and IP addresses are plain directory names.
# All verified against acme.sh v3.1.7 with --create-domain-key / --issue.
mkdir -p "$stub/*.example.com_ecc"
: > "$stub/*.example.com_ecc/*.example.com.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path '*.example.com'" 2>&1)
if [ "$got" = "$stub/*.example.com_ecc/*.example.com.key" ]; then
    ok "key: wildcard cert (*.example.com)"
else
    no "key: wildcard cert (*.example.com)" "got: $got"
fi
rm -rf "$stub/*.example.com_ecc"

mkdir -p "$stub/münchen.de_ecc"
: > "$stub/münchen.de_ecc/münchen.de.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path 'münchen.de'" 2>&1)
if [ "$got" = "$stub/münchen.de_ecc/münchen.de.key" ]; then
    ok "key: IDN cert keeps the UTF-8 name"
else
    no "key: IDN cert keeps the UTF-8 name" "got: $got"
fi
rm -rf "$stub/münchen.de_ecc"

mkdir -p "$stub/1.2.3.4_ecc"
: > "$stub/1.2.3.4_ecc/1.2.3.4.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path 1.2.3.4" 2>&1)
if [ "$got" = "$stub/1.2.3.4_ecc/1.2.3.4.key" ]; then
    ok "key: IPv4 cert (default ECC)"
else
    no "key: IPv4 cert (default ECC)" "got: $got"
fi
rm -rf "$stub/1.2.3.4_ecc"

mkdir -p "$stub/1.2.3.4"
: > "$stub/1.2.3.4/1.2.3.4.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path 1.2.3.4" 2>&1)
if [ "$got" = "$stub/1.2.3.4/1.2.3.4.key" ]; then
    ok "key: IPv4 cert (RSA keylength)"
else
    no "key: IPv4 cert (RSA keylength)" "got: $got"
fi
rm -rf "$stub/1.2.3.4"

# Colons are legal in Linux file names, and the path is always quoted.
mkdir -p "$stub/2001:db8::1_ecc"
: > "$stub/2001:db8::1_ecc/2001:db8::1.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path '2001:db8::1'" 2>&1)
if [ "$got" = "$stub/2001:db8::1_ecc/2001:db8::1.key" ]; then
    ok "key: IPv6 cert"
else
    no "key: IPv6 cert" "got: $got"
fi
rm -rf "$stub/2001:db8::1_ecc"

# A domain can hold both an RSA and an ECC certificate. The RSA layout wins
# here, which matches the order acme_cert_info uses for the certificate
# itself, so the displayed key always belongs to the displayed certificate.
mkdir -p "$stub/dual.example.com" "$stub/dual.example.com_ecc"
: > "$stub/dual.example.com/dual.example.com.key"
: > "$stub/dual.example.com_ecc/dual.example.com.key"
got=$(bash -c ". $lib >/dev/null 2>&1; _acme_key_path dual.example.com" 2>&1)
if [ "$got" = "$stub/dual.example.com/dual.example.com.key" ]; then
    ok "key: RSA layout wins when both exist (matches cert lookup order)"
else
    no "key: RSA layout wins when both exist (matches cert lookup order)" "got: $got"
fi
rm -rf "$stub/dual.example.com" "$stub/dual.example.com_ecc"

echo "== CA list sanity =="

if grep -q 'buypass' "$target"; then
    no "buypass removed (service terminated 2025-10-31)" "still present in $target"
else
    ok "buypass removed (service terminated 2025-10-31)"
fi

# Only short names acme.sh actually accepts for --server (wiki/Server).
bad_ca=0
for ca in letsencrypt letsencrypt_test zerossl sslcom google googletest actalis; do
    grep -q "printf '$ca'" "$target" || bad_ca=$((bad_ca + 1))
done
[ "$bad_ca" -eq 0 ] && ok "all 7 CA short names present" \
    || no "all 7 CA short names present" "$bad_ca missing"

echo "== English strings =="

export THREE_M_UI_LANG=en
: > "$ACME_LOG"
call '1\ny\n\nn\n\n\n\n\nn\n\n\n\n' 'acme_issue standalone example.com'
out=$(call '1\ny\n\nn\n\n\n\n\nn\n\n\n\n' 'acme_issue standalone example.com' 2>&1)
# A missing i18n key falls back to printing the raw key name.
if printf '%s' "$out" | grep -qE '\b(uimode|adv_|ssl_|acme_|cert_)[a-z_]*\b'; then
    no "no raw i18n keys leaked" "$out"
else
    ok "no raw i18n keys leaked"
fi

echo
printf 'PASS=%d FAIL=%d\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
