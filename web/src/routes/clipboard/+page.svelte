<script>
  import { api } from '$lib/utils/api.js';
  import { copyToClipboard } from '$lib/utils/helpers.js';

  let content = $state('');
  let language = $state('');
  let hours = $state(24);
  let encrypted = $state(false);
  let result = $state(null);
  let loading = $state(false);
  let error = $state('');
  let copied = $state(false);
  let viewId = $state('');
  let viewResult = $state(null);
  let viewError = $state('');

  async function create() {
    if (!content.trim()) {
      error = 'Content is required';
      return;
    }
    loading = true;
    error = '';
    try {
      result = await api.createClipboard({ content, language, hours, encrypted });
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function lookup() {
    if (!viewId.trim()) return;
    viewError = '';
    viewResult = null;
    try {
      viewResult = await api.getClipboard(viewId.trim());
    } catch (err) {
      viewError = err.message;
    }
  }

  function copyContent() {
    if (viewResult?.content) {
      copyToClipboard(viewResult.content);
      copied = true;
      setTimeout(() => copied = false, 2000);
    }
  }

  function copyLink(url) {
    copyToClipboard(url);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }
</script>

<div class="max-w-3xl mx-auto space-y-8">
  <!-- Create clipboard -->
  <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6">
    <h2 class="text-xl font-bold mb-4">Network Clipboard</h2>
    <p class="text-sm text-gray-400 mb-6">Share text, code, or notes with anyone via a simple link.</p>

    {#if result}
      <div class="text-center py-6">
        <div class="w-12 h-12 mx-auto mb-3 rounded-full bg-green-500/10 flex items-center justify-center">
          <svg class="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
        </div>
        <p class="text-sm text-gray-400 mb-3">Clipboard created</p>
        <div class="bg-gray-800 rounded-xl p-3 flex items-center gap-2">
          <input type="text" value={result.url} readonly class="flex-1 bg-transparent text-sm text-gray-300 outline-none" />
          <button onclick={() => copyLink(result.url)} class="px-3 py-1.5 rounded-lg bg-violet-600 hover:bg-violet-700 text-xs font-medium transition-colors">
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <button onclick={() => { result = null; content = ''; }} class="mt-3 text-sm text-gray-400 hover:text-white transition-colors">
          Create another
        </button>
      </div>
    {:else}
      <textarea
        bind:value={content}
        placeholder="Paste your text, code, or notes here..."
        rows="10"
        class="w-full px-4 py-3 bg-gray-800 border border-gray-700 rounded-xl text-sm font-mono focus:outline-none focus:border-violet-500 resize-y mb-4"
      ></textarea>

      <div class="flex flex-wrap items-center gap-4 mb-4">
        <div>
          <label class="block text-xs text-gray-400 mb-1">Language</label>
          <select bind:value={language} class="px-3 py-1.5 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500">
            <option value="">Plain text</option>
            <option value="javascript">JavaScript</option>
            <option value="python">Python</option>
            <option value="go">Go</option>
            <option value="rust">Rust</option>
            <option value="html">HTML</option>
            <option value="css">CSS</option>
            <option value="json">JSON</option>
            <option value="yaml">YAML</option>
            <option value="bash">Bash</option>
            <option value="sql">SQL</option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-gray-400 mb-1">Expires after</label>
          <select bind:value={hours} class="px-3 py-1.5 bg-gray-800 border border-gray-700 rounded-lg text-sm focus:outline-none focus:border-violet-500">
            <option value={1}>1 hour</option>
            <option value={6}>6 hours</option>
            <option value={24}>24 hours</option>
            <option value={168}>7 days</option>
            <option value={720}>30 days</option>
          </select>
        </div>
      </div>

      {#if error}
        <div class="mb-4 p-3 bg-red-500/10 border border-red-500/20 rounded-xl text-sm text-red-400">{error}</div>
      {/if}

      <button onclick={create} disabled={loading || !content.trim()} class="w-full py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 transition-all">
        {loading ? 'Creating...' : 'Create clipboard'}
      </button>
    {/if}
  </div>

  <!-- Lookup clipboard -->
  <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6">
    <h2 class="text-xl font-bold mb-4">View clipboard</h2>
    <div class="flex gap-2">
      <input
        type="text"
        bind:value={viewId}
        placeholder="Enter clipboard ID..."
        class="flex-1 px-4 py-2.5 bg-gray-800 border border-gray-700 rounded-xl text-sm focus:outline-none focus:border-violet-500"
        onkeydown={(e) => e.key === 'Enter' && lookup()}
      />
      <button onclick={lookup} class="px-6 py-2.5 rounded-xl bg-gray-800 hover:bg-gray-700 text-sm font-medium transition-colors">
        Lookup
      </button>
    </div>

    {#if viewError}
      <p class="mt-3 text-sm text-red-400">{viewError}</p>
    {/if}

    {#if viewResult}
      <div class="mt-4">
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs text-gray-500">{viewResult.language || 'plain text'}</span>
          <button onclick={copyContent} class="text-xs text-violet-400 hover:text-violet-300 transition-colors">
            {copied ? 'Copied!' : 'Copy content'}
          </button>
        </div>
        <pre class="p-4 bg-gray-800 rounded-xl text-sm font-mono overflow-x-auto whitespace-pre-wrap break-all">{viewResult.content}</pre>
      </div>
    {/if}
  </div>
</div>
