<script>
  import { api } from '$lib/utils/api.js';
  import { copyToClipboard, formatBytes, formatSpeed } from '$lib/utils/helpers.js';

  let mode = $state('send');
  let code = $state('');
  let session = $state(null);
  let error = $state('');
  let loading = $state(false);
  let copied = $state(false);
  let ws = $state(null);
  let messages = $state([]);
  let connected = $state(false);
  let peerConnection = $state(null);
  let dataChannel = $state(null);
  let sendFiles = $state([]);
  let sending = $state(false);
  let sendProgress = $state(0);
  let sendSpeed = $state(0);
  let receivedFiles = $state([]);
  let currentReceive = $state(null);
  let isCaller = $state(false);

  // Media sharing
  let mediaMode = $state('none'); // none, screen, camera, both
  let localStream = $state(null);
  let remoteStream = $state(null);
  let localVideoEl = $state(null);
  let remoteVideoEl = $state(null);

  const ICE_SERVERS = [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
  ];

  async function createSession() {
    loading = true;
    error = '';
    try {
      const res = await api.createP2P();
      session = res;
      code = res.code;
      isCaller = true;
      connectWS(res.code);
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function joinSession() {
    if (!code.trim()) { error = 'Please enter a code'; return; }
    loading = true;
    error = '';
    try {
      // Try to get session info, but connect regardless
      try {
        const res = await api.getP2P(code.trim());
        session = res;
      } catch {
        // Session may have expired in DB -- connect anyway via WebSocket
        session = { code: code.trim() };
      }
      isCaller = false;
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
      addMsg('system', 'Connected to signaling server');
      setupPeer();
    };
    ws.onmessage = async (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === 'signal' && msg.payload) {
          const sig = msg.payload;
          if (sig.type === 'offer') {
            await peerConnection.setRemoteDescription(new RTCSessionDescription(sig));
            const answer = await peerConnection.createAnswer();
            await peerConnection.setLocalDescription(answer);
            sendSignal(answer);
          } else if (sig.type === 'answer') {
            await peerConnection.setRemoteDescription(new RTCSessionDescription(sig));
          } else if (sig.candidate) {
            await peerConnection.addIceCandidate(new RTCIceCandidate(sig));
          }
        }
      } catch {}
    };
    ws.onclose = () => { connected = false; addMsg('system', 'Disconnected'); };
  }

  function setupPeer() {
    peerConnection = new RTCPeerConnection({ iceServers: ICE_SERVERS });

    peerConnection.onicecandidate = (e) => {
      if (e.candidate) sendSignal(e.candidate);
    };

    peerConnection.onconnectionstatechange = () => {
      if (peerConnection.connectionState === 'connected') {
        addMsg('system', 'Peer connected');
      }
    };

    // Handle incoming data channel (receiver side)
    peerConnection.ondatachannel = (e) => {
      dataChannel = e.channel;
      setupDataChannel();
    };

    // Handle incoming media tracks
    peerConnection.ontrack = (e) => {
      if (e.streams && e.streams[0]) {
        remoteStream = e.streams[0];
        if (remoteVideoEl) {
          remoteVideoEl.srcObject = remoteStream;
        }
      }
    };

    // If caller, create data channel and send offer
    if (isCaller) {
      dataChannel = peerConnection.createDataChannel('files', { ordered: true });
      setupDataChannel();
      createAndSendOffer();
    }
  }

  async function createAndSendOffer() {
    const offer = await peerConnection.createOffer();
    await peerConnection.setLocalDescription(offer);
    sendSignal(offer);
  }

  function setupDataChannel() {
    if (!dataChannel) return;
    dataChannel.onopen = () => addMsg('system', 'Data channel open');
    dataChannel.onclose = () => addMsg('system', 'Data channel closed');
    dataChannel.onmessage = (e) => handleDataMessage(e.data);
  }

  function handleDataMessage(data) {
    if (typeof data === 'string') {
      const msg = JSON.parse(data);
      if (msg.type === 'file-meta') {
        currentReceive = { name: msg.name, size: msg.size, mime: msg.mime, chunks: [], received: 0 };
      } else if (msg.type === 'file-done') {
        if (currentReceive) {
          const blob = new Blob(currentReceive.chunks, { type: currentReceive.mime });
          const url = URL.createObjectURL(blob);
          receivedFiles = [...receivedFiles, { name: currentReceive.name, size: currentReceive.size, url }];
          addMsg('file', `Received: ${currentReceive.name} (${formatBytes(currentReceive.size)})`);
          currentReceive = null;
        }
      }
    } else if (currentReceive) {
      currentReceive.chunks.push(data);
      currentReceive.received += data.byteLength;
    }
  }

  function sendSignal(payload) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'signal', payload }));
    }
  }

  async function sendFilesOverP2P() {
    if (!dataChannel || dataChannel.readyState !== 'open' || sendFiles.length === 0) return;
    sending = true;
    sendProgress = 0;
    const totalSize = sendFiles.reduce((s, f) => s + f.size, 0);
    let sent = 0;
    const startTime = Date.now();
    const CHUNK = 16384; // 16KB for WebRTC

    for (const file of sendFiles) {
      dataChannel.send(JSON.stringify({ type: 'file-meta', name: file.name, size: file.size, mime: file.type }));
      let offset = 0;
      while (offset < file.size) {
        const slice = file.slice(offset, offset + CHUNK);
        const buf = await slice.arrayBuffer();
        // Wait if buffer full
        while (dataChannel.bufferedAmount > 65536) {
          await new Promise(r => setTimeout(r, 50));
        }
        dataChannel.send(buf);
        offset += buf.byteLength;
        sent += buf.byteLength;
        sendProgress = Math.round((sent / totalSize) * 100);
        const elapsed = (Date.now() - startTime) / 1000;
        sendSpeed = sent / elapsed;
      }
      dataChannel.send(JSON.stringify({ type: 'file-done' }));
      addMsg('file', `Sent: ${file.name} (${formatBytes(file.size)})`);
    }
    sending = false;
    sendFiles = [];
  }

  function handleSendInput(e) { sendFiles = [...sendFiles, ...Array.from(e.target.files)]; }
  function removeSendFile(i) { sendFiles = sendFiles.filter((_, idx) => idx !== i); }

  // ---- Media sharing ----
  async function startScreenShare() {
    try {
      localStream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true });
      mediaMode = 'screen';
      localVideoEl.srcObject = localStream;
      localStream.getVideoTracks()[0].onended = () => stopMedia();
      // Add tracks to peer connection
      if (peerConnection) {
        localStream.getTracks().forEach(t => peerConnection.addTrack(t, localStream));
        if (isCaller) { await createAndSendOffer(); }
      }
    } catch (err) { error = 'Screen share cancelled'; }
  }

  async function startCameraShare() {
    try {
      localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
      mediaMode = 'camera';
      localVideoEl.srcObject = localStream;
      if (peerConnection) {
        localStream.getTracks().forEach(t => peerConnection.addTrack(t, localStream));
        if (isCaller) { await createAndSendOffer(); }
      }
    } catch (err) { error = 'Camera access denied'; }
  }

  function stopMedia() {
    if (localStream) { localStream.getTracks().forEach(t => t.stop()); localStream = null; }
    mediaMode = 'none';
  }

  function copyCode() {
    if (session?.code) { copyToClipboard(session.code); copied = true; setTimeout(() => copied = false, 2000); }
  }
  function copyUrl() {
    if (session?.url) { copyToClipboard(session.url); copied = true; setTimeout(() => copied = false, 2000); }
  }
  function addMsg(type, text) { messages = [...messages, { type, text }]; }
</script>

<div class="max-w-3xl mx-auto">
  <div class="text-center mb-10">
    <h1 class="text-3xl font-bold mb-3">Peer-to-Peer Transfer</h1>
    <p class="text-gray-400">Direct file transfer + screen sharing via WebRTC. Files never touch the server.</p>
  </div>

  <div class="flex gap-2 mb-8 p-1 bg-gray-900 rounded-xl">
    <button onclick={() => { mode = 'send'; session = null; error = ''; }}
      class="flex-1 py-2.5 rounded-lg text-sm font-medium transition-colors {mode === 'send' ? 'bg-violet-600 text-white' : 'text-gray-400 hover:text-white'}">
      Send files
    </button>
    <button onclick={() => { mode = 'receive'; session = null; error = ''; }}
      class="flex-1 py-2.5 rounded-lg text-sm font-medium transition-colors {mode === 'receive' ? 'bg-violet-600 text-white' : 'text-gray-400 hover:text-white'}">
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
          <button onclick={createSession} disabled={loading}
            class="px-8 py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 transition-all">
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
            {connected ? 'Peer connected' : 'Waiting for peer...'}
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
          <input type="text" bind:value={code} placeholder="000000" maxlength="6"
            class="flex-1 px-4 py-3 bg-gray-800 border border-gray-700 rounded-xl text-center text-2xl font-mono tracking-[0.3em] focus:outline-none focus:border-violet-500"
            onkeydown={(e) => e.key === 'Enter' && joinSession()} />
        </div>
        <button onclick={joinSession} disabled={loading || code.length < 6}
          class="px-8 py-3 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white font-semibold disabled:opacity-50 transition-all">
          {loading ? 'Connecting...' : 'Connect'}
        </button>
      </div>
    </div>
  {/if}

  {#if error}
    <div class="mt-4 p-3 bg-red-500/10 border border-red-500/20 rounded-xl text-sm text-red-400">{error}</div>
  {/if}

  {#if session}
    <!-- Media sharing controls -->
    <div class="mt-6 bg-gray-900 border border-gray-800 rounded-2xl p-6">
      <h3 class="font-bold mb-4">Screen / Camera Sharing</h3>
      <div class="flex gap-2 mb-4">
        {#if mediaMode === 'none'}
          <button onclick={startScreenShare} class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm font-medium transition-colors flex items-center gap-2">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>
            Share screen
          </button>
          <button onclick={startCameraShare} class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm font-medium transition-colors flex items-center gap-2">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>
            Camera
          </button>
        {:else}
          <button onclick={stopMedia} class="px-4 py-2 rounded-lg bg-red-500/20 hover:bg-red-500/30 text-red-400 text-sm font-medium transition-colors">
            Stop sharing
          </button>
          <span class="text-sm text-gray-400 py-2">Sharing: {mediaMode}</span>
        {/if}
      </div>
      <!-- Video elements -->
      {#if mediaMode !== 'none'}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="rounded-xl overflow-hidden bg-black aspect-video">
            <p class="text-xs text-gray-500 p-2">You</p>
            <video bind:this={localVideoEl} autoplay muted playsinline class="w-full h-auto"></video>
          </div>
          {#if remoteStream}
            <div class="rounded-xl overflow-hidden bg-black aspect-video">
              <p class="text-xs text-gray-500 p-2">Peer</p>
              <video bind:this={remoteVideoEl} autoplay playsinline class="w-full h-auto"></video>
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <!-- File transfer controls -->
    <div class="mt-6 bg-gray-900 border border-gray-800 rounded-2xl p-6">
      <h3 class="font-bold mb-4">File Transfer</h3>

      <div class="mb-4">
        <label class="cursor-pointer inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium transition-colors">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
          Select files
          <input type="file" multiple class="hidden" onchange={handleSendInput} />
        </label>
      </div>

      {#if sendFiles.length > 0}
        <div class="space-y-2 mb-4 max-h-40 overflow-y-auto">
          {#each sendFiles as file, i}
            <div class="flex items-center gap-2 p-2 bg-gray-800/50 rounded-lg">
              <span class="text-sm truncate flex-1">{file.name}</span>
              <span class="text-xs text-gray-500">{formatBytes(file.size)}</span>
              <button onclick={() => removeSendFile(i)} class="text-gray-500 hover:text-white">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
              </button>
            </div>
          {/each}
        </div>

        {#if sending}
          <div class="mb-4">
            <div class="flex justify-between text-sm mb-1">
              <span class="text-gray-400">Sending...</span>
              <span class="text-gray-400">{sendProgress}% ({formatSpeed(sendSpeed)})</span>
            </div>
            <div class="w-full h-2 bg-gray-800 rounded-full overflow-hidden">
              <div class="h-full bg-gradient-to-r from-violet-500 to-indigo-500 rounded-full transition-all" style="width: {sendProgress}%"></div>
            </div>
          </div>
        {/if}

        <button onclick={sendFilesOverP2P} disabled={sending}
          class="px-6 py-2 rounded-lg bg-violet-600 hover:bg-violet-700 text-sm font-medium disabled:opacity-50 transition-colors">
          {sending ? 'Sending...' : `Send ${sendFiles.length} file${sendFiles.length > 1 ? 's' : ''}`}
        </button>
      {/if}

      {#if receivedFiles.length > 0}
        <div class="mt-4 pt-4 border-t border-gray-800">
          <h4 class="text-sm font-medium text-gray-400 mb-2">Received files</h4>
          {#each receivedFiles as rf}
            <div class="flex items-center gap-2 p-2 bg-gray-800/50 rounded-lg mb-2">
              <span class="text-sm flex-1">{rf.name}</span>
              <span class="text-xs text-gray-500">{formatBytes(rf.size)}</span>
              <a href={rf.url} download={rf.name} class="px-3 py-1 rounded bg-violet-600 hover:bg-violet-700 text-xs font-medium transition-colors">Save</a>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  <!-- Activity log -->
  {#if messages.length > 0}
    <div class="mt-6 bg-gray-900 border border-gray-800 rounded-2xl p-6">
      <h3 class="font-bold mb-3">Activity</h3>
      <div class="space-y-1 max-h-40 overflow-y-auto">
        {#each messages as msg}
          <p class="text-xs {msg.type === 'system' ? 'text-gray-500' : msg.type === 'file' ? 'text-green-400' : 'text-gray-300'}">
            {msg.text}
          </p>
        {/each}
      </div>
    </div>
  {/if}
</div>