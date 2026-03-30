#!/usr/bin/env node
'use strict';
const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const { execFileSync } = require('child_process');

const pkg = require('./package.json');
const version = pkg.version;
const REPO = 'justEstif/openswarm';

// Map Node.js platform/arch to GoReleaser artifact names.
const PLATFORM_MAP = {
  'linux-x64':   { os: 'linux',   arch: 'amd64', ext: 'tar.gz' },
  'linux-arm64': { os: 'linux',   arch: 'arm64', ext: 'tar.gz' },
  'darwin-x64':  { os: 'darwin',  arch: 'amd64', ext: 'tar.gz' },
  'darwin-arm64':{ os: 'darwin',  arch: 'arm64', ext: 'tar.gz' },
  'win32-x64':   { os: 'windows', arch: 'amd64', ext: 'zip'    },
  'win32-arm64': { os: 'windows', arch: 'arm64', ext: 'zip'    },
};

const key = `${process.platform}-${process.arch}`;
const target = PLATFORM_MAP[key];
if (!target) {
  console.error(`openswarm: unsupported platform ${key}`);
  console.error('Supported: linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64');
  process.exit(1);
}

const binDir = path.join(__dirname, 'bin');
const binName = target.os === 'windows' ? 'swarm.exe' : 'swarm';
const binPath = path.join(binDir, binName);

// Skip if binary already exists (supports offline re-installs / npm ci).
if (fs.existsSync(binPath)) {
  console.log('openswarm: binary already present, skipping download');
  process.exit(0);
}

if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });

const filename = `swarm_${version}_${target.os}_${target.arch}.${target.ext}`;
const url = `https://github.com/${REPO}/releases/download/v${version}/${filename}`;
const tmpPath = path.join(os.tmpdir(), filename);

console.log(`openswarm: downloading ${filename}`);

function download(url, dest, cb) {
  const file = fs.createWriteStream(dest);
  function get(u) {
    https.get(u, res => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        return get(res.headers.location);
      }
      if (res.statusCode !== 200) {
        file.close();
        fs.unlinkSync(dest);
        return cb(new Error(`HTTP ${res.statusCode} downloading ${u}`));
      }
      res.pipe(file);
      file.on('finish', () => file.close(cb));
    }).on('error', err => {
      file.close();
      try { fs.unlinkSync(dest); } catch (_) {}
      cb(err);
    });
  }
  get(url);
}

download(url, tmpPath, err => {
  if (err) {
    console.error('openswarm: download failed:', err.message);
    console.error('You can also install from source: https://github.com/justEstif/openswarm');
    process.exit(1);
  }

  try {
    if (target.ext === 'tar.gz') {
      // -C extracts to binDir; strip the swarm binary out of the archive
      execFileSync('tar', ['-xzf', tmpPath, '-C', binDir, '--strip-components=0', 'swarm']);
    } else {
      // Windows: modern tar works on zip too
      execFileSync('tar', ['-xf', tmpPath, '-C', binDir, 'swarm.exe']);
    }
    fs.unlinkSync(tmpPath);
    if (target.os !== 'windows') fs.chmodSync(binPath, 0o755);
    console.log('openswarm: installed successfully →', binPath);
  } catch (e) {
    console.error('openswarm: extraction failed:', e.message);
    process.exit(1);
  }
});
