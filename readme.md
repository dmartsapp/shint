## Custom telnet client that connects to given URLs/hostnames to their ports.

>[!NOTE]
> The purpose of this client is to solve specific problem of:
> - TCP port alive checks for all URLs or addresses given
> - Download http/https URLs and report site response (no authentication)
> - DNS resolution report for all URLs or addresses given

>[!CAUTION]
> - This is not a replacement for actual telnet client that comes with many operating systems.
> - This is a hobby project that solves specific problem I encountered during my days as a sysadmin.

### Example:
'''bash
    $ telnet google.com 443 # Attempts to connect to google.com on TCP port 443. For every address returned by DNS resolution, telnet will attempt to connect to it.
    $ telnet 172.16.17.32 80 # Attempts to connect to 172.16.17.32 on TCP port 80.
    $ telnet --count 20 google.com 443 # Attempts to connect to google.com on TCP port 443 20 times concurrently
'''

[Latest Release](https://github.com/farhansabbir/telnet/releases/latest)
