# The README's demo. Record it from the repository root with:
#   vhs docs/assets/demo.tape
# It types the commands itself, so every recording is the same. Needs vhs
# (brew install vhs), jq, and the shint to show at bin/shint.

Output docs/assets/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 290
Set Padding 24
Set TypingSpeed 35ms

Hide
Type `export PS1='$ ' PATH="$PWD/bin:$PATH" && clear`
Enter
Show

# Can I connect?
Type "shint telnet example.com 443 -4"
Sleep 400ms
Enter
Sleep 3.5s
Hide
Type "clear"
Enter
Show

# What does it return, and where did the time go?
Type "shint web https://example.com -4 --timing"
Sleep 400ms
Enter
Sleep 4s
Hide
Type "clear"
Enter
Show

# What does DNS say?
Type "shint dns github.com MX @1.1.1.1"
Sleep 400ms
Enter
Sleep 3s
Hide
Type "clear"
Enter
Show

# When does the certificate expire? (--json, for scripts)
Type "shint tls example.com -4 --json | jq '.stats[0] | {tls_version, verified, expires_in_days}'"
Sleep 400ms
Enter
Sleep 4.5s
Hide
Type "clear"
Enter
Show

# What's open? (scanme.nmap.org is the Nmap project's host for light test scans)
Type "shint nmap scanme.nmap.org -4 --from 20 --to 100 --timeout 1"
Sleep 400ms
Enter
Sleep 5s
