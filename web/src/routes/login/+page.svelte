<script>
  import { api } from '$lib/utils/api.js';
  import { goto } from '$app/navigation';

  let username = $state('');
  let password = $state('');
  let email = $state('');
  let isRegister = $state(false);
  let error = $state('');
  let loading = $state(false);

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