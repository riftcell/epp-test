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
