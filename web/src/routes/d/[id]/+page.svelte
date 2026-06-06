<script>
  import { page } from '$app/stores';
  import { api } from '$lib/utils/api.js';
  import { formatBytes, timeUntil, getFileIcon } from '$lib/utils/helpers.js';
  import { generateQR } from '$lib/utils/qrcode.js';
  import { downloadViaMagnet, isWebTorrentSupported } from '$lib/utils/webtorrent.js';

  let transfer = $state(null);
  let files = $state([]);
  let loading = $state(true);
  let error = $state('');
  let downloading = $state({});
  let wtProgress = $state({});
  let password = $state('');
  let preview = $state(null);
  let copied = $state(false);
  let qrDataUrl = $state('');
  let wsConnected = $state(false);
  let liveDownloadCount = $state(0);
  let p2pPeers = $state(0);

  const transferId = $derived($page.params.id);

  $effect(() => {
    if (transferId) loadTransfer();
  });

  async function loadTransfer() {
    try {
      transfer = await api.getTransfer(transferId);
      files = await api.getTransferFiles(transferId);
      liveDownloadCount = transfer.download_count;
      const url = `${window.location.origin}/d/${transferId}`;
      generateQR(url).then(u => qrDataUrl = u);
      // Connect to WebSocket for live download updates
      connectWS(transferId);
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  function connectWS(transferID) {
    try {
      const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/ws?room=${transferID}`;
      const socket = new WebSocket(wsUrl);
      socket.onopen = () => { wsConnected = true; };
      socket.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data);
          if (msg.type === 'download' && msg.payload) {
            liveDownloadCount = msg.payload.download || liveDownloadCount + 1;
          }
        } catch {}
      };
      socket.onclose = () => { wsConnected = false; };
    } catch {}
  }

  function copyLink() {
    navigator.clipboard.writeText(`${window.location.origin}/d/${transferId}`);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }

  function isPreviewable(file) {
    const previewTypes = ['image/', 'video/', 'audio/', 'text/', 'application/pdf'];
    const previewExts = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.svg', '.mp4', '.webm', '.ogg', '.mp3', '.wav', '.pdf', '.txt', '.md', '.csv', '.json', '.html', '.htm'];
    if (transfer?.encrypted) return false;
    if (file.size > 100 * 1024 * 1024) return false; // 100MB limit for preview
    const name = file.name.toLowerCase();
    for (const ext of previewExts) {
      if (name.endsWith(ext)) return true;
    }
    for (const t of previewTypes) {
      if (file.mime_type && file.mime_type.startsWith(t)) return true;
    }
    return false;
  }

  function previewFile(file) {
    if (transfer?.encrypted) {
      preview = null;
      return;
    }
    const url = `/api/t/${transferId}/download/${file.id}${password ? '?password=' + encodeURIComponent(password) : ''}`;
    const name = file.name.toLowerCase();
    const mime = file.mime_type || '';

    // Determine preview type
    if (name.match(/\.(jpg|jpeg|png|gif|webp|svg|bmp|ico)$/) || mime.startsWith('image/')) {
      preview = { type: 'image', url, name: file.name };
    } else if (name.match(/\.(mp4|webm|ogg|mov)$/) || mime.startsWith('video/')) {
      preview = { type: 'video', url, name: file.name };
    } else if (name.match(/\.(mp3|wav|flac|ogg|aac)$/) || mime.startsWith('audio/')) {
      preview = { type: 'audio', url, name: file.name };
    } else if (name.endsWith('.pdf') || mime === 'application/pdf') {
      preview = { type: 'pdf', url, name: file.name };
    } else if (name.match(/\.(txt|md|json|xml|html|htm|css|js|py|go|rs|yaml|yml|csv)$/) || mime.startsWith('text/')) {
      preview = { type: 'text', url, name: file.name };
    } else {
      preview = null;
    }
  }

  async function downloadFile(file) {
    downloading = { ...downloading, [file.id]: true };
    wtProgress = { ...wtProgress, [file.id]: { downloaded: 0, total: 0, speed: 0 } };

    try {
      // Try WebTorrent P2P first if magnet URI is available
      if (transfer.magnet_uri && isWebTorrentSupported()) {
        try {
          const blob = await downloadViaMagnet(transfer.magnet_uri, (downloaded, total, speed) => {
            wtProgress = { ...wtProgress, [file.id]: { downloaded, total, speed } };
          });
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = file.name;
          a.click();
          URL.revokeObjectURL(url);
          p2pPeers += 1;
          return;
        } catch {
          // P2P failed, fallback to HTTP
        }
      }

      // HTTP fallback
      const res = await fetch(`/api/t/${transferId}/download/${file.id}`, {
        method: 'GET',
        headers: password ? { 'X-Password': password } : {},
      });

      if (!res.ok) throw new Error('Download failed');

      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = file.name;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      error = err.message;
    } finally {
      downloading = { ...downloading, [file.id]: false };
      wtProgress = { ...wtProgress, [file.id]: null };
    }
  }

  async function downloadAll() {
    for (const file of files) {
      await downloadFile(file);
    }
  }
</script>

<div class="max-w-3xl mx-auto">
  {#if loading}
    <div class="text-center py-20">
      <div class="w-12 h-12 mx-auto mb-4 border-2 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
      <p class="text-gray-400">Loading transfer...</p>
    </div>

  {:else if error}
    <div class="text-center py-20">
      <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-red-500/10 flex items-center justify-center">
        <svg class="w-8 h-8 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
      </div>
      <h2 class="text-xl font-bold mb-2">Transfer not available</h2>
      <p class="text-gray-400">{error}</p>
    </div>

  {:else if transfer}
    <div class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
      <!-- Header -->
      <div class="p-6 border-b border-gray-800">
        <div class="flex items-center gap-3 mb-3">
          <div class="w-12 h-12 rounded-xl bg-violet-500/10 flex items-center justify-center">
            <svg class="w-6 h-6 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/></svg>
          </div>
          <div>
            <h1 class="text-xl font-bold">{transfer.name}</h1>
            <p class="text-sm text-gray-400">
              {files.length} file{files.length > 1 ? 's' : ''} / {formatBytes(transfer.total_size)}
            </p>
          </div>
        </div>

        {#if transfer.note}
          <p class="text-sm text-gray-300 bg-gray-800/50 rounded-lg p-3 mt-3">{transfer.note}</p>
        {/if}

        <div class="flex items-center gap-4 mt-4 text-xs text-gray-500">
          <span>Downloads: {liveDownloadCount}/{transfer.max_downloads}</span>
          <span>Expires: {timeUntil(transfer.expires_at)}</span>
          {#if transfer.magnet_uri}
            <span class="flex items-center gap-1 text-blue-400" title="P2P distribution available">
              <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 24 24"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/></svg>
              P2P
            </span>
          {/if}
          {#if transfer.encrypted}
            <span class="flex items-center gap-1 text-green-400">
              <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
              Encrypted
            </span>
          {/if}
        </div>

        <!-- Share actions -->
        <div class="flex items-center gap-2 mt-4">
          <button onclick={copyLink} class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium transition-colors">
            {copied ? 'Copied!' : 'Copy link'}
          </button>
          {#if qrDataUrl}
            <img src={qrDataUrl} alt="QR" class="w-8 h-8 rounded bg-white p-0.5" />
          {/if}
        </div>
      </div>

      <!-- Password input -->
      {#if transfer.encrypted && transfer.password_hash}
        <div class="p-4 bg-gray-800/30">
          <div class="flex gap-2">
            <input type="password" bind:value={password} placeholder="Enter password" class="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
        </div>
      {/if}

      <!-- Preview area -->
      {#if preview}
        <div class="border-b border-gray-800 bg-black/40">
          {#if preview.type === 'image'}
            <img src={preview.url} alt={preview.name} class="max-w-full max-h-96 mx-auto object-contain" />
          {:else if preview.type === 'video'}
            <video src={preview.url} controls class="max-w-full max-h-96 mx-auto" playsinline>
              Your browser does not support the video tag.
            </video>
          {:else if preview.type === 'audio'}
            <div class="p-8">
              <p class="text-sm text-gray-400 mb-4">{preview.name}</p>
              <audio src={preview.url} controls class="w-full">
                Your browser does not support the audio tag.
              </audio>
            </div>
          {:else if preview.type === 'pdf'}
            <iframe src={preview.url + '#toolbar=0'} title={preview.name} class="w-full h-96 border-0"></iframe>
          {:else if preview.type === 'text'}
            <div class="p-4 max-h-96 overflow-auto">
              <pre class="text-xs text-gray-300 font-mono whitespace-pre-wrap hidden" bind:this={textContentRef}></pre>
            </div>
          {/if}
          <button onclick={() => preview = null} class="w-full py-2 text-xs text-gray-400 hover:text-white hover:bg-gray-800 transition-colors">
            Close preview
          </button>
        </div>
      {/if}

      <!-- File list -->
      <div class="divide-y divide-gray-800">
        {#each files as file}
          <div class="flex items-center gap-4 p-4 hover:bg-gray-800/30 transition-colors">
            <div class="w-10 h-10 rounded-lg bg-gray-800 flex items-center justify-center text-xs font-mono text-gray-400">
              {file.name.split('.').pop().toUpperCase().slice(0, 4)}
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium truncate">{file.name}</p>
              <p class="text-xs text-gray-500">{formatBytes(file.size)}</p>
            </div>
            {#if isPreviewable(file)}
              <button
                onclick={() => previewFile(file)}
                class="px-3 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium transition-colors"
              >
                Preview
              </button>
            {/if}
            <button
              onclick={() => downloadFile(file)}
              disabled={downloading[file.id]}
              class="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium disabled:opacity-50 transition-colors"
            >
              {#if downloading[file.id] && wtProgress[file.id]}
                P2P {Math.round((wtProgress[file.id].downloaded / (wtProgress[file.id].total || 1)) * 100)}%
              {:else if downloading[file.id]}
                Downloading...
              {:else}
                Download
              {/if}
            </button>
          </div>
        {/each}
      </div>

      <!-- Download all -->
      {#if files.length > 1}
        <div class="p-4 border-t border-gray-800">
          <button onclick={downloadAll} class="w-full py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold transition-all">
            Download all files
          </button>
        </div>
      {/if}
    </div>
  {/if}
</div>