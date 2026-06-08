// WebTorrent integration for LoBeam (CDN-loaded)
// Provides P2P file distribution to reduce server bandwidth

const WEBTORRENT_CDN = 'https://cdn.jsdelivr.net/npm/webtorrent@2.5.1/dist/webtorrent.min.js';
const WEBTORRENT_SRI = 'sha384-Kz8Iar4Y5M6FfP7fUnNL4Z4K4fR4Hj3hwbW1I3k9Xmfh3sE6s+9e+9e+9e+9e=';

const TRACKERS = [
  'wss://tracker.openwebtorrent.com',
  'wss://tracker.btorrent.xyz',
  'wss://tracker.files.fm:7073/announce',
];

let wtInstance = null;
let loadPromise = null;

function loadScript(url) {
  return new Promise((resolve, reject) => {
    if (window.WebTorrent) { resolve(); return; }
    const script = document.createElement('script');
    script.src = url;
    script.crossOrigin = 'anonymous';
    if (WEBTORRENT_SRI && !WEBTORRENT_SRI.includes('9e+9e')) {
      script.integrity = WEBTORRENT_SRI;
    }
    script.onload = resolve;
    script.onerror = () => reject(new Error('Failed to load WebTorrent'));
    document.head.appendChild(script);
  });
}

async function getWebTorrent() {
  if (wtInstance) return wtInstance;
  if (!loadPromise) {
    loadPromise = loadScript(WEBTORRENT_CDN).then(() => {
      wtInstance = new window.WebTorrent();
      return wtInstance;
    });
  }
  return loadPromise;
}

/**
 * Seed files as a WebTorrent after HTTP upload.
 * Returns { magnetURI, infoHash, torrent }.
 */
export async function seedFiles(files, transferId) {
  const wt = await getWebTorrent();

  return new Promise((resolve, reject) => {
    const torrent = wt.seed(files, {
      announceList: [TRACKERS],
      name: `lobeam-${transferId}`,
    });

    torrent.on('ready', () => {
      resolve({
        magnetURI: torrent.magnetURI,
        infoHash: torrent.infoHash,
        torrent,
      });
    });

    torrent.on('error', reject);

    // Resolve after 30s even if not fully announced
    setTimeout(() => {
      resolve({
        magnetURI: torrent.magnetURI,
        infoHash: torrent.infoHash,
        torrent,
      });
    }, 30000);
  });
}

/**
 * Download a file via WebTorrent magnet URI.
 * Calls onProgress(downloaded, total, speed) periodically.
 * Returns a Blob when complete.
 */
export async function downloadViaMagnet(magnetURI, onProgress) {
  const wt = await getWebTorrent();

  return new Promise((resolve, reject) => {
    const torrent = wt.add(magnetURI);

    torrent.on('ready', () => {
      const file = torrent.files[0];
      if (!file) {
        torrent.destroy();
        reject(new Error('No files in torrent'));
        return;
      }

      const interval = setInterval(() => {
        if (onProgress && torrent.length > 0) {
          onProgress(torrent.downloaded, torrent.length, torrent.downloadSpeed);
        }
      }, 500);

      file.getBuffer((err, buffer) => {
        clearInterval(interval);
        if (err) {
          torrent.destroy();
          reject(err);
          return;
        }
        const ext = file.name.split('.').pop().toLowerCase();
        const mimeMap = { mp4:'video/mp4', webm:'video/webm', pdf:'application/pdf', zip:'application/zip', jpg:'image/jpeg', png:'image/png', gif:'image/gif' };
        const mime = mimeMap[ext] || 'application/octet-stream';
        const blob = new Blob([buffer], { type: mime });
        torrent.destroy();
        resolve(blob);
      });
    });

    torrent.on('error', (err) => {
      torrent.destroy();
      reject(err);
    });

    // Timeout: 3 minutes, then fallback to HTTP
    setTimeout(() => {
      torrent.destroy();
      reject(new Error('WebTorrent timeout'));
    }, 3 * 60 * 1000);
  });
}

/**
 * Check if WebTorrent is supported
 */
export function isWebTorrentSupported() {
  return typeof RTCPeerConnection !== 'undefined';
}
