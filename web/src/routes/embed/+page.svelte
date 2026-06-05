<script>
  // Minimal embedded upload form - no layout, no nav, just the upload widget
  let dragOver = $state(false);
  let uploading = $state(false);
  let progress = $state(0);
  let result = $state(null);
  let error = $state('');
  let copied = $state(false);
  let note = $state('');

  const CHUNK_SIZE = 5 * 1024 * 1024;
  let currentFile = null;

  function handleDrop(e) {
    e.preventDefault();
    dragOver = false;
    if (e.dataTransfer.files.length > 0) currentFile = e.dataTransfer.files[0];
  }

  function handleFileInput(e) {
    if (e.target.files.length > 0) currentFile = e.target.files[0];
  }

  async function upload() {
    if (!currentFile) return;
    uploading = true;
    error = '';
    progress = 0;

    try {
      // Read brand config from parent frame if available
      const brandColor = new URLSearchParams(window.location.search).get('color') || '#7c3aed';

      const initRes = await (await fetch('/api/upload/init', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: currentFile.name,
          file_count: 1,
          encrypted: false,
          expiry_hours: 24,
          note: note,
        }),
      })).json();

      const transferId = initRes.transfer_id;
      const totalChunks = Math.ceil(currentFile.size / CHUNK_SIZE);
      let fileId = null;
      const startTime = Date.now();

      for (let i = 0; i < totalChunks; i++) {
        const start = i * CHUNK_SIZE;
        const end = Math.min(start + CHUNK_SIZE, currentFile.size);
        const chunk = currentFile.slice(start, end);
        const buffer = await chunk.arrayBuffer();

        const res = await fetch('/api/upload/chunk', {
          method: 'POST',
          headers: {
            'X-Transfer-ID': transferId,
            'X-File-ID': fileId || '',
            'X-Chunk-Index': String(i),
            'X-Total-Chunks': String(totalChunks),
            'X-File-Name': currentFile.name,
            'X-File-Size': String(currentFile.size),
            'X-Mime-Type': currentFile.type || 'application/octet-stream',
          },
          body: buffer,
        });
        if (!res.ok) throw new Error('Chunk upload failed');
        const data = await res.json();
        fileId = data.file_id;
        progress = Math.round(((i + 1) / totalChunks) * 100);
      }

      const completeRes = await (await fetch(`/api/upload/complete/${transferId}`, { method: 'POST' })).json();
      result = completeRes;
      progress = 100;

      // Notify parent window
      window.parent.postMessage({ type: 'lobeam-upload-complete', url: completeRes.download_url }, '*');
    } catch (err) {
      error = err.message;
    } finally {
      uploading = false;
    }
  }

  function copyLink() {
    if (result?.download_url) {
      navigator.clipboard.writeText(result.download_url);
      copied = true;
      setTimeout(() => copied = false, 2000);
    }
  }

  function reset() {
    currentFile = null;
    result = null;
    error = '';
    progress = 0;
  }
</script>

<div class="lobeam-embed">
  <style>
    .lobeam-embed {
      width: 100%;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: transparent;
      color: #d1d5db;
    }
  </style>

  {#if result}
    <div style="text-align:center;padding:24px;background:#111827;border-radius:16px;">
      <div style="width:48px;height:48px;margin:0 auto 12px;border-radius:50%;background:rgba(34,197,94,0.1);display:flex;align-items:center;justify-content:center;">
        <svg style="width:24px;height:24px;color:#22c55e;" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
      </div>
      <p style="font-weight:600;margin-bottom:8px;color:white;">File uploaded</p>
      <div style="display:flex;gap:8px;margin-bottom:12px;">
        <input type="text" value={result.download_url} readonly style="flex:1;background:#1f2937;border:1px solid #374151;border-radius:8px;padding:8px 12px;font-size:13px;color:#d1d5db;"/>
        <button onclick={copyLink} style="padding:8px 16px;background:#7c3aed;border:none;border-radius:8px;color:white;font-size:13px;font-weight:500;cursor:pointer;white-space:nowrap;">{copied ? 'Copied!' : 'Copy'}</button>
      </div>
      <button onclick={reset} style="background:none;border:none;color:#9ca3af;font-size:13px;cursor:pointer;">Upload another file</button>
    </div>

  {:else}
    <div
      style="border:2px dashed #374151;border-radius:16px;padding:32px;text-align:center;transition:border-color .2s;{dragOver ? 'border-color:#7c3aed;background:rgba(124,58,237,0.05);' : ''}"
      ondrop={handleDrop}
      ondragover={(e) => { e.preventDefault(); dragOver = true; }}
      ondragleave={() => dragOver = false}
    >
      {#if currentFile}
        <div style="margin-bottom:16px;">
          <p style="font-size:14px;color:#d1d5db;margin-bottom:4px;">{currentFile.name}</p>
          <p style="font-size:12px;color:#6b7280;">{(currentFile.size / (1024*1024)).toFixed(1)} MB</p>
        </div>

        {#if uploading}
          <div style="margin-bottom:16px;">
            <div style="width:100%;height:4px;background:#1f2937;border-radius:2px;overflow:hidden;margin-bottom:8px;">
              <div style="height:100%;background:linear-gradient(90deg,#7c3aed,#4f46e5);border-radius:2px;transition:width .3s;width:{progress}%;"></div>
            </div>
            <p style="font-size:12px;color:#6b7280;">{progress}%</p>
          </div>
        {:else}
          <div style="margin-bottom:16px;">
            <input type="text" bind:value={note} placeholder="Add a note (optional)" style="width:100%;max-width:300px;background:#1f2937;border:1px solid #374151;border-radius:8px;padding:8px 12px;font-size:13px;color:#d1d5db;outline:none;"/>
          </div>
        {/if}

        {#if error}
          <p style="font-size:12px;color:#ef4444;margin-bottom:12px;">{error}</p>
        {/if}

        <div style="display:flex;gap:8px;justify-content:center;">
          <button onclick={upload} disabled={uploading} style="padding:8px 24px;background:#7c3aed;border:none;border-radius:10px;color:white;font-size:14px;font-weight:600;cursor:pointer;opacity:{uploading ? '0.5' : '1'};">
            {uploading ? 'Uploading...' : 'Upload'}
          </button>
          <button onclick={() => currentFile = null} style="padding:8px 16px;background:#1f2937;border:none;border-radius:10px;color:#9ca3af;font-size:14px;cursor:pointer;">Cancel</button>
        </div>
      {:else}
        <div style="margin-bottom:16px;">
          <div style="width:48px;height:48px;margin:0 auto 12px;border-radius:12px;background:#1f2937;display:flex;align-items:center;justify-content:center;">
            <svg style="width:24px;height:24px;color:#6b7280;" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/></svg>
          </div>
          <p style="font-weight:500;margin-bottom:4px;color:#e5e7eb;">Drop file here</p>
          <p style="font-size:12px;color:#6b7280;">or click to browse</p>
        </div>
        <label style="display:inline-flex;align-items:center;gap:8px;padding:10px 24px;background:#7c3aed;border-radius:10px;color:white;font-size:14px;font-weight:500;cursor:pointer;">
          Select file
          <input type="file" class="hidden" onchange={handleFileInput} style="display:none;"/>
        </label>
      {/if}
    </div>
  {/if}
</div>