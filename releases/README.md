# Releases

One file per release, from v4.3.0 on: `vX.Y.Z.md`.

While a release is being built, its file is the **working page** of the `release/vX.Y.Z` branch - what the release is meant to deliver, the bugs it fixes, what changed - and it is kept up to date as the branch moves. When the release ships, the file merges into `main` with it and stays here as its record.

Two lines near the top are read by `make release`:

| Line | What it is for |
|---|---|
| `Summary:` | One plain-language sentence about the release. It becomes the release's row in the roadmap of [main's README](../readme.md), written by the release commit. Required. |
| `Includes:` | Releases folded into this one, if any (for example `v4.4.0`): their roadmap rows become "Shipped in vX.Y.Z" and their milestones are closed with this one. Optional. |

`make release-start VERSION=X.Y.Z` creates the branch and its file from a template. Releases before v4.3.0 kept their page as `readme.md` on their own `release/vX.Y.Z` branch, which is still there.

How a release is made: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
