# @illusionfield/mtest

Prebuilt binary wrapper for the Go-based `mtest` CLI so Node.js-centric workflows can install it via `npm`.

## Install

```bash
npm install --save-dev @illusionfield/mtest
```

or globally:

```bash
npm install --global @illusionfield/mtest
```

The postinstall script queries the latest GitHub release for `illusionfield/mtest`, inspects the published assets, and downloads the one that best matches your OS/arch (e.g. `mtest-{tag}-{GOOS}-{GOARCH}.zip`). To pin a specific release tag, run `npm install --save-dev @illusionfield/mtest --mtest-version=v1.2.3` or export `MTEST_DIST_VERSION=v1.2.3` before installation.

Additional controls:

- `MTEST_DIST_BASE_URL` — override the artefact host (supports `{tag}` / `{version}` placeholders).
- `MTEST_DIST_ASSET_PREFIX` — customise the asset basename before extensions.
- `MTEST_DIST_ASSET` — fetch a single explicit filename.
- `MTEST_DIST_REPO` — target a different GitHub repository for releases.
- `MTEST_DIST_GITHUB_TOKEN` — supply a token if GitHub API rate-limits are an issue.
- `MTEST_SKIP_BINARY_INSTALL` — skip downloads during local development or dry runs.

## Usage

```bash
npx mtest --package <package-path-or-name> --once
```

Every other CLI argument is forwarded unchanged to the underlying Go executable.

## License

MIT © The mtest Contributors.
