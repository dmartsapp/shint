### CI failure on a release tag

A check that runs on every `v*.*.*` release tag failed. The issue title names the workflow: **Lint** or **Vulnerability Check**.

Find the failed run under **Actions** - https://github.com/dmartsapp/shint/actions - it is the run for the tag that was just pushed - and read the log of the failing step.

A failed check does not publish anything: the binary build and both Docker workflows run the same lint and vulnerability gate first and stop if it fails.
