#!/usr/bin/env node

/** @file Lightweight self-test for the npm wrapper package. */

import { access, readFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const rootDir = resolve(__dirname, '..');

const requiredFiles = [
  'bin/mtest.js',
  'scripts/postinstall.mjs',
  'package.json'
];

await Promise.all(
  requiredFiles.map(async relativePath => {
    const target = resolve(rootDir, relativePath);
    try {
      await access(target, constants.R_OK);
    } catch (error) {
      throw new Error(`Missing required file: ${relativePath}`);
    }
  })
);

const pkgPath = resolve(rootDir, 'package.json');
const pkg = JSON.parse(await readFile(pkgPath, 'utf8'));

if (!pkg.name || pkg.name !== '@illusionfield/mtest') {
  throw new Error('package.json name is missing or incorrect.');
}

if (!pkg.version || typeof pkg.version !== 'string') {
  throw new Error('package.json version is missing or invalid.');
}

if (!pkg.bin || pkg.bin.mtest !== 'bin/mtest.js') {
  throw new Error('package.json bin entry is missing or incorrect.');
}

const expectedFiles = ['bin', 'scripts', 'LICENSE', 'README.md'];
if (!Array.isArray(pkg.files)) {
  throw new Error('package.json files list is missing or invalid.');
}
for (const entry of expectedFiles) {
  if (!pkg.files.includes(entry)) {
    throw new Error(`package.json files list is missing: ${entry}`);
  }
}

const entryPath = resolve(rootDir, 'bin/mtest.js');
const entrySource = await readFile(entryPath, 'utf8');

if (!entrySource.startsWith('#!/usr/bin/env node')) {
  throw new Error('bin/mtest.js is missing the Node.js shebang.');
}

if (process.env.MTEST_SELFTEST_RUNTIME === 'true') {
  const runtimePath = resolve(rootDir, 'bin', 'runtime');
  try {
    await access(runtimePath, constants.R_OK);
  } catch (error) {
    throw new Error(
      'Runtime directory is missing. Run the postinstall script or install the package to download binaries.'
    );
  }
}

console.log('npm wrapper self-test passed.');
