#!/usr/bin/env node
'use strict';
const { spawnSync } = require('child_process');
const path = require('path');
const os = require('os');

const ext = os.platform() === 'win32' ? '.exe' : '';
const binary = path.join(__dirname, `swarm${ext}`);

const result = spawnSync(binary, process.argv.slice(2), { stdio: 'inherit' });
if (result.error) {
  const msg = result.error.code === 'ENOENT'
    ? 'binary not found. Try reinstalling: npm install -g @justestif/openswarm'
    : result.error.message;
  process.stderr.write(`openswarm: ${msg}\n`);
  process.exit(1);
}
process.exit(result.status ?? 1);
