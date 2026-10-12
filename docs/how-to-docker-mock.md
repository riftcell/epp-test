# How-To: Test Any EPP/RRI Client Against the Docker Mock

You don't need Go to use the mock servers. The `riftcell/epp-mock` image contains the standalone mock EPP and DENIC RRI servers as static binaries, so a client written in PHP, Python, Java or any other language can test against it.

The mock checks that your client frames messages correctly, completes the TLS handshake and handles the session flow (greeting, login, commands, logout). It also handles [fault injection](how-to-fault-injection.md). It is **not** a full registry simulation: see [What the mock returns](#what-the-mock-returns).

## Start the Servers

```sh
# EPP over TLS on port 7700
docker run --rm -p 7700:7700 riftcell/epp-mock:latest

# DENIC RRI over plain TCP on port 7701
docker run --rm -p 7701:7701 --entrypoint /mock-rri-server riftcell/epp-mock:latest -addr 0.0.0.0:7701
```

Your client connects to `127.0.0.1:7700` (EPP) or `127.0.0.1:7701` (RRI). Use any username and password: the mock accepts all credentials. Each server logs the commands it receives to stderr, so `docker logs` shows what your client sent.

Use the tag `v1.0` instead of `latest` to pin a version. To build the image from source, run `make docker-mock-servers`.

### Passing Flags

Any arguments after the image name **replace** the default `-addr 0.0.0.0:7700`, so always repeat `-addr`:

```sh
# Reject every login with code 2200
docker run --rm -p 7700:7700 riftcell/epp-mock:latest -addr 0.0.0.0:7700 --login-mode=always
```

Without `-addr 0.0.0.0:…` the server listens on the container's loopback and can't be reached from outside. See [Fault Injection](how-to-fault-injection.md) for all flags; replace `go run ./cmd/mock-epp-server` with the `docker run` prefix above.

### Docker Compose

When your client runs in another container, it reaches the mock by service name. Pass `-san` with that name so the server's certificate matches the hostname:

```yaml
services:
  mock-epp:
    image: riftcell/epp-mock:latest
    command: ["-addr", "0.0.0.0:7700", "-san", "mock-epp"]

  mock-rri:
    image: riftcell/epp-mock:latest
    entrypoint: ["/mock-rri-server"]
    command: ["-addr", "0.0.0.0:7701"]

  client-tests:
    image: your-client-under-test   # e.g. your PHP test image running phpunit
    depends_on: [mock-epp, mock-rri]
    environment:
      EPP_HOST: mock-epp
      EPP_PORT: "7700"
      RRI_HOST: mock-rri
      RRI_PORT: "7701"
```

These environment variables are read by your own client code, as in the PHP examples below; the mock itself ignores them.

## Wire Protocol Essentials

Both protocols put a 4-byte big-endian length header before each message, but **the header counts different things**. Getting this wrong is the most common bug.

| | EPP (port 7700) | RRI (port 7701) |
|---|---|---|
| Transport | TLS 1.2+ (no client certificate required) | Plain TCP |
| Length header | 4 bytes, big-endian, **includes the 4 header bytes** (RFC 5734) | 4 bytes, big-endian, **body only** |
| Body | EPP XML (RFC 5730) | `key: value` lines separated by `\n` |
| Who speaks first | Server sends `<greeting>` on connect | Client sends `LOGIN` |
| Logout | Server answers `1500` and closes the connection | Server answers `RESULT: success` and closes the connection |

Example: a 100-byte XML body is sent as EPP header `00 00 00 68` (104). The same 100-byte body in RRI is sent with header `00 00 00 64` (100).

### TLS

The EPP mock creates a new self-signed certificate every time it starts, so you can't pin or trust it in advance. Disable verification in test configuration only. In PHP:

```php
$ctx = stream_context_create(['ssl' => [
    'verify_peer'      => false,
    'verify_peer_name' => false,
]]);
```

The certificate is valid for `localhost` and `127.0.0.1`, plus any names given with `-san`.

## PHP Examples

Both scripts are tested against the image with PHP 8.3 (`php:8.3-cli`), with no extensions beyond the defaults. Run them with `php epp.php` and `php rri.php` while the containers are up.

### EPP

```php
<?php
// Minimal EPP client for the riftcell/epp-mock container (RFC 5730/5734).
$host = getenv('EPP_HOST') ?: '127.0.0.1';
$port = getenv('EPP_PORT') ?: '7700';

// The mock's certificate is self-signed and regenerated on every start: skip verification.
$ctx = stream_context_create(['ssl' => [
    'verify_peer'      => false,
    'verify_peer_name' => false,
]]);
$sock = stream_socket_client("tls://$host:$port", $errno, $errstr, 10, STREAM_CLIENT_CONNECT, $ctx);
if ($sock === false) {
    fwrite(STDERR, "connect failed: $errstr\n");
    exit(1);
}

// EPP frame: 4-byte big-endian length that INCLUDES the 4 header bytes, then the XML.
function epp_write($sock, string $xml): void {
    fwrite($sock, pack('N', strlen($xml) + 4) . $xml);
}

function read_exact($sock, int $n): string {
    $buf = '';
    while (strlen($buf) < $n) {
        $chunk = fread($sock, $n - strlen($buf));
        if ($chunk === false || $chunk === '') {
            throw new RuntimeException('connection closed');
        }
        $buf .= $chunk;
    }
    return $buf;
}

function epp_read($sock): string {
    $len = unpack('N', read_exact($sock, 4))[1];
    return read_exact($sock, $len - 4);
}

function result_code(string $xml): string {
    $doc = simplexml_load_string($xml);
    $doc->registerXPathNamespace('e', 'urn:ietf:params:xml:ns:epp-1.0');
    return (string) $doc->xpath('//e:result/@code')[0];
}

// 1. The server speaks first: read the greeting.
$greeting = epp_read($sock);
echo "greeting: ", str_contains($greeting, '<greeting>') ? 'ok' : 'MISSING', "\n";

// 2. Login (the mock accepts any clID/pw).
epp_write($sock, '<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><login>
<clID>any-user</clID><pw>any-password</pw>
<options><version>1.0</version><lang>en</lang></options>
<svcs><objURI>urn:ietf:params:xml:ns:domain-1.0</objURI></svcs>
</login><clTRID>php-1</clTRID></command></epp>');
echo "login: ", result_code(epp_read($sock)), "\n";

// 3. domain:check
epp_write($sock, '<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><check>
<domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
<domain:name>example.com</domain:name></domain:check>
</check><clTRID>php-2</clTRID></command></epp>');
$resp = epp_read($sock);
echo "check: ", result_code($resp), " avail=", str_contains($resp, 'avail="1"') ? '1' : '0', "\n";

// 4. Logout: server answers 1500 and closes the connection.
epp_write($sock, '<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><logout/>
<clTRID>php-3</clTRID></command></epp>');
echo "logout: ", result_code(epp_read($sock)), "\n";
fclose($sock);
```

Expected output:

```
greeting: ok
login: 1000
check: 1000 avail=1
logout: 1500
```

### DENIC RRI

```php
<?php
// Minimal DENIC RRI client for the riftcell/epp-mock container (RRI mock entrypoint).
$host = getenv('RRI_HOST') ?: '127.0.0.1';
$port = getenv('RRI_PORT') ?: '7701';

// Plain TCP, no TLS.
$sock = stream_socket_client("tcp://$host:$port", $errno, $errstr, 10);
if ($sock === false) {
    fwrite(STDERR, "connect failed: $errstr\n");
    exit(1);
}

// RRI frame: 4-byte big-endian length of the BODY ONLY (unlike EPP), then "key: value" lines.
function rri_write($sock, string $body): void {
    fwrite($sock, pack('N', strlen($body)) . $body);
}

function read_exact($sock, int $n): string {
    $buf = '';
    while (strlen($buf) < $n) {
        $chunk = fread($sock, $n - strlen($buf));
        if ($chunk === false || $chunk === '') {
            throw new RuntimeException('connection closed');
        }
        $buf .= $chunk;
    }
    return $buf;
}

function rri_read($sock): array {
    $len = unpack('N', read_exact($sock, 4))[1];
    $fields = [];
    foreach (explode("\n", read_exact($sock, $len)) as $line) {
        if (str_contains($line, ':')) {
            [$k, $v] = explode(':', $line, 2);
            $fields[strtolower(trim($k))] = trim($v);
        }
    }
    return $fields;
}

// Real DENIC expects the password as a lowercase MD5 hex digest; the mock accepts anything.
rri_write($sock, "Version: 5.0\nAction: LOGIN\nUser: any-user\nPassword: " . md5('any-password') . "\n");
$r = rri_read($sock);
echo "login: {$r['result']} (STID {$r['stid']})\n";

rri_write($sock, "Version: 5.0\nAction: INFO\nDomain: example.de\n");
$r = rri_read($sock);
echo "info: {$r['result']}\n";

rri_write($sock, "Version: 5.0\nAction: LOGOUT\n");
echo "logout: ", rri_read($sock)['result'], "\n";
fclose($sock);
```

Expected output:

```
login: success (STID 1)
info: success
logout: success
```

## What the Mock Returns

The standalone mock is stateless and always succeeds unless a fault flag is set. Write your tests with these responses in mind:

| Command | EPP response | RRI response |
|---|---|---|
| Login | `1000` for any credentials | `RESULT: success` for any credentials |
| `<hello>` | `<greeting>` | not applicable |
| `domain:check` | `1000`; every name `avail="1"` | not applicable |
| Other `domain:*`, `contact:*`, `host:*` (create, info, update, renew, transfer, delete) | `1000` with **no `<resData>`** | `RESULT: success` with no other fields |
| `<poll>` | `2000 Unknown command` (not supported) | not applicable |
| Anything else | `2000 Unknown command` | `RESULT: success` |
| Logout | `1500`, then the server closes | `RESULT: success`, then the server closes |

Other behaviour to know:
- Every EPP response echoes your `<clTRID>` and adds a `<svTRID>` of the form `mock-N`. RRI responses include an incrementing `STID`.
- Commands are accepted before login: the mock doesn't enforce session state.
- Because info commands return no `<resData>`, the mock can't test how your client parses `domain:infData` and similar data, and it can't test polling. Test those against a registrar's OT&E environment (see [Integration Testing](how-to-integration-testing.md#tier-3-real-ote-integration)).
