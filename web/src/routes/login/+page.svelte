<script>
  import { api } from '$lib/utils/api.js';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  let username = $state('');
  let password = $state('');
  let email = $state('');
  let isRegister = $state(false);
  let error = $state('');
  let loading = $state(false);
  let oidcProviders = $state([]);

  const PROVIDER_ICONS = {
    google: 'M12.545,10.239v3.821h5.445c-0.712,2.315-2.647,3.972-5.445,3.972c-3.332,0-6.033-2.701-6.033-6.032s2.701-6.032,6.033-6.032c1.498,0,2.866,0.549,3.921,1.453l2.814-2.814C17.503,2.988,15.139,2,12.545,2C7.021,2,2.543,6.477,2.543,12s4.478,10,10.002,10c8.396,0,10.249-7.85,9.426-11.748L12.545,10.239z',
    github: 'M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z',
    microsoft: 'M11.4 24H0V12.6h11.4V24zM24 24H12.6V12.6H24V24zM11.4 11.4H0V0h11.4v11.4zM24 11.4H12.6V0H24v11.4z',
  };

  onMount(async () => {
    try {
      const res = await fetch('/api/auth/oidc/providers');
      if (res.ok) oidcProviders = await res.json();
    } catch {}
  });

  function loginWithProvider(providerName) {
    window.location.href = `/api/auth/oidc/${providerName}`;
  }

  async function handleSubmit() {
    if (!username || !password) { error = 'Username and password required'; return; }
    if (isRegister && !email) { error = 'Email required for registration'; return; }
    if (password.length < 8) { error = 'Password must be at least 8 characters'; return; }

    loading = true;
    error = '';
    try {
      let res;
      if (isRegister) {
        res = await api.register(username, email, password);
      } else {
        res = await api.login(username, password);
      }
      localStorage.setItem('access_token', res.access_token);
      if (res.refresh_token) localStorage.setItem('refresh_token', res.refresh_token);
      goto('/dashboard');
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="max-w-md mx-auto mt-20">
  <div class="text-center mb-8">
    <div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center font-bold text-2xl text-white shadow-lg shadow-violet-500/20">
      L
    </div>
    <h1 class="text-2xl font-bold">{isRegister ? 'Create account' : 'Sign in'}</h1>
    <p class="text-gray-400 text-sm mt-1">{isRegister ? 'Join LoBeam to manage transfers' : 'Access your transfers and settings'}</p>
  </div>

  <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6">
    <!-- SSO Providers -->
    {#if oidcProviders.length > 0}
      <div class="space-y-2 mb-5">
        {#each oidcProviders as provider}
          <button onclick={() => loginWithProvider(provider.name)}
            class="w-full flex items-center justify-center gap-3 py-2.5 px-4 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-xl text-sm font-medium transition-colors">
            {#if PROVIDER_ICONS[provider.name]}
              <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                <path d={PROVIDER_ICONS[provider.name]}/>
              </svg>
            {:else}
              <svg class="w-5 h-5 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9"/></svg>
            {/if}
            Continue with {provider.display_name || provider.name}
          </button>
        {/each}
      </div>

      <div class="relative mb-5">
        <div class="absolute inset-0 flex items-center"><div class="w-full border-t border-gray-700"></div></div>
        <div class="relative flex justify-center text-xs"><span class="bg-gray-900 px-3 text-gray-500">or continue with password</span></div>
      </div>
    {/if}

    <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-4">
      <div>
        <label class="block text-xs text-gray-400 mb-1">Username</label>
        <input type="text" bind:value={username} placeholder="alice" autofocus
          class="w-full px-3 py-2.5 bg-gray-800 border border-gray-700 rounded-xl text-sm focus:outline-none focus:border-violet-500 transition-colors" />
      </div>

      {#if isRegister}
        <div>
          <label class="block text-xs text-gray-400 mb-1">Email</label>
          <input type="email" bind:value={email} placeholder="alice@example.com"
            class="w-full px-3 py-2.5 bg-gray-800 border border-gray-700 rounded-xl text-sm focus:outline-none focus:border-violet-500 transition-colors" />
        </div>
      {/if}

      <div>
        <label class="block text-xs text-gray-400 mb-1">Password</label>
        <input type="password" bind:value={password} placeholder="Min 8 characters"
          class="w-full px-3 py-2.5 bg-gray-800 border border-gray-700 rounded-xl text-sm focus:outline-none focus:border-violet-500 transition-colors" />
      </div>

      {#if error}
        <div class="p-3 bg-red-500/10 border border-red-500/20 rounded-xl text-sm text-red-400">{error}</div>
      {/if}

      <button type="submit" disabled={loading}
        class="w-full py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 transition-all shadow-lg shadow-violet-500/20">
        {loading ? 'Please wait...' : isRegister ? 'Create account' : 'Sign in'}
      </button>
    </form>

    <div class="mt-4 pt-4 border-t border-gray-800 text-center">
      <button onclick={() => { isRegister = !isRegister; error = ''; }}
        class="text-sm text-violet-400 hover:text-violet-300 transition-colors">
        {isRegister ? 'Already have an account? Sign in' : "Don't have an account? Register"}
      </button>
    </div>
  </div>
</div>