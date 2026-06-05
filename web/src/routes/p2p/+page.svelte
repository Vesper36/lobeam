<script>
  import { api } from '$lib/utils/api.js';
  import { copyToClipboard } from '$lib/utils/helpers.js';

  let mode = $state('send'); // send or receive
  let code = $state('');
  let session = $state(null);
  let error = $state('');
  let loading = $state(false);
  let copied = $state(false);
  let ws = $state(null);
  let messages = $state([]);
  let connected = $state(false);

  async function createSession() {
    loading = true;
    error = '';
    try {
      const res = await api.createP2P();
      session = res;
      code = res.code;
      connectWS(res.code);
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function joinSession() {
    if (!code.trim()) {
      error = 'Please enter a code';
      return;
    }
    loading = true;
    error = '';
    try {
      const res = await api.getP2P(code.trim());
      session = res;
      connectWS(code.trim());
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  function connectWS(roomCode) {
    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/p2p/ws/${roomCode}`;
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      connected = true;
      messages = [...messages, { type: 'system', text: 'Connected to peer' }];
    };

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === 'signal') {
          messages = [...messages, { type: 'signal', text: 'Peer signal received' }];
        } else if (msg.type === 'chat') {
          messages = [...messages, { type: 'chat', text: msg.payload }];
        }
      } catch {}
    };

    ws.onclose = () => {
      connected = false;
      messages = [...messages, { type: 'system', text: 'Disconnected' }];
    };
  }

  function copyCode() {
    if (session?.code) {
      copyToClipboard(session.code);
      copied = true;
      setTimeout(() => copied = false, 2000);
    }
  }

  function copyUrl() {
    if (session?.url) {
      copyToClipboard(session.url);
      copied = true;
      setTimeout(() => copied = false, 2000);
    }
  }
</script>

<div class="max-w-3xl mx-auto">
  <div class="text-center mb-10">
    <h1 class="text-3xl font-bold mb-3">Peer-to-Peer Transfer</h1>
    <p class="text-gray-400">Direct file transfer via WebRTC. Files never touch the server.</p>
  </div>

  <!-- Mode selector -->
  <div class="flex gap-2 mb-8 p-1 bg-gray-900 rounded-xl">
    <button
      onclick={() => { mode = 'send'; session = null; error = ''; }}
      class="flex-1 py-2.5 rounded-lg text-sm font-medium transition-colors {mode === 'send' ? 'bg-violet-600 text-white' : 'text-gray-400 hover:text-white'}"
    >
      Send files
    </button>
    <button
      onclick={() => { mode = 'receive'; session = null; error = ''; }}
      class="flex-1 py-2.5 rounded-lg text-sm font-medium transition-colors {mode === 'receive' ? 'bg-violet-600 text-white' : 'text-gray-400 hover:text-white'}"
    >
      Receive files
    </button>
  </div>

  {#if mode === 'send'}
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6">
      {#if !session}
        <div class="text-center py-8">
          <div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-violet-500/10 flex items-center justify-center">
            <svg class="w-8 h-8 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z"/></svg>
          </div>
          <h2 class="text-xl font-bold mb-2">Create a transfer room</h2>
          <p class="text-sm text-gray-400 mb-6">Get a 6-digit code to share with the receiver</p>
          <button onclick={createSession} disabled={loading} class="px-8 py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 transition-all">
            {loading ? 'Creating...' : 'Generate code'}
          </button>
        </div>
      {:else}
        <div class="text-center py-6">
          <p class="text-sm text-gray-400 mb-4">Share this code with the receiver</p>
          <div class="text-5xl font-mono font-bold tracking-[0.3em] text-violet-400 mb-4">{session.code}</div>
          <div class="flex justify-center gap-3 mb-6">
            <button onclick={copyCode} class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm font-medium transition-colors">
              {copied ? 'Copied!' : 'Copy code'}
            </button>
            <button onclick={copyUrl} class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm font-medium transition-colors">
              Copy link
            </button>
          </div>

          <div class="flex items-center justify-center gap-2 text-sm {connected ? 'text-green-400' : 'text-yellow-400'}">
            <div class="w-2 h-2 rounded-full {connected ? 'bg-green-400' : 'bg-yellow-400 animate-pulse'}"></div>
            {connected ? 'Peer connected - ready to transfer' : 'Waiting for peer...'}
          </div>
        </div>
      {/if}
    </div>

  {:else}
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6">
      <div class="text-center py-4">
        <h2 class="text-xl font-bold mb-2">Enter transfer code</h2>
        <p class="text-sm text-gray-400 mb-6">Enter the 6-digit code from the sender</p>

        <div class="flex gap-2 max-w-xs mx-auto mb-4">
          <input
            type="text"
            bind:value={code}
            placeholder="000000"
            maxlength="6"
            class="flex-1 px-4 py-3 bg-gray-800 border border-gray-700 rounded-xl text-center text-2xl font-mono tracking-[0.3em] focus:outline-none focus:border-violet-500"
            onkeydown={(e) => e.key === 'Enter' && joinSession()}
          />
        </div>
        <button onclick={joinSession} disabled={loading || code.length < 6} class="px-8 py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 transition-all">
          {loading ? 'Connecting...' : 'Connect'}
        </button>
      </div>
    </div>
  {/if}

  {#if error}
    <div class="mt-4 p-3 bg-red-500/10 border border-red-500/20 rounded-xl text-sm text-red-400">{error}</div>
  {/if}

  {#if session && connected}
    <div class="mt-6 bg-gray-900 border border-gray-800 rounded-2xl p-6">
      <h3 class="font-bold mb-4">Transfer status</h3>
      <div class="space-y-2">
        {#each messages as msg}
          <div class="text-sm {msg.type === 'system' ? 'text-gray-500' : 'text-gray-300'}">
            {msg.text}
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>
