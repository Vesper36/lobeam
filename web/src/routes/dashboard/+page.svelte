<script>
  import { api } from '$lib/utils/api.js';
  import { formatBytes, timeAgo, timeUntil } from '$lib/utils/helpers.js';

  let transfers = $state([]);
  let loading = $state(true);
  let error = $state('');

  $effect(() => {
    loadTransfers();
  });

  async function loadTransfers() {
    try {
      transfers = await api.listTransfers();
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

  function getStatusColor(status) {
    const colors = {
      ready: 'bg-green-500/10 text-green-400',
      pending: 'bg-yellow-500/10 text-yellow-400',
      uploading: 'bg-blue-500/10 text-blue-400',
      expired: 'bg-gray-500/10 text-gray-400',
    };
    return colors[status] || 'bg-gray-500/10 text-gray-400';
  }
</script>

<div class="max-w-4xl mx-auto">
  <div class="flex items-center justify-between mb-8">
    <div>
      <h1 class="text-2xl font-bold">Dashboard</h1>
      <p class="text-sm text-gray-400">Manage your file transfers</p>
    </div>
    <a href="/" class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
      New transfer
    </a>
  </div>

  {#if loading}
    <div class="text-center py-20">
      <div class="w-12 h-12 mx-auto mb-4 border-2 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
      <p class="text-gray-400">Loading transfers...</p>
    </div>

  {:else if error}
    <div class="p-4 bg-red-500/10 border border-red-500/20 rounded-xl text-sm text-red-400">{error}</div>

  {:else if transfers.length === 0}
    <div class="text-center py-20 bg-gray-900 border border-gray-800 rounded-2xl">
      <div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gray-800 flex items-center justify-center">
        <svg class="w-8 h-8 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/></svg>
      </div>
      <h2 class="text-xl font-bold mb-2">No transfers yet</h2>
      <p class="text-gray-400 mb-4">Create your first file transfer</p>
      <a href="/" class="inline-flex px-6 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
        Get started
      </a>
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
                <span class="px-2 py-0.5 rounded text-xs font-medium {getStatusColor(transfer.status)}">
                  {transfer.status}
                </span>
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
                  onclick={() => navigator.clipboard.writeText(`${window.location.origin}/d/${transfer.id}`)}
                  class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium transition-colors"
                >
                  Copy link
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
</div>
