"""State shared by the battery's modules, filled in by run.py before the cases load."""
PORTS = {}   # server name -> port, from servers.start()
VARS = {}    # other {placeholders} the case arguments may use (cert paths, version)
BIN = None   # the shint binary under test
FEATURES = {}  # what this machine can do: ipv6, tls, icmp
