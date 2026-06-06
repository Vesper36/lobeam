<script>
  import { page } from '$app/stores';
  import { api } from '$lib/utils/api.js';
  import { formatBytes, timeUntil } from '$lib/utils/helpers.js';
  import { generateQR } from '$lib/utils/qrcode.js';

  let folder = $state(null);
  let files = $state([]);
  let loading = $state(true);
  let error = $state('');
  let uploading = $state(false);
  let uploadProgress = $state({});
  let uploadFiles = $state([]);
  let password = $state('');
  let showPassword = $state(false);
  let showQr = $state(false);
  let copied = $state(false);
  let qrDataUrl = $state('');

  const token = $derived($page.params.token);
  const canUpload = $derived(folder && folder.mode !== 'download_only');
  const canDownload = $derived(folder && folder.mode !== 'upload_only');

  $effect(() => {
    if (token) loadFolder();
  });

  async function loadFolder() {
    try {
      folder = await api.getWebFolder(token);
      files = await api.getWebFolderFiles(token);
      // Generate QR code
      const url = `${window.location.origin}/f/${token}`;
      qrDataUrl = await generateQR(url, 128);
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  function handleFileInput(e) {
    uploadFiles = [...uploadFiles, ...Array.from(e.target.files)];
  }

  function handleDrop(e) {
    e.preventDefault();
    uploadFiles = [...uploadFiles, ...Array.from(e.dataTransfer.files)];
  }

  function removeFile(i) {
    uploadFiles = uploadFiles.filter((_, idx) => idx !== i);
  }

  async function uploadAll() {
    if (uploadFiles.length === 0) return;
    uploading = true;
    try {
      for (const file of uploadFiles) {
        uploadProgress[file.name] = 0;
        await api.uploadToWebFolder(token, file, (p) => {
          uploadProgress[file.name] = Math.round(p * 100);
        });
      }
      uploadFiles = [];
      uploadProgress = {};
      files = await api.getWebFolderFiles(token);
      folder = await api.getWebFolder(token);
    } catch (err) {
      error = err.message;
    } finally {
      uploading = false;
    }
  }

  function downloadFile(file) {
    const url = api.getWebFolderDownloadUrl(token, file.id);
    window.open(url, '_blank');
  }

  function copyLink() {
    navigator.clipboard.writeText(`${window.location.origin}/f/${token}`);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }
</script>

<div class="max-w-3xl mx-auto">
  {#if loading}
    <div class="text-center py-20">
      <div class="w-12 h-12 mx-auto mb-4 border-2 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
      <p class="text-gray-400">Loading folder...</p>
    </div>

  {:else if error}
    <div class="text-center py-20">
      <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-red-500/10 flex items-center justify-center">
        <svg class="w-8 h-8 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
      </div>
      <h2 class="text-xl font-bold mb-2">Folder not available</h2>
      <p class="text-gray-400">{error}</p>
    </div>

  {:else if folder}
    <!-- Header -->
    <div class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
      <div class="p-6 border-b border-gray-800">
        <div class="flex items-start gap-4">
          <div class="w-12 h-12 rounded-xl bg-violet-500/10 flex items-center justify-center flex-shrink-0">
            <svg class="w-6 h-6 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
          </div>
          <div class="flex-1 min-w-0">
            <h1 class="text-xl font-bold">{folder.name}</h1>
            {#if folder.description}
              <p class="text-sm text-gray-400 mt-1">{folder.description}</p>
            {/if}
            <div class="flex items-center gap-3 mt-2 text-xs text-gray-500">
              <span>{files.length} file{files.length !== 1 ? 's' : ''}</span>
              <span>Expires: {timeUntil(folder.expires_at)}</span>
              <span class="px-2 py-0.5 rounded bg-gray-800 text-gray-300">
                {folder.mode === 'upload_only' ? 'Collect only' : folder.mode === 'download_only' ? 'Share only' : 'Collaborate'}
              </span>
              {#if folder.password_hash}
                <span class="flex items-center gap-1 text-amber-400">
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
                  Protected
                </span>
              {/if}
            </div>
          </div>
        </div>

        <!-- Share actions -->
        <div class="flex items-center gap-2 mt-4">
          <button onclick={copyLink} class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm transition-colors">
            {copied ? 'Copied!' : 'Copy link'}
          </button>
          {#if qrDataUrl}
            <button onclick={() => showQr = !showQr} class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm transition-colors">
              QR Code
            </button>
          {/if}
        </div>

        {#if showQr && qrDataUrl}
          <div class="mt-4 inline-flex flex-col items-center rounded-xl border border-gray-800 bg-gray-950 p-3">
            <img src={qrDataUrl} alt="Folder QR Code" class="h-32 w-32 rounded-lg bg-white p-2" />
            <p class="mt-2 text-xs text-gray-500">Scan folder link</p>
          </div>
        {/if}
      </div>

      <!-- Upload area (when allowed) -->
      {#if canUpload}
        <div class="p-6 border-b border-gray-800">
          <h3 class="text-sm font-medium mb-3">Upload files</h3>

          {#if folder.password_hash && !showPassword}
            <div class="flex gap-2">
              <input type="password" bind:value={password} placeholder="Enter folder password" class="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
              <button onclick={() => showPassword = true} class="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
                Unlock
              </button>
            </div>
          {:else}
            <div
              class="border-2 border-dashed rounded-xl transition-all duration-200 {uploadFiles.length > 0 ? 'border-violet-400 bg-violet-500/5' : 'border-gray-700 hover:border-gray-600'}"
              ondrop={handleDrop}
              ondragover={(e) => e.preventDefault()}
              role="region"
              aria-label="Web folder upload area"
            >
              {#if uploadFiles.length === 0}
                <div class="p-8 text-center">
                  <label class="cursor-pointer inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
                    Select files
                    <input type="file" multiple class="hidden" onchange={handleFileInput} />
                  </label>
                  <p class="text-xs text-gray-500 mt-2">or drag and drop</p>
                </div>
              {:else}
                <div class="p-4">
                  <div class="space-y-2 mb-4 max-h-40 overflow-y-auto">
                    {#each uploadFiles as file, i}
                      <div class="flex items-center gap-2 p-2 bg-gray-800/50 rounded-lg">
                        <span class="text-sm truncate flex-1">{file.name}</span>
                        <span class="text-xs text-gray-500">{formatBytes(file.size)}</span>
                        {#if uploadProgress[file.name] !== undefined && uploadProgress[file.name] < 100}
                          <span class="text-xs text-violet-400">{uploadProgress[file.name]}%</span>
                        {:else if uploadProgress[file.name] === 100}
                          <svg class="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
                        {/if}
                        <button onclick={() => removeFile(i)} class="text-gray-500 hover:text-white" aria-label="Remove file">
                          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
                        </button>
                      </div>
                    {/each}
                  </div>
                  <div class="flex items-center gap-2">
                    <label class="cursor-pointer px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm transition-colors">
                      Add more
                      <input type="file" multiple class="hidden" onchange={handleFileInput} />
                    </label>
                    <button onclick={uploadAll} disabled={uploading} class="px-4 py-1.5 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium disabled:opacity-50 transition-colors">
                      {uploading ? 'Uploading...' : 'Upload all'}
                    </button>
                  </div>
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/if}

      <!-- File list (when allowed) -->
      {#if canDownload && files.length > 0}
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
              <button onclick={() => downloadFile(file)} class="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
                Download
              </button>
            </div>
          {/each}
        </div>
      {:else if canDownload}
        <div class="p-8 text-center text-gray-400">
          <p>No files yet</p>
        </div>
      {/if}
    </div>
  {/if}
</div>
