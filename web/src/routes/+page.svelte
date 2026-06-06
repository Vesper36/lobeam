<script>
  import { api } from '$lib/utils/api.js';
  import { formatBytes, formatSpeed, formatDuration, copyToClipboard } from '$lib/utils/helpers.js';
  import { generateQR } from '$lib/utils/qrcode.js';
  import { seedFiles, isWebTorrentSupported } from '$lib/utils/webtorrent.js';

  let files = $state([]);
  let uploading = $state(false);
  let progress = $state(0);
  let speed = $state(0);
  let eta = $state(0);
  let result = $state(null);
  let error = $state('');
  let dragOver = $state(false);
  let encrypted = $state(true);
  let password = $state('');
  let maxDownloads = $state(100);
  let expiryHours = $state(24);
  let note = $state('');
  let copied = $state(false);
  let qrDataUrl = $state('');
  let shareEarlyLink = $state('');
  let senderEmail = $state('');
  let receiverEmail = $state('');
  let emailSent = $state(false);
  let transferId = $state('');

  const CHUNK_SIZE = 5 * 1024 * 1024; // 5MB

  function handleDrop(e) {
    e.preventDefault();
    dragOver = false;
    const droppedFiles = Array.from(e.dataTransfer.files);
    addFiles(droppedFiles);
  }

  function handleFileInput(e) {
    const selected = Array.from(e.target.files);
    addFiles(selected);
  }

  function addFiles(newFiles) {
    files = [...files, ...newFiles];
    error = '';
  }

  function removeFile(index) {
    files = files.filter((_, i) => i !== index);
  }

  function clearAll() {
    files = [];
    result = null;
    error = '';
    progress = 0;
    shareEarlyLink = '';
    transferId = '';
    qrDataUrl = '';
    emailSent = false;
  }

  async function upload() {
    if (files.length === 0) {
      error = 'Please select at least one file';
      return;
    }
    if (receiverEmail && !receiverEmail.includes('@')) {
      error = 'Invalid receiver email address';
      return;
    }

    uploading = true;
    error = '';
    progress = 0;

    try {
      const totalSize = files.reduce((sum, f) => sum + f.size, 0);
      let uploadedSize = 0;
      const startTime = Date.now();

      // Init transfer — returns share URL immediately for real-time sharing
      const initRes = await api.initUpload({
        name: files.length === 1 ? files[0].name : `${files.length} files`,
        file_count: files.length,
        encrypted: encrypted,
        password: password || undefined,
        max_downloads: maxDownloads,
        expiry_hours: expiryHours,
        note: note,
        sender_email: senderEmail || undefined,
        receiver_email: receiverEmail || undefined,
      });

      transferId = initRes.transfer_id;
      if (initRes.download_url) {
        shareEarlyLink = initRes.download_url;
        generateQR(initRes.download_url).then(url => qrDataUrl = url);
      }

      // Upload each file
      for (const file of files) {
        const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
        let fileId = null;

        for (let i = 0; i < totalChunks; i++) {
          const start = i * CHUNK_SIZE;
          const end = Math.min(start + CHUNK_SIZE, file.size);
          const chunk = file.slice(start, end);

          const buffer = await chunk.arrayBuffer();
          const hashBuffer = await crypto.subtle.digest('SHA-256', buffer);
          const hashArray = Array.from(new Uint8Array(hashBuffer));
          const chunkHash = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');

          const res = await api.uploadChunk(
            transferId, fileId, i, totalChunks,
            file.name, file.size, file.type || 'application/octet-stream',
            buffer, chunkHash
          );

          fileId = res.file_id;
          uploadedSize += chunk.size;
          progress = Math.round((uploadedSize / totalSize) * 100);

          const elapsed = (Date.now() - startTime) / 1000;
          speed = uploadedSize / elapsed;
          eta = (totalSize - uploadedSize) / speed;
        }
      }

      // Complete transfer
      const completeRes = await api.completeUpload(transferId);
      result = completeRes;
      progress = 100;

      // Seed via WebTorrent for P2P distribution (non-blocking)
      if (isWebTorrentSupported() && files.length > 0) {
        seedFiles(files, transferId).then(({ magnetURI }) => {
          if (magnetURI && transferId) {
            api.updateMagnet(transferId, magnetURI).catch(() => {});
          }
        }).catch(() => {});
      }

      // If email is set, send automatically after completion
      if (receiverEmail && transferId) {
        try {
          await api.emailTransfer(transferId, {
            email: receiverEmail,
            subject: `Files shared via LoBeam: ${files.length > 1 ? `${files.length} files` : files[0].name}`,
            message: note || 'Files have been shared with you.',
          });
          emailSent = true;
        } catch { /* email failed but transfer succeeded */ }
      }

    } catch (err) {
      error = err.message;
    } finally {
      uploading = false;
    }
  }

  function copyLink() {
    const url = result?.download_url || shareEarlyLink;
    if (url) {
      copyToClipboard(url);
      copied = true;
      setTimeout(() => copied = false, 2000);
    }
  }

  function handleKeyDown(e) {
    if (notesRef && e.key === ' ' && e.shiftKey) {
      e.preventDefault();
    }
  }
</script>

<div class="max-w-3xl mx-auto">
  <!-- Hero -->
  <div class="text-center mb-10">
    <h1 class="text-4xl sm:text-5xl font-bold mb-4 bg-gradient-to-r from-violet-400 to-indigo-400 bg-clip-text text-transparent">
      Share files, securely
    </h1>
    <p class="text-lg text-gray-400 max-w-xl mx-auto">
      End-to-end encrypted file transfer. No registration required. Files expire automatically.
    </p>
  </div>

  {#if result}
    <!-- Success state -->
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-8 text-center">
      <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-green-500/10 flex items-center justify-center">
        <svg class="w-8 h-8 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
      </div>
      <h2 class="text-2xl font-bold mb-2">Transfer ready</h2>
      <p class="text-gray-400 mb-6">Share this link with your recipient</p>

      <div class="bg-gray-800 rounded-xl p-4 flex items-center gap-3 mb-4">
        <input type="text" value={result.download_url} readonly class="flex-1 bg-transparent text-sm text-gray-300 outline-none" />
        <button onclick={copyLink} class="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors whitespace-nowrap">
          {copied ? 'Copied!' : 'Copy link'}
        </button>
      </div>

      {#if qrDataUrl}
        <div class="mb-4">
          <img src={qrDataUrl} alt="QR Code" class="inline-block w-32 h-32 rounded-xl bg-white p-2" />
          <p class="text-xs text-gray-500 mt-1">Scan to download</p>
        </div>
      {/if}

      {#if emailSent}
        <div class="mb-4 p-3 bg-green-500/10 border border-green-500/20 rounded-xl text-sm text-green-400">
          Download link sent to {receiverEmail}
        </div>
      {/if}

      <button onclick={clearAll} class="text-sm text-gray-400 hover:text-white transition-colors">
        Send more files
      </button>
    </div>

  {:else}
    <!-- Upload area -->
    <div
      class="relative border-2 border-dashed rounded-2xl transition-all duration-200 {dragOver ? 'border-violet-400 bg-violet-500/5' : 'border-gray-700 hover:border-gray-600'}"
      ondrop={handleDrop}
      ondragover={(e) => { e.preventDefault(); dragOver = true; }}
      ondragleave={() => dragOver = false}
      role="region"
      aria-label="File upload drop zone"
    >
      {#if files.length === 0}
        <!-- Empty state -->
        <div class="p-12 text-center">
          <div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gray-800 flex items-center justify-center">
            <svg class="w-8 h-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/></svg>
          </div>
          <p class="text-lg font-medium mb-2">Drop files here or click to browse</p>
          <p class="text-sm text-gray-500 mb-4">Any file type, any size</p>
          <label class="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-violet-600 hover:bg-violet-700 text-white font-medium cursor-pointer transition-colors">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
            Select files
            <input type="file" multiple class="hidden" onchange={handleFileInput} />
          </label>
        </div>
      {:else}
        <!-- File list -->
        <div class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h3 class="font-semibold">{files.length} file{files.length > 1 ? 's' : ''} selected</h3>
            <button onclick={clearAll} class="text-sm text-gray-400 hover:text-white transition-colors">Clear all</button>
          </div>

          <div class="space-y-2 mb-6 max-h-60 overflow-y-auto">
            {#each files as file, i}
              <div class="flex items-center gap-3 p-3 bg-gray-800/50 rounded-xl">
                <div class="w-10 h-10 rounded-lg bg-gray-700 flex items-center justify-center text-xs font-mono text-gray-400">
                  {file.name.split('.').pop().toUpperCase().slice(0, 4)}
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium truncate">{file.name}</p>
                  <p class="text-xs text-gray-500">{formatBytes(file.size)}</p>
                </div>
                <button onclick={() => removeFile(i)} class="p-1 rounded hover:bg-gray-700 transition-colors" aria-label="Remove file">
                  <svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
                </button>
              </div>
            {/each}
          </div>

          <!-- Add more files -->
          <label class="inline-flex items-center gap-2 text-sm text-violet-400 hover:text-violet-300 cursor-pointer mb-6">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
            Add more files
            <input type="file" multiple class="hidden" onchange={handleFileInput} />
          </label>

          <!-- Real-time sharing: show link while uploading -->
          {#if shareEarlyLink && uploading}
            <div class="mb-4 p-4 bg-violet-500/10 border border-violet-500/20 rounded-xl">
              <p class="text-sm font-medium text-violet-300 mb-2">Share now (before upload completes)</p>
              <div class="flex items-center gap-2">
                <input type="text" value={shareEarlyLink} readonly class="flex-1 bg-gray-800 rounded-lg px-3 py-1.5 text-xs text-gray-300 outline-none" />
                <button onclick={copyLink} class="px-3 py-1.5 rounded-lg bg-violet-600 hover:bg-violet-700 text-xs font-medium transition-colors whitespace-nowrap">
                  Copy
                </button>
              </div>
            </div>
          {/if}

          <!-- Options -->
          <div class="space-y-4 p-4 bg-gray-800/30 rounded-xl mb-6">
            <!-- Encryption -->
            <label class="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" bind:checked={encrypted} class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-violet-500 focus:ring-violet-500" />
              <div>
                <span class="text-sm font-medium">End-to-end encryption</span>
                <p class="text-xs text-gray-500">Files are encrypted before upload</p>
              </div>
            </label>

            {#if encrypted}
              <div>
                <label class="block text-xs text-gray-400 mb-1">Password (optional, for extra protection)</label>
                <input type="password" bind:value={password} placeholder="Leave empty for link-only encryption" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
              </div>
            {/if}

            <!-- Email section -->
            <div class="border-t border-gray-700/50 pt-4">
              <p class="text-sm font-medium mb-3">Send via email</p>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs text-gray-400 mb-1">Your email</label>
                  <input type="email" bind:value={senderEmail} placeholder="you@example.com" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
                </div>
                <div>
                  <label class="block text-xs text-gray-400 mb-1">Recipient email</label>
                  <input type="email" bind:value={receiverEmail} placeholder="them@example.com" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
                </div>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-xs text-gray-400 mb-1">Max downloads</label>
                <input type="number" bind:value={maxDownloads} min="1" max="10000" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
              </div>
              <div>
                <label class="block text-xs text-gray-400 mb-1">Expires after (hours)</label>
                <input type="number" bind:value={expiryHours} min="1" max="720" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
              </div>
            </div>

            <div>
              <label class="block text-xs text-gray-400 mb-1">Note (optional)</label>
              <textarea bind:value={note} placeholder="Add a message for the recipient..." rows="2" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500 resize-none"></textarea>
            </div>
          </div>

          <!-- Progress -->
          {#if uploading}
            <div class="mb-6">
              <div class="flex items-center justify-between text-sm mb-2">
                <span class="text-gray-400">Uploading...</span>
                <span class="text-gray-400">{progress}%</span>
              </div>
              <div class="w-full h-2 bg-gray-800 rounded-full overflow-hidden">
                <div class="h-full bg-gradient-to-r from-violet-500 to-indigo-500 rounded-full transition-all duration-300" style="width: {progress}%"></div>
              </div>
              <div class="flex items-center justify-between text-xs text-gray-500 mt-2">
                <span>{formatSpeed(speed)}</span>
                <span>{eta > 0 ? formatDuration(eta) + ' remaining' : ''}</span>
              </div>
            </div>
          {/if}

          <!-- Error -->
          {#if error}
            <div class="mb-4 p-3 bg-red-500/10 border border-red-500/20 rounded-xl text-sm text-red-400">
              {error}
            </div>
          {/if}

          <!-- Upload button -->
          <button
            onclick={upload}
            disabled={uploading || files.length === 0}
            class="w-full py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-lg shadow-violet-500/20 hover:shadow-violet-500/30"
          >
            {uploading ? 'Uploading...' : `Upload ${files.length} file${files.length > 1 ? 's' : ''}`}
          </button>
        </div>
      {/if}
    </div>
  {/if}
</div>