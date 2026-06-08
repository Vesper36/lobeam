<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/utils/api.js';
  import { formatBytes, timeAgo, timeUntil, copyToClipboard, getFileIcon } from '$lib/utils/helpers.js';

  let transfers = $state([]);
  let folders = $state([]);
  let fileRequests = $state([]);
  let loading = $state(true);
  let error = $state('');
  let activeTab = $state('transfers');
  let copied = $state('');

  // Create folder form
  let showCreateFolder = $state(false);
  let folderName = $state('');
  let folderDesc = $state('');
  let folderMode = $state('both');
  let folderPassword = $state('');
  let folderExpiry = $state(30);

  // Create file request form
  let showCreateRequest = $state(false);
  let requestTitle = $state('');
  let requestDesc = $state('');
  let requestMaxSize = $state(0);
  let requestMaxFiles = $state(0);
  let requestExpiry = $state(30);
  let requestAllowedTypes = $state('');
  let requestCustomFields = $state('name,email');
  let requestRequiredFields = $state('email');

  onMount(() => {
    loadAll();
  });

  async function loadAll() {
    try {
      [transfers, folders, fileRequests] = await Promise.all([
        api.listTransfers().catch(() => []),
        api.listWebFolders().catch(() => []),
        api.listFileRequests().catch(() => []),
      ]);
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function deleteTransfer(id) {
    try {
      await api.deleteTransfer(id);
      transfers = transfers.filter(t => t.id !== id);
    } catch (err) {
      error = err.message;
    }
  }

  function copyLink(url, label) {
    copyToClipboard(url);
    copied = label;
    setTimeout(() => copied = '', 2000);
  }

  function parseList(value) {
    return value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }

  async function createFolder() {
    try {
      const res = await api.createWebFolder({
        name: folderName,
        description: folderDesc,
        mode: folderMode,
        password: folderPassword || undefined,
        expiry_days: folderExpiry,
      });
      showCreateFolder = false;
      folderName = '';
      folderDesc = '';
      folders = await api.listWebFolders();
      copyLink(res.url, 'folder-url');
    } catch (err) {
      error = err.message;
    }
  }

  async function createRequest() {
    try {
      const maxFileSizeMB = Number(requestMaxSize) || 0;
      const res = await api.createFileRequest({
        title: requestTitle,
        description: requestDesc,
        max_file_size: maxFileSizeMB > 0 ? Math.round(maxFileSizeMB * 1024 * 1024) : 0,
        max_files: Number(requestMaxFiles) || 0,
        allowed_types: parseList(requestAllowedTypes),
        custom_fields: parseList(requestCustomFields),
        require_fields: parseList(requestRequiredFields),
        expiry_days: Number(requestExpiry) || 30,
      });
      showCreateRequest = false;
      requestTitle = '';
      requestDesc = '';
      requestMaxSize = 0;
      requestMaxFiles = 0;
      requestAllowedTypes = '';
      requestCustomFields = 'name,email';
      requestRequiredFields = 'email';
      fileRequests = await api.listFileRequests();
      copyLink(res.url, 'request-url');
    } catch (err) {
      error = err.message;
    }
  }

  function getStatusColor(status) {
    const colors = {
      ready: 'bg-green-500/10 text-green-400',
      pending: 'bg-yellow-500/10 text-yellow-400',
      uploading: 'bg-blue-500/10 text-blue-400',
      expired: 'bg-gray-500/10 text-gray-400',
      active: 'bg-green-500/10 text-green-400',
    };
    return colors[status] || 'bg-gray-500/10 text-gray-400';
  }
</script>

<div class="max-w-4xl mx-auto">
  <div class="flex items-center justify-between mb-8">
    <div>
      <h1 class="text-2xl font-bold">Dashboard</h1>
      <p class="text-sm text-gray-400">Manage transfers, folders, and file requests</p>
    </div>
    <div class="flex gap-2">
      <a href="/" class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
        New transfer
      </a>
    </div>
  </div>

  {#if loading}
    <div class="text-center py-20">
      <div class="w-12 h-12 mx-auto mb-4 border-2 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
      <p class="text-gray-400">Loading...</p>
    </div>

  {:else}
    <!-- Tabs -->
    <div class="flex gap-1 mb-6 bg-gray-900 rounded-xl p-1">
      {#each [
        { key: 'transfers', label: 'Transfers', count: transfers.length },
        { key: 'folders', label: 'Web Folders', count: folders.length },
        { key: 'requests', label: 'File Requests', count: fileRequests.length },
      ] as tab}
        <button
          onclick={() => activeTab = tab.key}
          class="flex-1 py-2 rounded-lg text-sm font-medium transition-colors {activeTab === tab.key ? 'bg-violet-600 text-white' : 'text-gray-400 hover:text-white'}"
        >
          {tab.label}
          {#if tab.count > 0}
            <span class="ml-1.5 px-1.5 py-0.5 rounded text-xs {activeTab === tab.key ? 'bg-violet-500' : 'bg-gray-800'}">{tab.count}</span>
          {/if}
        </button>
      {/each}
    </div>

    <!-- Transfers Tab -->
    {#if activeTab === 'transfers'}
      {#if transfers.length === 0}
        <div class="text-center py-16 bg-gray-900 border border-gray-800 rounded-2xl">
          <div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gray-800 flex items-center justify-center">
            <svg class="w-8 h-8 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/></svg>
          </div>
          <h2 class="text-xl font-bold mb-2">No transfers yet</h2>
          <a href="/" class="inline-flex px-6 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">Create transfer</a>
        </div>
      {:else}
        <div class="space-y-3">
          {#each transfers as transfer}
            <div class="bg-gray-900 border border-gray-800 rounded-xl p-4 hover:border-gray-700 transition-colors">
              <div class="flex items-center gap-4">
                <div class="w-10 h-10 rounded-lg bg-violet-500/10 flex items-center justify-center">
                  <svg class="w-5 h-5 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/></svg>
                </div>
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <h3 class="font-medium truncate">{transfer.name}</h3>
                    <span class="px-2 py-0.5 rounded text-xs font-medium {getStatusColor(transfer.status)}">{transfer.status}</span>
                    {#if transfer.encrypted}
                      <svg class="w-3.5 h-3.5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
                    {/if}
                  </div>
                  <div class="flex items-center gap-4 mt-1 text-xs text-gray-500">
                    <span>{transfer.file_count} file{transfer.file_count > 1 ? 's' : ''}</span>
                    <span>{formatBytes(transfer.total_size)}</span>
                    <span>{transfer.download_count}/{transfer.max_downloads} downloads</span>
                    <span>{timeUntil(transfer.expires_at)}</span>
                    <span>{timeAgo(transfer.created_at)}</span>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  {#if transfer.status === 'ready'}
                    <button
                      onclick={() => copyLink(`${window.location.origin}/d/${transfer.id}`, `transfer-${transfer.id}`)}
                      class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium transition-colors"
                    >
                      {copied === `transfer-${transfer.id}` ? 'Copied!' : 'Copy link'}
                    </button>
                  {/if}
                  <button
                    onclick={() => deleteTransfer(transfer.id)}
                    class="p-1.5 rounded-lg hover:bg-red-500/10 text-gray-500 hover:text-red-400 transition-colors"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
                  </button>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}

    <!-- Web Folders Tab -->
    {:else if activeTab === 'folders'}
      <div class="mb-4">
        <button onclick={() => showCreateFolder = !showCreateFolder} class="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
          {showCreateFolder ? 'Cancel' : 'Create Web Folder'}
        </button>
      </div>

      {#if showCreateFolder}
        <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6 mb-6">
          <h3 class="font-semibold mb-4">New Web Folder</h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label class="block text-xs text-gray-400 mb-1">Folder name</label>
              <input type="text" bind:value={folderName} placeholder="Project files" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Mode</label>
              <select bind:value={folderMode} class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500">
                <option value="both">Upload & Download</option>
                <option value="upload_only">Upload only (collect files)</option>
                <option value="download_only">Download only (distribute)</option>
              </select>
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Description</label>
              <input type="text" bind:value={folderDesc} placeholder="Optional description" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Password (optional)</label>
              <input type="password" bind:value={folderPassword} placeholder="Protect folder access" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Expiry (days)</label>
              <input type="number" bind:value={folderExpiry} min="1" max="365" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
          </div>
          <button onclick={createFolder} disabled={!folderName} class="mt-4 px-6 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium disabled:opacity-50 transition-colors">
            Create Folder
          </button>
        </div>
      {/if}

      {#if folders.length === 0}
        <div class="text-center py-16 bg-gray-900 border border-gray-800 rounded-2xl">
          <div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gray-800 flex items-center justify-center">
            <svg class="w-8 h-8 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
          </div>
          <p class="text-gray-400">No web folders yet. Create one to collect or share files.</p>
        </div>
      {:else}
        <div class="space-y-3">
          {#each folders as folder}
            <div class="bg-gray-900 border border-gray-800 rounded-xl p-4 hover:border-gray-700 transition-colors">
              <div class="flex items-center gap-4">
                <div class="w-10 h-10 rounded-lg bg-violet-500/10 flex items-center justify-center">
                  <svg class="w-5 h-5 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
                </div>
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <h3 class="font-medium truncate">{folder.name}</h3>
                    <span class="px-2 py-0.5 rounded text-xs bg-gray-800 text-gray-400">{folder.mode}</span>
                    {#if folder.password_hash}
                      <svg class="w-3.5 h-3.5 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
                    {/if}
                  </div>
                  <div class="flex items-center gap-4 mt-1 text-xs text-gray-500">
                    <span>{folder.file_count || 0} files</span>
                    <span>Expires: {timeUntil(folder.expires_at)}</span>
                    <span>{timeAgo(folder.created_at)}</span>
                  </div>
                </div>
                <button
                  onclick={() => copyLink(`${window.location.origin}/f/${folder.token}`, `folder-${folder.id}`)}
                  class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium transition-colors"
                >
                  {copied === `folder-${folder.id}` ? 'Copied!' : 'Copy link'}
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}

    <!-- File Requests Tab -->
    {:else if activeTab === 'requests'}
      <div class="mb-4">
        <button onclick={() => showCreateRequest = !showCreateRequest} class="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
          {showCreateRequest ? 'Cancel' : 'Create File Request'}
        </button>
      </div>

      {#if showCreateRequest}
        <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6 mb-6">
          <h3 class="font-semibold mb-4">New File Request</h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label class="block text-xs text-gray-400 mb-1">Title</label>
              <input type="text" bind:value={requestTitle} placeholder="Project deliverables" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Expiry (days)</label>
              <input type="number" bind:value={requestExpiry} min="1" max="365" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Description</label>
              <input type="text" bind:value={requestDesc} placeholder="What files do you need?" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Max file size (MB, 0 = unlimited)</label>
              <input type="number" bind:value={requestMaxSize} min="0" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Max files (0 = unlimited)</label>
              <input type="number" bind:value={requestMaxFiles} min="0" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Allowed types</label>
              <input type="text" bind:value={requestAllowedTypes} placeholder=".zip,.mp4,.psd or blank for all" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Sender fields</label>
              <input type="text" bind:value={requestCustomFields} placeholder="name,email" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
            <div>
              <label class="block text-xs text-gray-400 mb-1">Required fields</label>
              <input type="text" bind:value={requestRequiredFields} placeholder="email" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
          </div>
          <button onclick={createRequest} disabled={!requestTitle} class="mt-4 px-6 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium disabled:opacity-50 transition-colors">
            Create Request
          </button>
        </div>
      {/if}

      {#if fileRequests.length === 0}
        <div class="text-center py-16 bg-gray-900 border border-gray-800 rounded-2xl">
          <div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gray-800 flex items-center justify-center">
            <svg class="w-8 h-8 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>
          </div>
          <p class="text-gray-400">No file requests yet. Create one to collect files from others.</p>
        </div>
      {:else}
        <div class="space-y-3">
          {#each fileRequests as fr}
            <div class="bg-gray-900 border border-gray-800 rounded-xl p-4 hover:border-gray-700 transition-colors">
              <div class="flex items-center gap-4">
                <div class="w-10 h-10 rounded-lg bg-violet-500/10 flex items-center justify-center">
                  <svg class="w-5 h-5 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>
                </div>
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <h3 class="font-medium truncate">{fr.title}</h3>
                    <span class="px-2 py-0.5 rounded text-xs font-medium {getStatusColor(fr.status)}">{fr.status}</span>
                  </div>
                  <div class="flex items-center gap-4 mt-1 text-xs text-gray-500">
                    {#if fr.max_file_size > 0}<span>Max {formatBytes(fr.max_file_size)}</span>{/if}
                    <span>Expires: {timeUntil(fr.expires_at)}</span>
                    <span>{timeAgo(fr.created_at)}</span>
                  </div>
                </div>
                <button
                  onclick={() => copyLink(`${window.location.origin}/r/${fr.id}`, `request-${fr.id}`)}
                  class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium transition-colors"
                >
                  {copied === `request-${fr.id}` ? 'Copied!' : 'Copy link'}
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    {/if}
  {/if}
</div>
