#!/usr/bin/env node

/** @file CLI entrypoint that proxies to the platform-specific mtest runtime binary. */

import { spawn } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { existsSync, chmodSync } from 'node:fs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const isWindows = process.platform === 'win32';
const binaryName = isWindows ? 'mtest.exe' : 'mtest';
const binaryPath = join(__dirname, 'runtime', binaryName);

if (!existsSync(binaryPath)) {
  const relPath = join('bin', 'runtime', binaryName);
  console.error(
    `[mtest] Missing compiled binary at ${relPath}. ` +
      'Run `npm rebuild @illusionfield/mtest` or reinstall the package to fetch the correct artefact.'
  );
  process.exit(1);
}

if (!isWindows) {
  try {
    chmodSync(binaryPath, 0o755);
  } catch (error) {
    console.warn(`[mtest] Failed to set executable bit on ${binaryName}: ${error.message}`);
  }
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  env: process.env,
  windowsHide: false
});

/**
 * Relay termination signals received by the wrapper process to the spawned child.
 * @param {NodeJS.Signals} signal - POSIX signal captured on the parent process.
 */
const forwardSignal = signal => {
  if (!child.killed) {
    child.kill(signal);
  }
};

// Forward termination signals so the runtime can shut down gracefully.
for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
  process.on(signal, forwardSignal);
}

child.on('error', error => {
  console.error(`[mtest] Failed to start binary: ${error.message}`);
  process.exit(error.code === 'ENOENT' ? 127 : 1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
