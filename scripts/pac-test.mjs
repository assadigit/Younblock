#!/usr/bin/env node
// pac-test.mjs evaluates the generated PAC script with real JavaScript and
// asserts the routing table from file 01, section 8 (Phase 3 acceptance):
// YouTube hosts (including subdomains) go through the proxy; look-alikes
// such as notyoutube.com and everything else go DIRECT; in all mode only
// localhost and private ranges stay DIRECT; manual mode is all DIRECT.
import { execFileSync } from 'node:child_process';

const failures = [];
let checks = 0;

function shExpMatch(str, shexp) {
  const re = new RegExp(
    '^' + shexp.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.') + '$'
  );
  return re.test(str);
}
function isPlainHostName(host) {
  return !host.includes('.');
}
function ipToInt(ip) {
  const parts = ip.split('.').map(Number);
  if (parts.length !== 4 || parts.some((n) => !Number.isInteger(n) || n < 0 || n > 255)) {
    return NaN;
  }
  return ((parts[0] << 24) | (parts[1] << 16) | (parts[2] << 8) | parts[3]) >>> 0;
}
function isInNet(ip, net, mask) {
  const a = ipToInt(ip);
  const n = ipToInt(net);
  const m = ipToInt(mask);
  if (Number.isNaN(a) || Number.isNaN(n) || Number.isNaN(m)) return false;
  return (a & m) === (n & m);
}
function dnsResolve(host) {
  if (/^\d+\.\d+\.\d+\.\d+$/.test(host)) return host;
  // A fixed fake answer: the public internet.
  return '93.184.216.34';
}

function loadPAC(mode, port) {
  const pac = execFileSync('go', ['run', './cmd/pacgen', mode, String(port)], {
    encoding: 'utf8',
    maxBuffer: 1024 * 1024,
  });
  const sandbox = { shExpMatch, isPlainHostName, isInNet, dnsResolve, isResolvable: () => true };
  const keys = Object.keys(sandbox);
  const vals = Object.values(sandbox);
  const factory = new Function(...keys, pac + '\nreturn FindProxyForURL;');
  return factory(...vals);
}

function check(find, url, host, want, label) {
  checks++;
  const got = find(url, host);
  if (got !== want) {
    failures.push(`${label}: FindProxyForURL(${host}) = ${got}, want ${want}`);
  }
}

function run() {
  const PROXY = (port) => `PROXY 127.0.0.1:${port}; DIRECT`;

  // ---- youtube mode ----
  const yt = loadPAC('youtube', 4525);
  for (const host of [
    'youtube.com', 'www.youtube.com', 'm.youtube.com', 'a.b.youtube.com',
    'youtu.be', 'www.youtu.be',
    'googlevideo.com', 'r1---sn-4g5ednsk.googlevideo.com',
    'ytimg.com', 'i.ytimg.com',
    'ggpht.com', 'yt3.ggpht.com',
    'youtube-nocookie.com', 'www.youtube-nocookie.com',
    'youtubei.googleapis.com',
  ]) {
    check(yt, `https://${host}/x`, host, PROXY(4525), 'youtube mode');
  }
  for (const host of [
    'notyoutube.com', 'www.notyoutube.com',
    'youtube.com.evil.example', 'evilyoutube.com',
    'example.com', 'google.com', 'redirector.googlevideo.com.evil.example',
    'youtubecom', 'x-youtube.com',
  ]) {
    check(yt, `https://${host}/x`, host, 'DIRECT', 'youtube mode');
  }
  // Case-insensitivity: Chrome passes lowercase, but the script lowers too.
  check(yt, 'https://WWW.YouTube.com/x', 'WWW.YouTube.com', PROXY(4525), 'youtube mode case');
  check(yt, 'https://NOTYOUTUBE.com/x', 'NOTYOUTUBE.com', 'DIRECT', 'youtube mode case');

  // ---- all mode ----
  const all = loadPAC('all', 4530);
  for (const host of ['example.com', 'youtube.com', 'www.google.com']) {
    check(all, `https://${host}/x`, host, PROXY(4530), 'all mode');
  }
  for (const host of ['localhost', 'intranet', 'printer.local']) {
    check(all, `https://${host}/x`, host, 'DIRECT', 'all mode local');
  }
  for (const host of ['127.0.0.1', '10.0.0.5', '172.16.0.1', '172.31.255.255', '192.168.1.1', '169.254.7.7']) {
    check(all, `https://${host}/x`, host, 'DIRECT', 'all mode private');
  }
  check(all, 'https://172.32.0.1/x', '172.32.0.1', PROXY(4530), 'all mode just outside 172.16/12');
  check(all, 'https://8.8.8.8/x', '8.8.8.8', PROXY(4530), 'all mode public IP');
  check(all, 'https://example.test/x', 'example.test', PROXY(4530), 'all mode resolved public');

  // ---- manual mode ----
  const manual = loadPAC('manual', 4525);
  check(manual, 'https://www.youtube.com/x', 'www.youtube.com', 'DIRECT', 'manual mode');
  check(manual, 'https://example.com/x', 'example.com', 'DIRECT', 'manual mode');

  if (failures.length > 0) {
    console.error(`PAC behavior test FAILED (${failures.length} of ${checks} checks):`);
    for (const f of failures) console.error('  ' + f);
    process.exit(1);
  }
  console.log(`PAC behavior test: OK (${checks} checks across youtube/all/manual modes)`);
}

run();
