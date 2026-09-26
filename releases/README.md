# Releases

One file per release, from v4.2.2 on: `vX.Y.Z.md` - the release's **record**.

While a release is being built, the same page is **`branch_readme.md`** at the root of its `release/vX.Y.Z` branch: what the release is meant to deliver, the bugs it fixes, what changed, and a **To do** checklist, kept up to date as the branch moves. Main's README roadmap row for the release links to it all along. When the release ships, its release commit moves the page here, and the roadmap row links here instead.

Three things in the page are read by `make release`:

| | What it is for |
|---|---|
| `Summary:` | One plain-language sentence about the release. It becomes the release's row in the roadmap of [main's README](../readme.md), written by the release commit. Required. |
| `Includes:` | Releases folded into this one, if any (for example `v4.4.0`): their roadmap rows become "Shipped in vX.Y.Z" and their milestones are closed with this one. Optional. |
| `## To do` | A checklist (`- [ ]` / `- [x]`). `make release` refuses to publish while an item is unchecked. |

`make release-start VERSION=X.Y.Z` creates the branch and its `branch_readme.md` from a template. Releases up to v4.2.2 kept their page as `readme.md` on their own `release/vX.Y.Z` branch, which is still there; v4.2.2's record was also added here after its release.

How a release is made: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html#making-a-release).
