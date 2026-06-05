<script>
  import '../app.css';

  let { children } = $props();

  let darkMode = $state(true);
  let mobileMenuOpen = $state(false);

  function toggleDark() {
    darkMode = !darkMode;
    document.documentElement.classList.toggle('dark', darkMode);
  }
</script>

<div class="min-h-screen bg-gray-950 text-gray-100">
  <!-- Nav -->
  <nav class="border-b border-gray-800 bg-gray-950/80 backdrop-blur-xl sticky top-0 z-50">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between h-16">
        <!-- Logo -->
        <a href="/" class="flex items-center gap-3 group">
          <div class="w-9 h-9 rounded-xl bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center font-bold text-lg shadow-lg shadow-violet-500/20 group-hover:shadow-violet-500/40 transition-shadow">
            M
          </div>
          <span class="text-xl font-bold tracking-tight">LoBeam</span>
        </a>

        <!-- Desktop nav -->
        <div class="hidden md:flex items-center gap-1">
          <a href="/" class="px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800 transition-colors">
            Transfer
          </a>
          <a href="/clipboard" class="px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800 transition-colors">
            Clipboard
          </a>
          <a href="/p2p" class="px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800 transition-colors">
            P2P
          </a>
          <a href="/dashboard" class="px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800 transition-colors">
            Dashboard
          </a>
        </div>

        <!-- Right side -->
        <div class="flex items-center gap-3">
          <a href="/admin" class="text-xs text-gray-500 hover:text-gray-300 transition-colors hidden sm:block">Admin</a>
          <button onclick={toggleDark} class="p-2 rounded-lg hover:bg-gray-800 transition-colors" aria-label="Toggle dark mode">
            {#if darkMode}
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/></svg>
            {:else}
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/></svg>
            {/if}
          </button>

          <!-- Mobile menu button -->
          <button onclick={() => mobileMenuOpen = !mobileMenuOpen} class="md:hidden p-2 rounded-lg hover:bg-gray-800 transition-colors">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/></svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Mobile menu -->
    {#if mobileMenuOpen}
      <div class="md:hidden border-t border-gray-800">
        <div class="px-4 py-3 space-y-1">
          <a href="/" class="block px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800">Transfer</a>
          <a href="/clipboard" class="block px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800">Clipboard</a>
          <a href="/p2p" class="block px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800">P2P</a>
          <a href="/dashboard" class="block px-4 py-2 rounded-lg text-sm font-medium text-gray-300 hover:text-white hover:bg-gray-800">Dashboard</a>
        </div>
      </div>
    {/if}
  </nav>

  <!-- Main content -->
  <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    {@render children()}
  </main>

  <!-- Footer -->
  <footer class="border-t border-gray-800 mt-auto">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      <div class="flex flex-col sm:flex-row items-center justify-between gap-4 text-sm text-gray-500">
        <div class="flex items-center gap-2">
          <div class="w-6 h-6 rounded-lg bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center font-bold text-xs">L</div>
          <span>LoBeam - Self-hosted file transfer</span>
        </div>
        <div class="flex items-center gap-4">
          <a href="https://github.com/Vesper36/lobeam" class="hover:text-gray-300 transition-colors">GitHub</a>
          <span>v1.0.0</span>
        </div>
      </div>
    </div>
  </footer>
</div>
