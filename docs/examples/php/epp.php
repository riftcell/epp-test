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
