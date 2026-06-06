<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';

  let status = $state('Processing authentication...');
  let error = $state('');

  onMount(() => {
    const hash = window.location.hash;
    if (!hash) {
      error = 'No authentication data received';
      return;
    }

    const params = new URLSearchParams(hash.substring(1));
    const accessToken = params.get('access_token');
    const refreshToken = params.get('refresh_token');

    if (!accessToken) {
      error = 'Missing access token';
      return;
    }

    localStorage.setItem('access_token', accessToken);
    if (refreshToken) {
      localStorage.setItem('refresh_token', refreshToken);
    }

    status = 'Login successful! Redirecting...';

    // Clean URL hash
    window.history.replaceState(null, '', '/auth/callback');

    // Redirect to dashboard after short delay
    setTimeout(() => goto('/dashboard'), 800);
  });
</script>

<div class="max-w-md mx-auto text-center py-20">
  {#if error}
    <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-red-500/10 flex items-center justify-center">
      <svg class="w-8 h-8 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
    </div>
    <h1 class="text-xl font-bold mb-2">Authentication Failed</h1>
    <p class="text-gray-400 mb-6">{error}</p>
    <a href="/login" class="px-6 py-2 rounded-xl bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
      Back to login
    </a>
  {:else}
    <div class="w-12 h-12 mx-auto mb-4 border-2 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
    <p class="text-gray-300">{status}</p>
  {/if}
</div>
