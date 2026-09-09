#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const appDir = path.resolve(__dirname, '..');
const iconDir = path.join(appDir, 'assets', 'icon');
const webDir = path.join(appDir, 'web');
const webIconsDir = path.join(webDir, 'icons');

// 1. Locate Single Source of Truth
const ssotSvgPath = path.join(iconDir, 'shelfd_icon.svg');
if (!fs.existsSync(ssotSvgPath)) {
  console.error(`Error: Single source of truth not found at ${ssotSvgPath}`);
  process.exit(1);
}

console.log('=== Shelfd App Icon & Favicon Generator ===');
console.log(`Source of Truth: ${ssotSvgPath}`);

const ssotContent = fs.readFileSync(ssotSvgPath, 'utf8');

// Extract glyph path from SSOT
const glyphMatch = ssotContent.match(/<path[^>]*id="shelfd-book-glyph"[^>]*d="([^"]+)"/);
if (!glyphMatch) {
  console.error('Error: Could not find <path id="shelfd-book-glyph" ...> in SSOT SVG');
  process.exit(1);
}
const glyphD = glyphMatch[1];

// Locate Chrome / Chromium binary for headless SVG rasterization
function findChrome() {
  const candidates = [
    process.env.CHROME_BIN,
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Chromium.app/Contents/MacOS/Chromium',
    '/usr/bin/google-chrome',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
  ].filter(Boolean);

  for (const bin of candidates) {
    if (fs.existsSync(bin)) return bin;
  }
  throw new Error('Chrome/Chromium binary not found for headless rendering.');
}

const chromeBin = findChrome();
console.log(`Headless Renderer: ${chromeBin}`);

// Helper to render SVG to PNG via headless Chrome
function renderSvgToPng(svgPath, pngPath, size = 1024) {
  const fileUrl = `file://${path.resolve(svgPath)}`;
  execFileSync(chromeBin, [
    '--headless',
    `--screenshot=${pngPath}`,
    `--window-size=${size},${size}`,
    '--default-background-color=00000000',
    '--hide-scrollbars',
    fileUrl,
  ], { stdio: 'pipe' });

  if (!fs.existsSync(pngPath) || fs.statSync(pngPath).size === 0) {
    throw new Error(`Failed to render PNG to ${pngPath}`);
  }
  console.log(`  ✓ Rendered ${path.relative(appDir, pngPath)} (${size}x${size})`);
}

// 2. Generate Derived SVGs
// a) Full-bleed 1024x1024 (for iOS & standard launcher base)
const fullBleedSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <rect width="1024" height="1024" fill="#111111"/>
  <g transform="translate(224, 271) scale(0.6)">
    <path transform="translate(0, 960)" fill="#F9F9F8" d="${glyphD}"/>
  </g>
</svg>`;

// b) Squircle 1024x1024 (for macOS Dock, Web, Windows)
// macOS standard icon tile: 824x824 at (100,100), rx=185
const squircleSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <rect x="100" y="100" width="824" height="824" rx="185" fill="#111111"/>
  <g transform="translate(265, 305) scale(0.515)">
    <path transform="translate(0, 960)" fill="#F9F9F8" d="${glyphD}"/>
  </g>
</svg>`;

// c) Adaptive Foreground 1024x1024 (for Android adaptive icons - 66% safe zone)
const adaptiveFgSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <g transform="translate(301, 335) scale(0.44)">
    <path transform="translate(0, 960)" fill="#F9F9F8" d="${glyphD}"/>
  </g>
</svg>`;

// d) Web Favicon SVG (128x128)
const faviconSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <rect width="128" height="128" rx="28" fill="#111111"/>
  <g transform="translate(28, 34) scale(0.075)">
    <path transform="translate(0, 960)" fill="#F9F9F8" d="${glyphD}"/>
  </g>
</svg>`;

// e) Web App Icon SVG (512x512)
const webAppIconSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <rect width="512" height="512" rx="112" fill="#111111"/>
  <g transform="translate(112, 135) scale(0.3)">
    <path transform="translate(0, 960)" fill="#F9F9F8" d="${glyphD}"/>
  </g>
</svg>`;

console.log('\n[1/4] Writing Derived Vector Assets...');
const fullBleedSvgPath = path.join(iconDir, 'app_icon.svg');
const squircleSvgPath = path.join(iconDir, 'app_icon_squircle.svg');
const adaptiveFgSvgPath = path.join(iconDir, 'app_icon_foreground.svg');
const webFaviconSvgPath = path.join(webIconsDir, 'shelfd_favicon.svg');
const webAppIconSvgPath = path.join(webIconsDir, 'shelfd_app_icon.svg');

fs.writeFileSync(fullBleedSvgPath, fullBleedSvg);
fs.writeFileSync(squircleSvgPath, squircleSvg);
fs.writeFileSync(adaptiveFgSvgPath, adaptiveFgSvg);
fs.writeFileSync(webFaviconSvgPath, faviconSvg);
fs.writeFileSync(webAppIconSvgPath, webAppIconSvg);
console.log('  ✓ Updated vector SVGs');

console.log('\n[2/4] Rasterizing Master 1024x1024 PNGs...');
const fullBleedPngPath = path.join(iconDir, 'app_icon.png');
const squirclePngPath = path.join(iconDir, 'app_icon_squircle.png');
const adaptiveFgPngPath = path.join(iconDir, 'app_icon_foreground.png');

renderSvgToPng(fullBleedSvgPath, fullBleedPngPath, 1024);
renderSvgToPng(squircleSvgPath, squirclePngPath, 1024);
renderSvgToPng(adaptiveFgSvgPath, adaptiveFgPngPath, 1024);

console.log('\n[3/4] Running flutter_launcher_icons...');
try {
  execFileSync('mise', ['exec', '--', 'dart', 'run', 'flutter_launcher_icons'], {
    cwd: appDir,
    stdio: 'inherit',
  });
} catch (e) {
  // Fallback to direct dart run if mise is not needed
  execFileSync('dart', ['run', 'flutter_launcher_icons'], {
    cwd: appDir,
    stdio: 'inherit',
  });
}

console.log('\n[4/4] Generating Browser Favicon PNG...');
const faviconPngPath = path.join(webDir, 'favicon.png');
if (process.platform === 'darwin') {
  execFileSync('sips', ['-z', '64', '64', squirclePngPath, '--out', faviconPngPath], { stdio: 'pipe' });
} else {
  const favicon64Svg = faviconSvg.replace('width="128" height="128"', 'width="64" height="64"');
  const tempSvgPath = path.join(iconDir, 'temp_favicon_64.svg');
  fs.writeFileSync(tempSvgPath, favicon64Svg);
  renderSvgToPng(tempSvgPath, faviconPngPath, 64);
  fs.unlinkSync(tempSvgPath);
}
console.log('  ✓ Generated web/favicon.png (64x64)');

console.log('\n✅ All app icons and favicons generated successfully from SSOT!');
