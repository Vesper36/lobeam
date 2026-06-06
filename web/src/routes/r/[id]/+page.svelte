<script>
  import { page } from '$app/stores';
  import { api } from '$lib/utils/api.js';
  import { formatBytes, timeUntil } from '$lib/utils/helpers.js';

  let req = $state(null);
  let loading = $state(true);
  let error = $state('');
  let success = $state(false);
  let uploading = $state(false);
  let uploadProgress = $state(0);
  let files = $state([]);
  let senderName = $state('');
  let senderEmail = $state('');
  let message = $state('');

  const id = $derived($page.params.id);
  let customFields = $derived(req ? JSON.parse(req.custom_fields || '[]') : []);
  let requireFields = $derived(req ? JSON.parse(req.require_fields || '[]') : []);
  let allowedTypes = $derived(req ? JSON.parse(req.allowed_types || '[]') : []);

  $effect(() => {
    if (id) loadRequest();
  });

  async function loadRequest() {
    try {
      req = await api.getFileRequest(id);
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  function handleDrop(e) {
    e.preventDefault();
    files = [...files, ...Array.from(e.dataTransfer.files)];
  }

  function handleFileInput(e) {
    files = [...files, ...Array.from(e.target.files)];
  }

  function removeFile(i) {
    files = files.filter((_, idx) => idx !== i);
  }

  async function uploadAll() {
    if (files.length === 0) return;
    if (requireFields.includes('name') && !senderName.trim()) {
      error = 'Your name is required';
      return;
    }
    if (requireFields.includes('email') && !senderEmail.trim()) {
      error = 'Your email is required';
      return;
    }
    if (req.max_files > 0 && files.length > req.max_files) {
      error = `This request accepts up to ${req.max_files} file${req.max_files === 1 ? '' : 's'}`;
      return;
    }
    const blockedFile = files.find((file) => !isTypeAllowed(file.name));
    if (blockedFile) {
      error = `${blockedFile.name} is not an allowed file type`;
      return;
    }
    uploading = true;
    error = '';
    try {
      for (const file of files) {
        uploadProgress = 0;
        await api.uploadToFileRequest(id, file, (p) => {
          uploadProgress = Math.round(p * 100);
        }, { name: senderName, email: senderEmail, message });
      }
      success = true;
    } catch (err) {
      error = err.message;
    } finally {
      uploading = false;
    }
  }

  function isTypeAllowed(fileName) {
    if (allowedTypes.length === 0) return true;
    const ext = fileName.split('.').pop().toLowerCase();
    return allowedTypes.some(t => t.replace('.', '').toLowerCase() === ext);
  }
</script>

<div class="max-w-3xl mx-auto">
  {#if loading}
    <div class="text-center py-20">
      <div class="w-12 h-12 mx-auto mb-4 border-2 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
      <p class="text-gray-400">Loading...</p>
    </div>

  {:else if error}
    <div class="text-center py-20">
      <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-red-500/10 flex items-center justify-center">
        <svg class="w-8 h-8 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
      </div>
      <h2 class="text-xl font-bold mb-2">Request not available</h2>
      <p class="text-gray-400">{error}</p>
    </div>

  {:else if success}
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-8 text-center">
      <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-green-500/10 flex items-center justify-center">
        <svg class="w-8 h-8 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
      </div>
      <h2 class="text-2xl font-bold mb-2">Files sent</h2>
      <p class="text-gray-400 mb-6">Your files have been uploaded successfully.</p>
      <button onclick={() => { success = false; files = []; }} class="px-6 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
        Send more files
      </button>
    </div>

  {:else if req}
    <div class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
      <!-- Header -->
      <div class="p-6 border-b border-gray-800">
        <h1 class="text-2xl font-bold">{req.title}</h1>
        {#if req.description}
          <p class="text-gray-400 mt-2">{req.description}</p>
        {/if}
        <div class="flex items-center gap-3 mt-3 text-xs text-gray-500">
          <span>Expires: {timeUntil(req.expires_at)}</span>
          {#if req.max_file_size > 0}
            <span>Max file size: {formatBytes(req.max_file_size)}</span>
          {/if}
          {#if req.max_files > 0}
            <span>Max files: {req.max_files}</span>
          {/if}
          {#if allowedTypes.length > 0}
            <span>Allowed: {allowedTypes.join(', ')}</span>
          {/if}
        </div>
      </div>

      <!-- Upload form -->
      <div class="p-6">
        <!-- Sender info fields -->
        <div class="space-y-4 mb-6">
          {#if requireFields.includes('name') || customFields.includes('name')}
            <div>
              <label class="block text-xs text-gray-400 mb-1">
                Your name
                {#if requireFields.includes('name')}<span class="text-red-400">*</span>{/if}
              </label>
              <input type="text" bind:value={senderName} placeholder="Enter your name" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
          {/if}
          {#if requireFields.includes('email') || customFields.includes('email')}
            <div>
              <label class="block text-xs text-gray-400 mb-1">
                Your email
                {#if requireFields.includes('email')}<span class="text-red-400">*</span>{/if}
              </label>
              <input type="email" bind:value={senderEmail} placeholder="Enter your email" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
          {/if}
          <div>
            <label class="block text-xs text-gray-400 mb-1">Message (optional)</label>
            <textarea bind:value={message} placeholder="Add a message..." rows="2" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500 resize-none"></textarea>
          </div>
        </div>

        <!-- File drop area -->
        <div
          class="border-2 border-dashed rounded-xl transition-all duration-200 mb-6 {files.length > 0 ? 'border-violet-400 bg-violet-500/5' : 'border-gray-700 hover:border-gray-600'}"
          ondrop={handleDrop}
          ondragover={(e) => e.preventDefault()}
          role="region"
          aria-label="File request upload area"
        >
          {#if files.length === 0}
            <div class="p-8 text-center">
              <label class="cursor-pointer inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
                Select files
                <input type="file" multiple class="hidden" onchange={handleFileInput} />
              </label>
              <p class="text-xs text-gray-500 mt-2">or drag and drop here</p>
            </div>
          {:else}
            <div class="p-4">
              <div class="space-y-2 mb-4 max-h-48 overflow-y-auto">
                {#each files as file, i}
                  <div class="flex items-center gap-2 p-2 bg-gray-800/50 rounded-lg">
                    <span class="text-sm truncate flex-1">{file.name}</span>
                    <span class="text-xs text-gray-500">{formatBytes(file.size)}</span>
                    {#if !isTypeAllowed(file.name)}
                      <span class="text-xs text-red-400">Not allowed</span>
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
              </div>
            </div>
          {/if}
        </div>

        <!-- Progress -->
        {#if uploading}
          <div class="mb-4">
            <div class="flex items-center justify-between text-sm mb-2">
              <span class="text-gray-400">Uploading...</span>
              <span class="text-gray-400">{uploadProgress}%</span>
            </div>
            <div class="w-full h-2 bg-gray-800 rounded-full overflow-hidden">
              <div class="h-full bg-gradient-to-r from-violet-500 to-indigo-500 rounded-full transition-all duration-300" style="width: {uploadProgress}%"></div>
            </div>
          </div>
        {/if}

        <!-- Error -->
        {#if error}
          <div class="mb-4 p-3 bg-red-500/10 border border-red-500/20 rounded-xl text-sm text-red-400">{error}</div>
        {/if}

        <button
          onclick={uploadAll}
          disabled={uploading || files.length === 0}
          class="w-full py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-lg shadow-violet-500/20"
        >
          {uploading ? 'Uploading...' : `Send ${files.length} file${files.length !== 1 ? 's' : ''}`}
        </button>
      </div>
    </div>
  {/if}
</div>
