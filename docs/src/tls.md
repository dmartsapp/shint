---
title: tls
lead: Check a server's TLS certificate the way a browser would - the chain, whether it is trusted and valid for the name, when it expires - and the protocol and cipher the server chose.
description: shint tls connects to a server, completes a TLS handshake and reports the TLS version, the cipher suite and every certificate presented, whether the chain verifies, and how long until it expires, with warning and failure thresholds.
section: Commands
order: 4
nav: tls
---

## What it does

An expired or misissued certificate takes a service down as surely as a crashed server, and the error users see - "your connection is not private" - says nothing about which certificate, or why. `shint tls` connects to a server, completes a TLS handshake, and reports what the server presented: the TLS version and cipher suite it chose, every certificate it sent - subject, issuer, the names it is valid for, when it expires, its key - and then whether the chain leads to a trusted root and is valid for the name you asked for, the way a browser decides.

It checks every address the name resolves to, so one load-balanced server with an old certificate does not hide behind the others.

## Syntax

```bash
shint tls <host> [port] [--servername NAME] [--cacert FILE] [--warn-days N] [--fail-days N] [--count N] [--timeout S] [--json]
```

`port` is `443` unless given. `host` is a name or an address.

| Flag | Default | Meaning |
|---|---|---|
| `--servername NAME` | the host | The name sent in the handshake (SNI) and verified against the certificate. Set it to check an address (`shint tls 192.0.2.10 --servername www.example.com`), or one site of several on one server. |
| `--cacert FILE` | none | A PEM file of CA certificates to trust as well as the system's - for a private or internal CA. |
| `--warn-days N` | `30` | A certificate that expires within this many days gets a `WARN` line. The check still passes. `0` turns the warning off. |
| `--fail-days N` | `0` (off) | Turn expiry into a check: a certificate that expires within this many days fails it, and the command exits `1`. |

The other [shared flags](usage.md#flags-shared-by-every-command) apply: `-4`/`-6` choose the address family, and `--timeout` is how long each connection and handshake gets.

## What it reports

For each address, one `handshake` line, then the certificates, then the verdict:

| Line | Meaning |
|---|---|
| `handshake` | The handshake completed: `version` (`TLS 1.3` or `TLS 1.2`), `cipher` (the cipher suite the server chose) and how many certificates it sent. |
| `certificate` | One certificate, `depth=0` being the server's own and each next one its issuer: `subject`, `issuer`, `not_after` and `expires_in` (in days, or hours under two days), `names` (the host names and addresses it is valid for), `key` (`ECDSA P-256`, `RSA 2048`, ...) and `signature`. A certificate is listed once per run, not once per address or attempt. |
| `verified` | The chain leads to a trusted root and the certificate is valid for the name: `chain` is the path, from the server's certificate to the root that is trusted, and `trust` says which roots - `system`, or `system and --cacert`. |
| `verification failed` | It does not, with the reason: an unknown authority, an expired or not-yet-valid certificate, a name the certificate is not valid for. |
| `expires soon` | A `WARN`: a certificate in the verified chain expires within `--warn-days`. |
| `expires too soon` | An `ERROR`: within `--fail-days`. |
| `handshake failed` | No handshake: nothing listens there, the server did not answer with TLS, or it refused the handshake - each said in plain words. |

Expiry is judged on the chain that verified - the server's certificate, its intermediates and the root a client trusts - not on every certificate sent: servers often add cross-signed certificates that clients never use, and one of those expiring does not matter.

shint speaks TLS 1.2 and 1.3. A server that speaks only TLS 1.1 or older fails the handshake, and the line says so.

## Examples

### A public site

```bash
shint tls example.com -4
```

```text
Mon Nov  9 10:23:46 CST 2026: [tls] OK dns resolved host=example.com addresses=2 ips=[104.20.23.154,172.66.147.243] time=52.809208ms
Mon Nov  9 10:23:46 CST 2026: [tls] OK dns authoritative zone=example.com nameservers=[elliott.ns.cloudflare.com,hera.ns.cloudflare.com] time=46.694ms
Mon Nov  9 10:23:46 CST 2026: [tls] OK handshake server=example.com address=104.20.23.154 port=443 attempt=1/1 version="TLS 1.3" cipher=TLS_AES_128_GCM_SHA256 certificates=4 time=213.148792ms
Mon Nov  9 10:23:46 CST 2026: [tls] OK certificate depth=0 subject=CN=example.com issuer="CN=Cloudflare TLS Issuing ECC CA 3,O=SSL Corporation,C=US" not_after=2026-12-25 expires_in=46d names=[example.com,*.example.com] key="ECDSA P-256" signature=ECDSA-SHA256
Mon Nov  9 10:23:46 CST 2026: [tls] OK certificate depth=1 subject="CN=Cloudflare TLS Issuing ECC CA 3,O=SSL Corporation,C=US" issuer="CN=SSL.com TLS Transit ECC CA R2,O=SSL Corporation,C=US" not_after=2035-05-27 expires_in=3121d key="ECDSA P-256" signature=ECDSA-SHA384
Mon Nov  9 10:23:46 CST 2026: [tls] OK certificate depth=2 subject="CN=SSL.com TLS Transit ECC CA R2,O=SSL Corporation,C=US" issuer="CN=SSL.com TLS ECC Root CA 2022,O=SSL Corporation,C=US" not_after=2037-10-17 expires_in=3995d key="ECDSA P-384" signature=ECDSA-SHA384
Mon Nov  9 10:23:46 CST 2026: [tls] OK certificate depth=3 subject="CN=SSL.com TLS ECC Root CA 2022,O=SSL Corporation,C=US" issuer="CN=AAA Certificate Services,O=Comodo CA Limited,L=Salford,ST=Greater Manchester,C=GB" not_after=2028-12-31 expires_in=783d key="ECDSA P-384" signature=SHA256-RSA
Mon Nov  9 10:23:46 CST 2026: [tls] OK verified server=example.com address=104.20.23.154 port=443 attempt=1/1 chain="CN=example.com > CN=Cloudflare TLS Issuing ECC CA 3,O=SSL Corporation,C=US > CN=SSL.com TLS Transit ECC CA R2,O=SSL Corporation,C=US > CN=SSL.com TLS ECC Root CA 2022,O=SSL Corporation,C=US" trust=system
Mon Nov  9 10:23:46 CST 2026: [tls] OK handshake server=example.com address=172.66.147.243 port=443 attempt=1/1 version="TLS 1.3" cipher=TLS_AES_128_GCM_SHA256 certificates=4 time=61.750375ms
Mon Nov  9 10:23:46 CST 2026: [tls] OK verified server=example.com address=172.66.147.243 port=443 attempt=1/1 chain="CN=example.com > CN=Cloudflare TLS Issuing ECC CA 3,O=SSL Corporation,C=US > CN=SSL.com TLS Transit ECC CA R2,O=SSL Corporation,C=US > CN=SSL.com TLS ECC Root CA 2022,O=SSL Corporation,C=US" trust=system
Mon Nov  9 10:23:46 CST 2026: [tls] OK done checks=2 passed=2 total_time=398.159416ms
```

Both addresses present the same chain, so its certificates are listed once.

### A private CA, and a certificate about to expire

```bash
shint tls 127.0.0.1 8443 --cacert ca.pem
```

```text
Mon Nov  9 10:30:26 CST 2026: [tls] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=128.208µs
Mon Nov  9 10:30:27 CST 2026: [tls] OK handshake server=127.0.0.1 address=127.0.0.1 port=8443 attempt=1/1 version="TLS 1.2" cipher=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 certificates=1 time=6.528833ms
Mon Nov  9 10:30:27 CST 2026: [tls] OK certificate depth=0 subject=CN=localhost issuer=CN=localhost not_after=2026-11-10 expires_in=23h names=[localhost,127.0.0.1,::1] key="ECDSA P-256" signature=ECDSA-SHA256
Mon Nov  9 10:30:27 CST 2026: [tls] OK verified server=127.0.0.1 address=127.0.0.1 port=8443 attempt=1/1 chain=CN=localhost trust="system and --cacert"
Mon Nov  9 10:30:27 CST 2026: [tls] WARN expires soon server=127.0.0.1 address=127.0.0.1 port=8443 attempt=1/1 warning="CN=localhost expires in 23h, on 2026-11-10 - within --warn-days 30"
Mon Nov  9 10:30:27 CST 2026: [tls] OK done checks=1 passed=1 total_time=1.03560275s
```

The chain is trusted through `--cacert`, so the check passes (exit `0`); the certificate expires within 30 days, so it says so in a `WARN` line.

### Not trusted, or not for this name

Without `--cacert`, the same server's certificate leads to no trusted root:

```text
Mon Nov  9 10:30:28 CST 2026: [tls] ERROR verification failed server=127.0.0.1 address=127.0.0.1 port=8443 attempt=1/1 error="signed by an unknown authority: \"CN=localhost\" is not a trusted root, and the server sent no certificate that leads to one (a private CA is trusted with --cacert)"
```

And asked for another name with `--servername app.internal`:

```text
Mon Nov  9 10:30:29 CST 2026: [tls] ERROR verification failed server=app.internal address=127.0.0.1 port=8443 attempt=1/1 error="certificate is valid for localhost, not app.internal"
```

Both exit `1`.

### Expiry as a check: --fail-days

```bash
shint tls example.com -4 --fail-days 60
```

```text
Mon Nov  9 10:30:30 CST 2026: [tls] OK handshake server=example.com address=172.66.147.243 port=443 attempt=1/1 version="TLS 1.3" cipher=TLS_AES_128_GCM_SHA256 certificates=4 time=65.343084ms
Mon Nov  9 10:30:30 CST 2026: [tls] OK verified server=example.com address=172.66.147.243 port=443 attempt=1/1 chain="CN=example.com > CN=Cloudflare TLS Issuing ECC CA 3,O=SSL Corporation,C=US > CN=SSL.com TLS Transit ECC CA R2,O=SSL Corporation,C=US > CN=SSL.com TLS ECC Root CA 2022,O=SSL Corporation,C=US" trust=system
Mon Nov  9 10:30:30 CST 2026: [tls] ERROR expires too soon server=example.com address=172.66.147.243 port=443 attempt=1/1 error="a certificate expires in 46d, within --fail-days 60"
```

The chain is trusted, but the certificate expires in 46 days, within 60: the check fails and the command exits `1` - a cron job's cue to renew. See the [cookbook](cookbook.md#will-a-certificate-expire-soon).

### JSON

```bash
shint tls 127.0.0.1 8443 --cacert ca.pem --delay 0 --json
```

```json
{
  "input_params": {
    "module_name": "tls",
    "sequential": false,
    "throttle": false,
    "host": "127.0.0.1",
    "from_port": 8443,
    "to_port": 8443,
    "protocol": "tcp",
    "timeout_ms": 5,
    "count": 1,
    "delay_ms": 0,
    "payload_bytes": 0,
    "method": "",
    "data": "",
    "headers": null
  },
  "module_name": "tls",
  "dns_lookup": {
    "hostname": "127.0.0.1",
    "resolved_addresses": [
      "127.0.0.1"
    ],
    "error": "",
    "success": true,
    "time_taken_µs": 80
  },
  "stats": [
    {
      "address": "127.0.0.1",
      "port": 8443,
      "server_name": "127.0.0.1",
      "success": true,
      "tls_version": "TLS 1.2",
      "cipher_suite": "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
      "verified": true,
      "trust": "system and --cacert",
      "verified_chain": [
        "CN=localhost"
      ],
      "certificates": [
        {
          "subject": "CN=localhost",
          "issuer": "CN=localhost",
          "dns_names": [
            "localhost"
          ],
          "ip_addresses": [
            "127.0.0.1",
            "::1"
          ],
          "not_before": "2026-11-09T15:30:24Z",
          "not_after": "2026-11-10T16:30:24Z",
          "expires_in_days": 1,
          "serial_number": "1209972062d104b9",
          "public_key": "ECDSA P-256",
          "signature_algorithm": "ECDSA-SHA256",
          "is_ca": true,
          "sha256_fingerprint": "bbc201546aa47a79bbd81ace03e7cc7374c850fcfd4184278854f87736bff0bc"
        }
      ],
      "expires_in_days": 1,
      "warnings": [
        "CN=localhost expires in 23h, on 2026-11-10 - within --warn-days 30"
      ],
      "sent_unixtime_µs": 1794241853293626,
      "recv_unixtime_µs": 1794241853296897,
      "time_taken_µs": 3271
    }
  ],
  "end_time_unixtime_µs": 1794241853310537,
  "start_time_unixtime_µs": 1794241853293532,
  "total_time_taken_µs": 17005,
  "error": ""
}
```

One stat per address and attempt, with every certificate in full each time. `expires_in_days` is fractional, and negative once a certificate has expired; at the top of a stat it is the soonest in the verified chain. A failed check has `success: false` and the reason in `error` - and, for a chain that did not verify, in `verify_error` too.

## Exit status

`0` when every address completed a handshake, its chain verified, and - with `--fail-days` - no certificate in it expires within that many days. `1` otherwise: the lookup failed, nothing answered, the handshake failed, a chain did not verify, a certificate expires within `--fail-days`, or the run was cut short with `Ctrl+C`. `2` for a usage error (a bad port, a negative `--warn-days` or `--fail-days`, a `--cacert` file that cannot be read). A `WARN` line never changes the exit status.

## Good to know

- **Unprivileged**: an ordinary TCP connection and a TLS handshake.
- **The handshake skips Go's own verification so every certificate can be shown**, trusted or not; the chain is then verified separately, against the name asked for and the system's roots (plus `--cacert`), as a browser would. Nothing is sent after the handshake.
- **An address checked by itself is verified against the certificate's IP addresses**, and no SNI is sent - as browsers do. A server that serves several sites needs `--servername` to pick one.
- **No STARTTLS** in this release: `tls` speaks TLS from the first byte, so it checks HTTPS, IMAPS (993), SMTPS (465), LDAPS (636) and the like, not a mail server's port 25 or 587.
- **`web` checks a site; `tls` checks its certificate.** `web` fails on a certificate it does not trust (or skips the check with `-k`); `tls` shows what the certificate is and why it is or is not trusted, and when it expires.
