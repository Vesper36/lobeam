<script>
  import { api } from '$lib/utils/api.js';
  import { timeAgo } from '$lib/utils/helpers.js';

  let me = $state(null);
  let users = $state([]);
  let logs = $state([]);
  let activeTab = $state('users');
  let loading = $state(true);
  let error = $state('');

  // Brand form
  let brand = $state({
    name: 'LoBeam',
    domain: '',
    logo_url: '',
    primary_color: '#7c3aed',
    background_color: '#09090b',
    accent_color: '#4f46e5',
    email_from: '',
    email_footer: '',
    custom_css: '',
    custom_html: '',
    show_powered_by: true,
    default_expiry_hours: 24,
    default_max_downloads: 100,
    max_file_size: 0,
  });
  let brandSaved = $state(false);

  // Settings form
  let settings = $state({
    allow_anonymous: true,
    allow_registration: true,
    max_file_size: 0,
    default_expiry_hours: 24,
    default_max_downloads: 100,
    enable_p2p: true,
    enable_clipboard: true,
  });
  let settingsSaved = $state(false);

  $effect(async () => {
    try {
      me = await api.getMe();
      if (me?.role !== 'admin') {
        error = 'Admin access required';
        loading = false;
        return;
      }
      await loadAll();
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  });

  async function loadAll() {
    const [u, b, s] = await Promise.all([
      api.listUsers(),
      api.getBrand(),
      api.getSettings(),
    ]);
    users = u;
    if (b) brand = { ...brand, ...b };
    if (s) settings = { ...settings, ...s };
  }

  async function loadLogs() {
    try {
      logs = await api.getAuditLogs();
    } catch (err) {
      error = err.message;
    }
  }

  async function deleteUser(id, username) {
    if (!confirm(`Delete user "${username}"? This cannot be undone.`)) return;
    try {
      await api.deleteUser(id);
      users = users.filter(u => u.id !== id);
    } catch (err) {
      error = err.message;
    }
  }

  async function saveBrand() {
    try {
      await api.updateBrand(brand);
      brandSaved = true;
      setTimeout(() => brandSaved = false, 2000);
    } catch (err) {
      error = err.message;
    }
  }

  async function saveSettings() {
    try {
      await api.updateSettings(settings);
      settingsSaved = true;
      setTimeout(() => settingsSaved = false, 2000);
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="max-w-4xl mx-auto">
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
      <h2 class="text-xl font-bold mb-2">{error}</h2>
    </div>

  {:else}
    <div class="mb-8">
      <h1 class="text-2xl font-bold">Admin Panel</h1>
      <p class="text-gray-400 text-sm">Manage users, branding, and system settings</p>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 mb-6 bg-gray-900 rounded-xl p-1">
      {#each [
        { key: 'users', label: 'Users' },
        { key: 'brand', label: 'Brand' },
        { key: 'settings', label: 'Settings' },
        { key: 'logs', label: 'Audit Logs' },
      ] as tab}
        <button
          onclick={() => { activeTab = tab.key; if (tab.key === 'logs') loadLogs(); }}
          class="flex-1 py-2 rounded-lg text-sm font-medium transition-colors {activeTab === tab.key ? 'bg-violet-600 text-white' : 'text-gray-400 hover:text-white'}"
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Users Tab -->
    {#if activeTab === 'users'}
      <div class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div class="divide-y divide-gray-800">
          {#each users as user}
            <div class="flex items-center gap-4 p-4">
              <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center font-bold text-white">
                {user.username[0].toUpperCase()}
              </div>
              <div class="flex-1">
                <p class="font-medium">{user.username}</p>
                <p class="text-xs text-gray-500">{user.email}</p>
              </div>
              <span class="px-2 py-1 rounded text-xs font-medium {user.role === 'admin' ? 'bg-violet-500/10 text-violet-400' : 'bg-gray-800 text-gray-400'}">
                {user.role}
              </span>
              <span class="text-xs text-gray-500">{timeAgo(user.created_at)}</span>
              {#if user.role !== 'admin'}
                <button
                  onclick={() => deleteUser(user.id, user.username)}
                  class="p-1.5 rounded-lg hover:bg-red-500/10 text-gray-500 hover:text-red-400 transition-colors"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
                </button>
              {/if}
            </div>
          {/each}
        </div>
      </div>

    <!-- Brand Tab -->
    {:else if activeTab === 'brand'}
      <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label class="block text-xs text-gray-400 mb-1">Site Name</label>
            <input type="text" bind:value={brand.name} class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Domain</label>
            <input type="text" bind:value={brand.domain} placeholder="files.example.com" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Logo URL</label>
            <input type="text" bind:value={brand.logo_url} placeholder="https://..." class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Primary Color</label>
            <div class="flex gap-2">
              <input type="color" bind:value={brand.primary_color} class="w-10 h-10 rounded border-0 cursor-pointer bg-transparent" />
              <input type="text" bind:value={brand.primary_color} class="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Background Color</label>
            <div class="flex gap-2">
              <input type="color" bind:value={brand.background_color} class="w-10 h-10 rounded border-0 cursor-pointer bg-transparent" />
              <input type="text" bind:value={brand.background_color} class="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Accent Color</label>
            <div class="flex gap-2">
              <input type="color" bind:value={brand.accent_color} class="w-10 h-10 rounded border-0 cursor-pointer bg-transparent" />
              <input type="text" bind:value={brand.accent_color} class="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
            </div>
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Email From</label>
            <input type="text" bind:value={brand.email_from} placeholder="noreply@example.com" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Email Footer</label>
            <input type="text" bind:value={brand.email_footer} placeholder="Sent via LoBeam" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
          <div>
            <label class="block text-xs text-gray-400 mb-1">Max File Size (0 = unlimited)</label>
            <input type="number" bind:value={brand.max_file_size} min="0" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Default Expiry (hours)</label>
            <input type="number" bind:value={brand.default_expiry_hours} min="1" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-400 mb-1">Default Max Downloads</label>
            <input type="number" bind:value={brand.default_max_downloads} min="1" class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500" />
          </div>
        </div>

        <div class="mt-4">
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={brand.show_powered_by} class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-violet-500 focus:ring-violet-500" />
            <span class="text-sm text-gray-300">Show "Powered by" footer</span>
          </label>
        </div>

        <div class="mt-4">
          <label class="block text-xs text-gray-400 mb-1">Custom CSS</label>
          <textarea bind:value={brand.custom_css} rows="4" placeholder={'/* Custom CSS rules */'} class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm font-mono focus:outline-none focus:border-violet-500 resize-none"></textarea>
        </div>

        <button onclick={saveBrand} class="mt-4 px-6 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
          {brandSaved ? 'Saved!' : 'Save Brand Settings'}
        </button>
      </div>

    <!-- Settings Tab -->
    {:else if activeTab === 'settings'}
      <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6">
        <div class="space-y-4">
          <label class="flex items-center justify-between gap-4">
            <div>
              <span class="text-sm font-medium">Allow anonymous uploads</span>
              <p class="text-xs text-gray-500">Users without accounts can upload files</p>
            </div>
            <div class="relative">
              <input type="checkbox" bind:checked={settings.allow_anonymous} class="sr-only peer" />
              <div class="w-9 h-5 bg-gray-700 rounded-full peer-checked:bg-violet-600 peer-focus:ring-2 peer-focus:ring-violet-500 transition-colors"></div>
              <div class="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full peer-checked:translate-x-4 transition-transform"></div>
            </div>
          </label>

          <label class="flex items-center justify-between gap-4">
            <div>
              <span class="text-sm font-medium">Allow registration</span>
              <p class="text-xs text-gray-500">New users can create accounts</p>
            </div>
            <input type="checkbox" bind:checked={settings.allow_registration} class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-violet-500 focus:ring-violet-500" />
          </label>

          <label class="flex items-center justify-between gap-4">
            <div>
              <span class="text-sm font-medium">Enable P2P transfers</span>
              <p class="text-xs text-gray-500">Browser-to-browser direct file transfers</p>
            </div>
            <input type="checkbox" bind:checked={settings.enable_p2p} class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-violet-500 focus:ring-violet-500" />
          </label>

          <label class="flex items-center justify-between gap-4">
            <div>
              <span class="text-sm font-medium">Enable clipboard</span>
              <p class="text-xs text-gray-500">Text/code sharing feature</p>
            </div>
            <input type="checkbox" bind:checked={settings.enable_clipboard} class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-violet-500 focus:ring-violet-500" />
          </label>
        </div>

        <button onclick={saveSettings} class="mt-6 px-6 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
          {settingsSaved ? 'Saved!' : 'Save Settings'}
        </button>
      </div>

    <!-- Audit Logs Tab -->
    {:else if activeTab === 'logs'}
      <div class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        {#if logs.length === 0}
          <div class="p-8 text-center text-gray-400">
            <p>No audit log entries yet</p>
          </div>
        {:else}
          <div class="divide-y divide-gray-800">
            {#each logs as log}
              <div class="p-4">
                <div class="flex items-center gap-3">
                  <span class="text-xs font-mono px-2 py-0.5 rounded {log.action === 'upload' ? 'bg-blue-500/10 text-blue-400' : log.action === 'download' ? 'bg-green-500/10 text-green-400' : log.action === 'delete' ? 'bg-red-500/10 text-red-400' : 'bg-gray-800 text-gray-400'}">
                    {log.action}
                  </span>
                  <p class="text-sm flex-1">{log.details}</p>
                  <span class="text-xs text-gray-500">{timeAgo(log.created_at)}</span>
                </div>
                {#if log.ip_address}
                  <p class="text-xs text-gray-600 mt-1 ml-24">IP: {log.ip_address}</p>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {/if}
</div>