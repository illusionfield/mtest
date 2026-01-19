#!/usr/bin/env node

/** @file Postinstall script that downloads the correct prebuilt mtest binary for the host platform. */

import { promises as fs } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import os from 'node:os';
import crypto from 'node:crypto';
import { extract } from 'tar'
import extractZip from 'extract-zip';

const fetch = await resolveFetch();

/**
 * Resolve a fetch implementation that works on the current Node.js runtime.
 * Prefers the built-in global fetch when present (Node.js >= 18),
 * otherwise falls back to node-fetch for older runtimes.
 * @returns {Promise<typeof globalThis.fetch>}
 */
async function resolveFetch() {
  if (typeof globalThis.fetch === 'function') {
    return globalThis.fetch.bind(globalThis);
  }
  let nodeFetchModule;
  try {
    nodeFetchModule = await import('node-fetch');
  } catch (error) {
    const message =
      'Global fetch is unavailable and the optional dependency `node-fetch` could not be loaded.';
    throw new Error(`${message} ${error?.message ?? error}`);
  }
  const nodeFetch = nodeFetchModule.default ?? nodeFetchModule;

  // Ensure WHATWG globals exist for downstream consumers of the Response object.
  if (!globalThis.Headers && nodeFetchModule.Headers) {
    globalThis.Headers = nodeFetchModule.Headers;
  }
  if (!globalThis.Request && nodeFetchModule.Request) {
    globalThis.Request = nodeFetchModule.Request;
  }
  if (!globalThis.Response && nodeFetchModule.Response) {
    globalThis.Response = nodeFetchModule.Response;
  }

  return nodeFetch;
}

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, '..');
const runtimeDir = join(projectRoot, 'bin', 'runtime');
const packageJson = JSON.parse(
  await fs.readFile(new URL('../package.json', import.meta.url), 'utf8')
);

/**
 * @typedef {Object} PlatformTarget
 * @property {string} goos - GOOS identifier used for release assets.
 * @property {string} goarch - GOARCH identifier used for release assets.
 * @property {string} binaryName - Expected filename for the runtime binary on this platform.
 */

/**
 * @typedef {Object} AssetCandidate
 * @property {string} name - Candidate filename to download when a URL is not provided.
 * @property {'tar'|'zip'|'binary'} unpack - Strategy used to unpack the candidate.
 * @property {string} [url] - Fully qualified URL that overrides name-based resolution.
 */

/**
 * @typedef {Object} ReleaseContext
 * @property {string} tag - Git tag selected for the download.
 * @property {string} source - Description of how the tag was chosen (explicit, latest, fallback).
 * @property {Array<Object>} assets - Asset metadata returned by GitHub for the chosen release.
 */

// Allow optional skip for environments that manage binaries manually.
if (process.env.MTEST_SKIP_BINARY_INSTALL === 'true') {
  console.log('[mtest] Skipping binary download (MTEST_SKIP_BINARY_INSTALL=true).');
  process.exit(0);
}

/**
 * Mapping between Node.js platform/architecture pairs and release metadata.
 * @type {Record<string, Record<string, PlatformTarget>>}
 */
const platformMatrix = {
  darwin: {
    arm64: { goos: 'darwin', goarch: 'arm64', binaryName: 'mtest' },
    x64: { goos: 'darwin', goarch: 'amd64', binaryName: 'mtest' }
  },
  linux: {
    arm64: { goos: 'linux', goarch: 'arm64', binaryName: 'mtest' },
    x64: { goos: 'linux', goarch: 'amd64', binaryName: 'mtest' }
  },
  win32: {
    arm64: { goos: 'windows', goarch: 'arm64', binaryName: 'mtest.exe' },
    ia32: { goos: 'windows', goarch: '386', binaryName: 'mtest.exe' },
    x64: { goos: 'windows', goarch: 'amd64', binaryName: 'mtest.exe' }
  }
};

/** @type {Record<string, PlatformTarget> | undefined} */
const platformTargets = platformMatrix[process.platform];
if (!platformTargets) {
  console.error(`[mtest] Unsupported platform: ${process.platform}.`);
  process.exit(1);
}

/** @type {PlatformTarget | undefined} */
const target = platformTargets[process.arch];
if (!target) {
  console.error(`[mtest] Unsupported architecture: ${process.platform}/${process.arch}.`);
  process.exit(1);
}

await fs.mkdir(runtimeDir, { recursive: true });

const destinationPath = join(runtimeDir, target.binaryName);
const repository =
  process.env.MTEST_DIST_REPO ??
  inferRepository(packageJson.repository) ??
  'illusionfield/mtest';
const explicitVersion =
  process.env.MTEST_DIST_VERSION ??
  process.env.MTEST_DIST_TAG ??
  process.env.npm_config_mtest_version;
const fallbackTag = normalizeTag(packageJson.version);
const baseUrlOverride = process.env.MTEST_DIST_BASE_URL;

/** @type {ReleaseContext} */
const releaseContext = await resolveReleaseContext({
  explicitVersion,
  fallbackTag,
  repository,
  baseUrlOverride
});

const { tag: releaseTag, source: tagSource, assets: releaseAssets } = releaseContext;
console.log(`[mtest] Using release tag ${releaseTag} (${tagSource}).`);

const templateValues = {
  tag: releaseTag,
  version: stripLeadingV(releaseTag)
};

const baseUrl = ensureTrailingSlash(
  applyTemplate(
    baseUrlOverride ?? `https://github.com/${repository}/releases/download/{tag}`,
    templateValues
  )
);

const explicitAsset = applyTemplate(process.env.MTEST_DIST_ASSET, templateValues);
const assetPrefixOverride = applyTemplate(process.env.MTEST_DIST_ASSET_PREFIX, templateValues);

const assetCandidates = buildAssetCandidates({
  target,
  explicitAsset,
  assetPrefixOverride,
  templateValues,
  releaseAssets,
  baseUrlOverride
});

let lastError;

// Attempt each candidate sequentially until one successfully downloads and installs.
for (const candidate of assetCandidates) {
  const url = candidate.url ?? new URL(candidate.name, baseUrl).toString();
  try {
    console.log(`[mtest] Fetching ${url}`);
    await acquireBinary({ url, destinationPath, candidate, target });
    if (process.platform !== 'win32') {
      await fs.chmod(destinationPath, 0o755);
    }
    console.log('[mtest] Binary ready:', destinationPath);
    process.exit(0);
  } catch (error) {
    lastError = error;
    console.warn(`[mtest] Failed to acquire ${candidate.name}: ${error.message}`);
  }
}

console.error('[mtest] Unable to download a matching binary for your platform.');
if (lastError) {
  console.error(`[mtest] Last error: ${lastError.stack ?? lastError.message}`);
}
console.error(
  '[mtest] If you have Go installed you can build from source with `go install ./cmd/mtest`.'
);
process.exit(1);

/**
 * Ensure directory-like strings include a trailing slash to ease URL composition.
 * @param {string} input - URL or path fragment that may need normalization.
 * @returns {string} Normalized string that always ends with a slash.
 */
function ensureTrailingSlash(input) {
  return input.endsWith('/') ? input : `${input}/`;
}

/**
 * Replace templated tokens in a string with provided values.
 * @param {string|undefined} value - Template string containing `{key}` placeholders.
 * @param {Record<string, string>} replacements - Key/value pairs for substitution.
 * @returns {string|undefined} Interpolated string or the original falsy value.
 */
function applyTemplate(value, replacements) {
  if (!value) {
    return value;
  }
  let result = value;
  for (const [key, replacement] of Object.entries(replacements)) {
    result = result.split(`{${key}}`).join(replacement);
  }
  return result;
}

/**
 * Generate ordered candidate assets for the current platform based on environment hints.
 * @param {Object} params - Inputs that influence asset resolution.
 * @param {PlatformTarget} params.target - Platform descriptor derived from Node.js runtime.
 * @param {string|undefined} params.explicitAsset - Fully qualified asset filename override.
 * @param {string|undefined} params.assetPrefixOverride - Prefix override used to construct default asset names.
 * @param {Record<string, string>} params.templateValues - Template values derived from the selected release tag.
 * @param {Array<Object>|undefined} params.releaseAssets - Assets returned by the release API.
 * @param {string|undefined} params.baseUrlOverride - Explicit base URL that bypasses GitHub asset lookup.
 * @returns {AssetCandidate[]} Ordered list of asset candidates to attempt.
 */
function buildAssetCandidates({
  target,
  explicitAsset,
  assetPrefixOverride,
  templateValues,
  releaseAssets,
  baseUrlOverride
}) {
  const candidates = [];
  const seen = new Set();

  const addCandidate = candidate => {
    const key = candidate.url ?? candidate.name;
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    candidates.push(candidate);
  };

  if (explicitAsset) {
    addCandidate({ name: explicitAsset, unpack: inferAssetKind(explicitAsset, target) });
    return candidates;
  }

  if (!baseUrlOverride && releaseAssets?.length) {
    const releaseCandidates = selectReleaseAssetCandidates(releaseAssets, target);
    for (const candidate of releaseCandidates) {
      addCandidate(candidate);
    }
  }

  const prefixes = new Set();
  if (assetPrefixOverride) {
    prefixes.add(assetPrefixOverride);
  } else {
    prefixes.add(
      applyTemplate(`mtest-{tag}-${target.goos}-${target.goarch}`, templateValues)
    );
    prefixes.add(
      applyTemplate(`mtest-{version}-${target.goos}-${target.goarch}`, templateValues)
    );
  }

  for (const prefix of prefixes) {
    addCandidateVariants({ addCandidate, prefix, target });
  }

  return candidates;
}

/**
 * Expand candidate name permutations for common archive extensions and binary suffixes.
 * @param {Object} options - Aggregation context shared with the caller.
 * @param {(candidate: AssetCandidate) => void} options.addCandidate - Collector callback used to register candidates.
 * @param {string} options.prefix - Base filename that should receive extension permutations.
 * @param {PlatformTarget} options.target - Platform metadata informing binary suffix handling.
 */
function addCandidateVariants({ addCandidate, prefix, target }) {
  const names = new Set([prefix]);

  if (target.binaryName.endsWith('.exe') && !prefix.endsWith('.exe')) {
    names.add(`${prefix}.exe`);
  }
  if (!prefix.endsWith('.tar.gz')) {
    names.add(`${prefix}.tar.gz`);
  }
  if (!prefix.endsWith('.tgz')) {
    names.add(`${prefix}.tgz`);
  }
  if (!prefix.endsWith('.zip')) {
    names.add(`${prefix}.zip`);
  }

  for (const name of names) {
    addCandidate({ name, unpack: inferAssetKind(name, target) });
  }
}

/**
 * Infer how a downloaded asset should be unpacked based on its filename.
 * @param {string} assetName - Candidate asset filename.
 * @param {PlatformTarget} targetInfo - Platform metadata providing expected binary name.
 * @returns {'tar'|'zip'|'binary'} Strategy to use for the acquired artifact.
 */
function inferAssetKind(assetName, targetInfo) {
  if (assetName.endsWith('.zip')) return 'zip';
  if (assetName.endsWith('.tar.gz') || assetName.endsWith('.tgz')) return 'tar';
  if (assetName.endsWith('.exe') || assetName.endsWith(targetInfo.binaryName)) return 'binary';
  return 'binary';
}

/**
 * Download, optionally unpack, and install the appropriate runtime binary.
 * @param {Object} options - Parameters describing the download target.
 * @param {string} options.url - Fully qualified URL to fetch.
 * @param {string} options.destinationPath - Filesystem location where the binary should be placed.
 * @param {AssetCandidate} options.candidate - Candidate metadata describing unpack strategy.
 * @param {PlatformTarget} options.target - Platform descriptor used when locating binaries inside archives.
 * @returns {Promise<void>} Promise that resolves when the binary is ready.
 */
async function acquireBinary({ url, destinationPath, candidate, target }) {
  const tmpBase = join(os.tmpdir(), `mtest-npm-${crypto.randomUUID()}`);
  await fs.mkdir(tmpBase, { recursive: true });

  try {
    if (candidate.unpack === 'binary') {
      await downloadToFile(url, destinationPath);
      return;
    }

    const archivePath = join(tmpBase, candidate.name.split('/').pop());
    await downloadToFile(url, archivePath);

    const extractDir = join(tmpBase, 'extract');
    await fs.mkdir(extractDir, { recursive: true });

    if (candidate.unpack === 'tar') {
      await extract({ file: archivePath, cwd: extractDir });
    } else if (candidate.unpack === 'zip') {
      await extractZip(archivePath, { dir: extractDir });
    } else {
      throw new Error(`Unknown unpack strategy: ${candidate.unpack}`);
    }

    const locatedBinary = await findBinary(extractDir, target.binaryName);
    if (!locatedBinary) {
      throw new Error('Binary not found inside archive.');
    }
    await fs.copyFile(locatedBinary, destinationPath);
  } finally {
    await safeRm(tmpBase);
  }
}

/**
 * Fetch content from a URL and persist it to disk.
 * @param {string} url - Remote resource to download.
 * @param {string} destination - Local path where the response body should be stored.
 * @returns {Promise<void>} Resolves once the file is written.
 */
async function downloadToFile(url, destination) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Request failed with status ${response.status} ${response.statusText}`);
  }
  const arrayBuffer = await response.arrayBuffer();
  await fs.writeFile(destination, Buffer.from(arrayBuffer));
}

/**
 * Recursively search for the expected binary within an extracted archive.
 * @param {string} rootDir - Directory that serves as the search root.
 * @param {string} expectedName - Preferred filename for the executable.
 * @returns {Promise<string|undefined>} Absolute path to the located binary, if present.
 */
async function findBinary(rootDir, expectedName) {
  const entries = await fs.readdir(rootDir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = join(rootDir, entry.name);
    if (entry.isDirectory()) {
      const nested = await findBinary(fullPath, expectedName);
      if (nested) return nested;
    } else if (entry.isFile()) {
      if (entry.name === expectedName) {
        return fullPath;
      }
      if (!process.platform.startsWith('win') && entry.name === 'mtest' && expectedName === 'mtest.exe') {
        // fallback if archive only contains non-suffixed binary
        return fullPath;
      }
    }
  }
  return undefined;
}

/**
 * Attempt to delete a directory tree, suppressing errors that are non-fatal.
 * @param {string} targetPath - Filesystem path scheduled for cleanup.
 * @returns {Promise<void>} Always resolves, even if removal fails.
 */
async function safeRm(targetPath) {
  try {
    await fs.rm(targetPath, { recursive: true, force: true });
  } catch {
    // noop
  }
}

/**
 * Determine which release tag and assets should be used for downloading the binary.
 * @param {Object} options - Inputs describing user overrides and fallback behavior.
 * @param {string|undefined} options.explicitVersion - Exact version requested via env or npm config.
 * @param {string|undefined} options.fallbackTag - Package version fallback when no override is provided.
 * @param {string|undefined} options.repository - GitHub repository slug used for API lookups.
 * @param {string|undefined} options.baseUrlOverride - Custom CDN root that bypasses release lookups.
 * @returns {Promise<ReleaseContext>} Selected release metadata and accompanying assets.
 */
async function resolveReleaseContext({
  explicitVersion,
  fallbackTag,
  repository,
  baseUrlOverride
}) {
  if (explicitVersion) {
    const normalized = normalizeTag(explicitVersion);
    const assets =
      !baseUrlOverride && repository
        ? await fetchReleaseAssetsSafely(repository, normalized)
        : [];
    return { tag: normalized, source: 'explicit', assets };
  }

  if (!baseUrlOverride && repository) {
    try {
      const latest = await fetchLatestRelease(repository);
      if (latest?.tag) {
        return { tag: latest.tag, source: 'github-latest', assets: latest.assets ?? [] };
      }
    } catch (error) {
      console.warn(`[mtest] Falling back to package version: ${error.message}`);
    }
  }

  if (!fallbackTag) {
    throw new Error('Unable to determine a release tag. Provide MTEST_DIST_VERSION or set MTEST_DIST_BASE_URL.');
  }

  const assets =
    !baseUrlOverride && repository
      ? await fetchReleaseAssetsSafely(repository, fallbackTag)
      : [];

  return { tag: fallbackTag, source: 'package.json', assets };
}

/**
 * Parse the package.json repository field into a GitHub repository slug.
 * @param {string|Object|undefined} repoField - Repository value from package metadata.
 * @returns {string|undefined} Repository owner/name string, if extractable.
 */
function inferRepository(repoField) {
  if (!repoField) {
    return undefined;
  }
  if (typeof repoField === 'string') {
    return extractRepository(repoField);
  }
  if (typeof repoField === 'object' && repoField.url) {
    return extractRepository(repoField.url);
  }
  return undefined;
}

/**
 * Extract the owner/name portion from a GitHub repository URL.
 * @param {string} url - Repository URL or slug.
 * @returns {string|undefined} Normalized owner/name representation if matched.
 */
function extractRepository(url) {
  const match = url.match(/github\.com[:/](.+?)(?:\.git)?$/i);
  return match ? match[1] : undefined;
}

/**
 * Ensure tags are prefixed with the canonical v marker expected by releases.
 * @param {string|undefined} input - User-provided version, possibly missing the prefix.
 * @returns {string|undefined} Tag beginning with `v`, or undefined if no input.
 */
function normalizeTag(input) {
  if (!input) {
    return undefined;
  }
  return input.startsWith('v') ? input : `v${input}`;
}

/**
 * Remove a leading v prefix from a semver string when present.
 * @param {string|undefined} input - Version string that may contain a leading v.
 * @returns {string} Clean version string suitable for display.
 */
function stripLeadingV(input) {
  if (!input) {
    return '';
  }
  return input.startsWith('v') ? input.slice(1) : input;
}

/**
 * Fetch release asset metadata for a specific tag, ignoring failures.
 * @param {string} repository - GitHub repository slug.
 * @param {string} tag - Release tag name to inspect.
 * @returns {Promise<Array<Object>>} Asset list or an empty array when unavailable.
 */
async function fetchReleaseAssetsSafely(repository, tag) {
  try {
    const release = await fetchReleaseByTag(repository, tag);
    return release.assets ?? [];
  } catch (error) {
    console.warn(`[mtest] Unable to fetch assets for ${tag}: ${error.message}`);
    return [];
  }
}

/**
 * Retrieve metadata for the latest GitHub release.
 * @param {string} repository - GitHub repository slug.
 * @returns {Promise<{tag: string, assets: Array<Object>}|undefined>} Latest release payload if available.
 */
async function fetchLatestRelease(repository) {
  const payload = await githubRequest(`/repos/${repository}/releases/latest`);
  return payload?.tag_name
    ? { tag: payload.tag_name, assets: payload.assets ?? [] }
    : undefined;
}

/**
 * Request release information for an explicit tag.
 * @param {string} repository - GitHub repository slug.
 * @param {string} tag - Target tag to fetch.
 * @returns {Promise<Object>} Parsed JSON response from the GitHub API.
 */
async function fetchReleaseByTag(repository, tag) {
  return githubRequest(`/repos/${repository}/releases/tags/${encodeURIComponent(tag)}`);
}

/**
 * Execute an authenticated GitHub API request.
 * @param {string} path - API path beginning with `/repos/...`.
 * @returns {Promise<any>} Parsed JSON response body.
 * @throws {Error} If the HTTP status indicates a failed request.
 */
async function githubRequest(path) {
  const apiUrl = `https://api.github.com${path}`;
  const headers = {
    Accept: 'application/vnd.github+json',
    'User-Agent': 'mtest-postinstall'
  };
  const token = process.env.MTEST_DIST_GITHUB_TOKEN ?? process.env.GITHUB_TOKEN;
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(apiUrl, { headers });
  if (!response.ok) {
    throw new Error(`GitHub API error ${response.status}: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Score and rank available release assets relative to the target platform.
 * @param {Array<Object>} assets - Asset entries returned by the GitHub release API.
 * @param {PlatformTarget} target - Current platform descriptor.
 * @returns {AssetCandidate[]} Sorted candidate list favoring the most suitable assets.
 */
function selectReleaseAssetCandidates(assets, target) {
  const scored = assets
    .map(asset => ({
      asset,
      score: scoreAssetForTarget(asset.name, target)
    }))
    .filter(entry => entry.score > 0)
    .sort((a, b) => b.score - a.score);

  return scored.map(entry => ({
    name: entry.asset.name,
    url: entry.asset.browser_download_url,
    unpack: inferAssetKind(entry.asset.name, target)
  }));
}

/**
 * Assign a heuristic score to a candidate asset based on naming conventions.
 * @param {string} name - Asset filename to inspect.
 * @param {PlatformTarget} target - Current platform descriptor.
 * @returns {number} Higher numbers indicate a better match.
 */
function scoreAssetForTarget(name, target) {
  const normalized = name.toLowerCase();
  let score = 0;

  const goosAliases = getGoosAliases(target.goos);
  const goarchAliases = getGoarchAliases(target.goarch);

  if (goosAliases.some(alias => normalized.includes(alias))) {
    score += 6;
  }

  if (goarchAliases.some(alias => normalized.includes(alias))) {
    score += 6;
  }

  const binaryStem = target.binaryName.replace(/\.exe$/i, '');
  if (normalized.includes(binaryStem)) {
    score += 2;
  }

  if (normalized.endsWith('.tar.gz') || normalized.endsWith('.tgz')) {
    score += 2;
  } else if (normalized.endsWith('.zip')) {
    score += 1;
  } else if (normalized.endsWith('.exe')) {
    score += 1;
  }

  if (normalized.includes('latest')) {
    score -= 2;
  }

  return score;
}

/**
 * Provide common synonyms for GOOS identifiers encountered in binary names.
 * @param {string} goos - Canonical GOOS value.
 * @returns {string[]} Array of lowercase aliases to search for.
 */
function getGoosAliases(goos) {
  const map = {
    darwin: ['darwin', 'macos', 'mac', 'osx'],
    linux: ['linux'],
    windows: ['windows', 'win32', 'win', 'win64']
  };
  return map[goos] ?? [goos];
}

/**
 * Provide common synonyms for GOARCH identifiers encountered in binary names.
 * @param {string} goarch - Canonical GOARCH value.
 * @returns {string[]} Array of lowercase aliases to search for.
 */
function getGoarchAliases(goarch) {
  const map = {
    amd64: ['amd64', 'x86_64', 'x64'],
    arm64: ['arm64', 'aarch64'],
    '386': ['386', 'x86', 'ia32']
  };
  return map[goarch] ?? [goarch];
}
