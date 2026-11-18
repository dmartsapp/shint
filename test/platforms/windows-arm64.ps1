# shint platform test for windows/arm64 - made by test/platforms/platforms.py from template.ps1:
# edit the template, not this file, then run `make platform-tests`.
#
# Run it on a windows/arm64 machine with that release's binary, in PowerShell:
#   powershell -ExecutionPolicy Bypass -File .\windows-arm64.ps1 .\shint.windows.arm64.exe
# It writes results/windows-arm64.txt next to itself. Send that file back:
# https://github.com/dmartsapp/shint/issues/new
#
# What it does: starts shint's own TCP, UDP and HTTP listeners on 127.0.0.1, checks
# them with telnet, nmap, udp, wol and web, runs the commands that read this machine
# (ip, ip route, ip arp, ss, dns and rdns, cidr, ping 127.0.0.1), and - when
# example.com answers - a few checks over the internet (dns, web, tls, ntp). About a
# minute. It changes nothing on the machine and needs no administrator rights; run it
# as a normal user.
#
# The results file holds which release was tested (its version and the binary's
# SHA-256), what the machine is (Windows edition and version - never its name or
# addresses), how many checks passed, and for each failure its exit status and the
# error lines shint printed - never a table of sockets, routes or neighbours.

$Platform = 'windows/arm64'
$Slug = 'windows-arm64'
$Asset = 'shint.windows.arm64.exe'
$ExpectSS = 'yes'      # ss: yes = works; no = says "not supported yet"; absent = not in this binary
$ExpectRoute = 'yes'   # ip route and ip arp: yes = work; no = say "not supported yet"
$ExpectPing = 'yes'    # ping 127.0.0.1: yes = must answer

$ErrorActionPreference = 'Continue'

$here = Split-Path -Parent $PSCommandPath
$resultsDir = Join-Path $here 'results'
$out = Join-Path $resultsDir "$Slug.txt"
$scriptName = Split-Path -Leaf $PSCommandPath

# The binary: the argument, or the release's file name here, or .\shint.exe, or shint.exe on PATH.
$bin = $null
if ($args.Count -gt 0) { $bin = $args[0] }
if (-not $bin) {
    foreach ($c in @((Join-Path (Get-Location) $Asset), (Join-Path $here $Asset), (Join-Path (Get-Location) 'shint.exe'))) {
        if (Test-Path -LiteralPath $c -PathType Leaf) { $bin = $c; break }
    }
}
if (-not $bin) {
    $cmd = Get-Command shint.exe -ErrorAction SilentlyContinue
    if ($cmd) { $bin = $cmd.Source }
}
if (-not $bin -or -not (Test-Path -LiteralPath $bin -PathType Leaf)) {
    Write-Host "usage: powershell -ExecutionPolicy Bypass -File $scriptName path\to\$Asset"
    Write-Host "  (the $Platform binary from https://github.com/dmartsapp/shint/releases)"
    exit 2
}
$bin = (Resolve-Path -LiteralPath $bin).Path

# Which release, and for which platform: "v4.4.0/<commit>/<built> (go1.27.4 windows/amd64)",
# or "4.4.0 (go1.27.4 windows/amd64)" for a build made without the release's stamp.
$version = ((& $bin --version 2>&1) | Out-String).Trim()
$binplat = ''
$goversion = ''
if ($version -match '\((go\S+) ([^)]+)\)') { $goversion = $Matches[1]; $binplat = $Matches[2] }
$stamp = ($version -split ' \(')[0]
$parts = $stamp -split '/', 3
$release = $parts[0]
$commit = ''
$built = ''
if ($parts.Count -ge 3) { $commit = $parts[1]; $built = $parts[2] }
if ($binplat -ne $Platform) {
    Write-Host "$bin is not a $Platform build of shint (--version says: $version)."
    Write-Host "Use the script named after its platform, or the $Asset binary."
    exit 2
}

New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null
$work = Join-Path $resultsDir ".work.$PID"
New-Item -ItemType Directory -Force -Path $work | Out-Null

$script:passed = 0
$script:failed = 0
$script:skipped = 0
$script:lines = New-Object System.Collections.Generic.List[string]
$script:listeners = @()
$script:code = 0
$script:text = ''

# Reads a file another process may still be writing.
function Read-Text([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return '' }
    $fs = [IO.File]::Open($Path, 'Open', 'Read', 'ReadWrite')
    try { (New-Object IO.StreamReader($fs)).ReadToEnd() } finally { $fs.Dispose() }
}

function Record([string]$Status, [string]$Name, [string]$Detail) {
    $l = '{0,-4}  {1}' -f $Status, $Name
    Write-Host $l
    $script:lines.Add($l)
    if ($Detail) {
        foreach ($d in ($Detail -split "`r?`n")) {
            if ($d.Trim()) { $x = "      $d"; Write-Host $x; $script:lines.Add($x) }
        }
    }
    switch ($Status) {
        'PASS' { $script:passed++ }
        'FAIL' { $script:failed++ }
        'SKIP' { $script:skipped++ }
    }
}

function Invoke-Shint([string[]]$A) {
    $o = Join-Path $work 'stdout'
    $e = Join-Path $work 'stderr'
    $p = Start-Process -FilePath $bin -ArgumentList $A -RedirectStandardOutput $o -RedirectStandardError $e -NoNewWindow -Wait -PassThru
    $script:code = $p.ExitCode
    $script:text = (Read-Text $o) + (Read-Text $e)
}

# A failure's detail: shint's error lines only (never the tables), without the date.
function Get-Errors {
    $found = $script:text -split "`r?`n" | Where-Object { $_ -match 'error|panic|fatal|denied|not permitted' } | Select-Object -First 5
    ($found | ForEach-Object {
        $s = $_ -replace '^[A-Z][a-z]{2} [A-Z][a-z]{2} [ 0-9]*[0-9] [0-9:]+ \S+ [0-9]+: ', ''
        if ($s.Length -gt 300) { $s.Substring(0, 300) } else { $s }
    }) -join "`n"
}

function Check([string]$Name, [int]$Want, [string]$Pattern, [string[]]$A) {
    Invoke-Shint $A
    if ($script:code -eq $Want -and $script:text -cmatch $Pattern) {
        Record 'PASS' $Name
    } else {
        Record 'FAIL' $Name ("exit=$($script:code) (wanted $Want), wanted a line matching: $Pattern`n" + (Get-Errors))
    }
}

# Like Check, but a failure is a skip: for what depends on the network or the
# machine's setup rather than on shint (a firewall blocking NTP, say).
function Soft([string]$Name, [string]$Reason, [int]$Want, [string]$Pattern, [string[]]$A) {
    Invoke-Shint $A
    if ($script:code -eq $Want -and $script:text -cmatch $Pattern) {
        Record 'PASS' $Name
    } else {
        Record 'SKIP' $Name ("$Reason (exit=$($script:code))`n" + (Get-Errors))
    }
}

function Start-Listener([string]$Name, [string]$Log, [string[]]$A) {
    $p = Start-Process -FilePath $bin -ArgumentList $A -RedirectStandardOutput $Log -RedirectStandardError "$Log.err" -NoNewWindow -PassThru
    $script:listeners += $p
    for ($i = 0; $i -lt 10; $i++) {
        if (((Read-Text $Log) + (Read-Text "$Log.err")) -match 'listening address=') { Record 'PASS' $Name; return $p }
        if ($p.HasExited) { break }
        Start-Sleep -Seconds 1
    }
    $script:text = (Read-Text $Log) + (Read-Text "$Log.err")
    Record 'FAIL' $Name ("it did not start listening`n" + (Get-Errors))
    return $p
}

try {
    Write-Host "shint $release on $Platform - results go to $out"
    Write-Host ''

    # --- the binary itself ---
    Record 'PASS' "version: --version names $Platform ($release)"
    Check 'help: lists the commands' 0 'Available Commands' @('--help')

    # --- listeners on 127.0.0.1, and the checks against them ---
    $base = 20000 + ($PID % 20000)
    $tcpPort = $base
    $udpPort = $base + 1
    $httpPort = $base + 2
    $wolPort = $base + 3
    $closedPort = $base + 4

    $tcp = Start-Listener 'listen tcp: starts on 127.0.0.1' (Join-Path $work 'listen-tcp') @('listen', 'tcp', "$tcpPort", '--bind', '127.0.0.1', '--timeout', '0')
    $null = Start-Listener 'listen udp: starts on 127.0.0.1, echoing' (Join-Path $work 'listen-udp') @('listen', 'udp', "$udpPort", '--bind', '127.0.0.1', '--echo')
    $null = Start-Listener 'listen http: starts on 127.0.0.1' (Join-Path $work 'listen-http') @('listen', 'http', "$httpPort", '--bind', '127.0.0.1')
    $null = Start-Listener 'listen udp: a second one, for wol' (Join-Path $work 'listen-wol') @('listen', 'udp', "$wolPort", '--bind', '127.0.0.1')
    $tcpPid = $tcp.Id

    Check 'telnet: connects to the local TCP listener' 0 "connect ok host=127\.0\.0\.1 port=$tcpPort" @('telnet', '127.0.0.1', "$tcpPort")
    Check 'telnet: a closed port is a failed check (exit 1)' 1 'ERROR' @('telnet', '127.0.0.1', "$closedPort")
    Check 'telnet --interface 127.0.0.1: sends from that address' 0 'source interface=127\.0\.0\.1' @('telnet', '127.0.0.1', "$tcpPort", '--interface', '127.0.0.1')
    Check '--interface: an unknown name fails and lists the real ones' 1 'no such interface nosuch0 \(available: ' @('telnet', '127.0.0.1', "$tcpPort", '--interface', 'nosuch0')
    Check 'nmap -p: one open port and one closed' 0 'scan complete ports_scanned=2 open=1' @('nmap', '127.0.0.1', '-p', "$tcpPort,$closedPort")
    Check 'nmap --json: the open port in stats' 0 "`"port`": $tcpPort\b" @('nmap', '127.0.0.1', '-p', "$tcpPort", '--json')
    Check 'udp: the echoing listener answers' 0 "probe open host=127\.0\.0\.1 port=$udpPort" @('udp', '127.0.0.1', "$udpPort", '--data', 'shint')
    Check 'wol: sends a magic packet' 0 'magic packet sent' @('wol', '02:00:00:00:00:01', '--broadcast', '127.0.0.1', '--port', "$wolPort")
    Start-Sleep -Seconds 1
    $wolLog = (Read-Text (Join-Path $work 'listen-wol')) + (Read-Text (Join-Path $work 'listen-wol.err'))
    if ($wolLog -match 'packet received .*bytes=102') {
        Record 'PASS' 'wol: the listener received the 102-byte packet'
    } else {
        $script:text = $wolLog
        Record 'FAIL' 'wol: the listener received the 102-byte packet' (Get-Errors)
    }
    Check 'web: GET from the local HTTP listener' 0 'status=200' @('web', "http://127.0.0.1:$httpPort/")
    Check 'web --json: status_code 200' 0 '"status_code": 200' @('web', "http://127.0.0.1:$httpPort/", '--json')
    Check "web --headers-only: the response's headers" 0 'header name=Content-Type' @('web', "http://127.0.0.1:$httpPort/", '--headers-only')

    # --- this machine ---
    Check 'ip: lists the interfaces, loopback included' 0 '127\.0\.0\.1' @('ip')
    Check 'cidr: a /24 has 254 hosts' 0 'hosts=254' @('cidr', '192.168.1.0/24')
    # Windows' hosts file usually leaves localhost out (the system answers it itself),
    # so these two depend on the machine's setup.
    Soft 'dns: localhost from the hosts file' 'localhost is not in this machine''s hosts file' 0 'data=127\.0\.0\.1' @('dns', 'localhost', 'A')
    Soft 'rdns: 127.0.0.1 has a name' '127.0.0.1 has no name in the hosts file or DNS here' 0 'names=\[' @('rdns', '127.0.0.1')

    if ($ExpectRoute -eq 'yes') {
        Check 'ip route: lists the routing table' 0 'OK route destination=' @('ip', 'route')
        Check 'ip route 127.0.0.1: goes out through loopback' 0 'source=127\.0\.0\.1' @('ip', 'route', '127.0.0.1')
        Check 'ip arp: reads the neighbour table' 0 'OK done' @('ip', 'arp')
    } else {
        Check 'ip route: says it is not supported yet (exit 1)' 1 'not supported' @('ip', 'route')
        Check 'ip arp: says it is not supported yet (exit 1)' 1 'not supported' @('ip', 'arp')
    }

    switch ($ExpectSS) {
        'yes' {
            Check 'ss -l: finds the TCP listener and its process' 0 "pid=$tcpPid\b" @('ss', '-l', '--ports', "$tcpPort")
            Check "ss --json: the listener's process" 0 "`"pid`": $tcpPid\b" @('ss', '-l', '--ports', "$tcpPort", '--json')
            Check 'ss: nothing matching is a failed check (exit 1)' 1 '.' @('ss', '--ports', "$closedPort")
        }
        'no' { Check 'ss: says it is not supported yet (exit 1)' 1 'not supported' @('ss', '-l') }
        'absent' { Check 'ss: not in this binary (unknown command, exit 2)' 2 'unknown command' @('ss', '-l') }
    }

    Check 'ping: 127.0.0.1 answers' 0 'received reply' @('ping', '127.0.0.1')

    # --- over the internet, when there is one ---
    Invoke-Shint @('telnet', 'example.com', '443', '-4')
    if ($script:code -eq 0) {
        $internet = 'yes'
        Record 'PASS' 'telnet: example.com 443 over the internet'
        Check "dns: example.com through the system's name server" 0 'type=A .*answers=[1-9]' @('dns', 'example.com', 'A')
        Check 'web: https://example.com, its certificate trusted' 0 'status=200' @('web', 'https://example.com', '-4')
        Check "tls: example.com's chain verified" 0 'OK verified server=example\.com' @('tls', 'example.com', '-4')
        Soft 'ntp: pool.ntp.org answers' 'no NTP answer - UDP port 123 may be blocked here' 0 'OK response server=pool\.ntp\.org' @('ntp', 'pool.ntp.org', '-4')
    } else {
        $internet = 'no'
        Record 'SKIP' 'internet checks' 'example.com:443 did not answer, so dns, web, tls and ntp over the internet were not run'
    }
}
finally {
    foreach ($p in $script:listeners) { if ($p -and -not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue } }
    Start-Sleep -Milliseconds 300
}

# --- the results file ---
if ($script:failed -eq 0) { $result = 'pass' } else { $result = 'fail' }
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if ($isAdmin) { $privileges = 'administrator' } else { $privileges = 'normal user' }
$osInfo = [Environment]::OSVersion.VersionString
$cim = Get-CimInstance Win32_OperatingSystem -ErrorAction SilentlyContinue
if ($cim) { $osInfo = "$($cim.Caption) $($cim.Version)" }
$sha = (Get-FileHash -Algorithm SHA256 -LiteralPath $bin).Hash.ToLower()

$content = @(
    "# shint platform test: $Platform",
    "# Written by test/platforms/$scriptName; run it again to replace this file.",
    "platform: $Platform",
    "release: $release",
    "commit: $commit",
    "built: $built",
    "go: $goversion",
    "binary: $(Split-Path -Leaf $bin)",
    "binary_sha256: $sha",
    "tested_at: $((Get-Date).ToUniversalTime().ToString("yyyy-MM-dd'T'HH:mm:ss'Z'"))",
    "os: $osInfo",
    "privileges: $privileges",
    "internet: $internet",
    "passed: $($script:passed)",
    "failed: $($script:failed)",
    "skipped: $($script:skipped)",
    "result: $result",
    ''
) + $script:lines
[IO.File]::WriteAllText($out, (($content -join "`n") + "`n"))
Remove-Item -Recurse -Force -LiteralPath $work -ErrorAction SilentlyContinue

Write-Host ''
Write-Host "$($script:passed) passed, $($script:failed) failed, $($script:skipped) skipped - $out"
Write-Host 'Please send that file: https://github.com/dmartsapp/shint/issues/new'
if ($script:failed -eq 0) { exit 0 } else { exit 1 }
