// LoBeam Chrome Extension - Popup Script

const CHUNK_SIZE = 5 * 1024 * 1024; // 5MB
let selectedFiles = [];

// Tab switching
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
    tab.classList.add('active');
    document.getElementById(tab.dataset.tab).classList.add('active');
  });
});

// Load settings on open
document.addEventListener('DOMContentLoaded', async () => {
  const config = await getConfig();
  document.getElementById('serverUrl').value = config.serverUrl || '';
  document.getElementById('apiToken').value = config.apiToken || '';
  updateConnectionStatus(config.serverUrl);
});

function getConfig() {
  return new Promise(resolve => {
    chrome.storage.local.get(['serverUrl', 'apiToken'], r => {
      resolve({ serverUrl: r.serverUrl || '', apiToken: r.apiToken || '' });
    });
  });
}

function updateConnectionStatus(url) {
  const el = document.getElementById('status');
  if (url) {
    el.textContent = 'Connected';
    el.className = 'connected';
  } else {
    el.textContent = 'Not configured';
    el.style.color = '#f87171';
    el.style.fontSize = '11px';
  }
}

// ---- Settings ----
document.getElementById('saveBtn').addEventListener('click', async () => {
  const serverUrl = document.getElementById('serverUrl').value.trim().replace(/\/+$/, '');
  const apiToken = document.getElementById('apiToken').value.trim();
  chrome.storage.local.set({ serverUrl, apiToken }, () => {
    const el = document.getElementById('saveStatus');
    el.style.display = 'block';
    setTimeout(() => el.style.display = 'none', 2000);
    updateConnectionStatus(serverUrl);
  });
});

// ---- File Upload ----
const dropZone = document.getElementById('dropZone');
const fileInput = document.getElementById('fileInput');

dropZone.addEventListener('click', () => fileInput.click());
dropZone.addEventListener('dragover', e => { e.preventDefault(); dropZone.classList.add('dragover'); });
dropZone.addEventListener('dragleave', () => dropZone.classList.remove('dragover'));
dropZone.addEventListener('drop', e => {
  e.preventDefault();
  dropZone.classList.remove('dragover');
  addFiles(Array.from(e.dataTransfer.files));
});
fileInput.addEventListener('change', () => addFiles(Array.from(fileInput.files)));

function addFiles(files) {
  selectedFiles = [...selectedFiles, ...files];
  renderFileList();
  document.getElementById('uploadOptions').style.display = 'block';
  document.getElementById('uploadBtn').style.display = 'block';
}

function renderFileList() {
  const el = document.getElementById('fileList');
  el.innerHTML = selectedFiles.map((f, i) => `
    <div class="file-item">
      <span class="name">${f.name}</span>
      <span class="size">${formatBytes(f.size)}</span>
      <span class="remove" data-i="${i}">&times;</span>
    </div>
  `).join('');
  el.querySelectorAll('.remove').forEach(btn => {
    btn.addEventListener('click', () => {
      selectedFiles.splice(parseInt(btn.dataset.i), 1);
      renderFileList();
      if (selectedFiles.length === 0) {
        document.getElementById('uploadOptions').style.display = 'none';
        document.getElementById('uploadBtn').style.display = 'none';
      }
    });
  });
}

document.getElementById('uploadBtn').addEventListener('click', async () => {
  if (selectedFiles.length === 0) return;
  const config = await getConfig();
  if (!config.serverUrl) {
    showError('uploadError', 'Please configure server URL in Settings');
    return;
  }

  const btn = document.getElementById('uploadBtn');
  btn.disabled = true;
  btn.textContent = 'Uploading...';
  hideError('uploadError');

  try {
    const totalSize = selectedFiles.reduce((s, f) => s + f.size, 0);
    let uploadedSize = 0;
    const startTime = Date.now();

    // Init transfer
    const initRes = await fetch(`${config.serverUrl}/api/upload/init`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(config.apiToken ? { 'Authorization': `Bearer ${config.apiToken}` } : {}),
      },
      body: JSON.stringify({
        name: selectedFiles.length === 1 ? selectedFiles[0].name : `${selectedFiles.length} files`,
        file_count: selectedFiles.length,
        encrypted: false,
        max_downloads: parseInt(document.getElementById('maxDownloads').value) || 100,
        expiry_hours: parseInt(document.getElementById('expiryHours').value) || 24,
      }),
    });
    const initData = await initRes.json();

    document.getElementById('uploadProgress').style.display = 'block';

    // Upload each file
    for (const file of selectedFiles) {
      const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
      let fileId = null;
      for (let i = 0; i < totalChunks; i++) {
        const start = i * CHUNK_SIZE;
        const end = Math.min(start + CHUNK_SIZE, file.size);
        const chunk = file.slice(start, end);

        const headers = {
          'X-Transfer-ID': initData.transfer_id,
          'X-File-ID': fileId || '',
          'X-Chunk-Index': String(i),
          'X-Total-Chunks': String(totalChunks),
          'X-File-Name': file.name,
          'X-File-Size': String(file.size),
          'X-Mime-Type': file.type || 'application/octet-stream',
        };
        if (config.apiToken) headers['Authorization'] = `Bearer ${config.apiToken}`;

        const chunkRes = await fetch(`${config.serverUrl}/api/upload/chunk`, {
          method: 'POST', headers, body: chunk,
        });
        const chunkData = await chunkRes.json();
        fileId = chunkData.file_id;

        uploadedSize += chunk.size;
        const pct = Math.round((uploadedSize / totalSize) * 100);
        document.getElementById('progressFill').style.width = `${pct}%`;
        const elapsed = (Date.now() - startTime) / 1000;
        const speed = uploadedSize / elapsed;
        document.getElementById('progressStatus').textContent = `${pct}% - ${formatBytes(speed)}/s`;
      }
    }

    // Complete
    await fetch(`${config.serverUrl}/api/upload/complete/${initData.transfer_id}`, {
      method: 'POST',
      ...(config.apiToken ? { headers: { 'Authorization': `Bearer ${config.apiToken}` } } : {}),
    });

    // Show result
    document.getElementById('uploadProgress').style.display = 'none';
    const url = initData.download_url || `${config.serverUrl}/d/${initData.transfer_id}`;
    document.getElementById('resultUrl').textContent = url;
    document.getElementById('uploadResult').style.display = 'block';
    document.getElementById('copyBtn').onclick = () => {
      navigator.clipboard.writeText(url);
      document.getElementById('copyBtn').textContent = 'Copied!';
      setTimeout(() => document.getElementById('copyBtn').textContent = 'Copy link', 2000);
    };

    selectedFiles = [];
    renderFileList();
    document.getElementById('uploadOptions').style.display = 'none';
    document.getElementById('uploadBtn').style.display = 'none';
  } catch (err) {
    showError('uploadError', err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Upload files';
  }
});

// ---- Clipboard ----
document.getElementById('clipBtn').addEventListener('click', async () => {
  const content = document.getElementById('clipContent').value;
  if (!content.trim()) return;
  const config = await getConfig();
  if (!config.serverUrl) {
    showError('clipError', 'Please configure server URL in Settings');
    return;
  }

  hideError('clipError');
  try {
    const res = await fetch(`${config.serverUrl}/api/clipboard`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(config.apiToken ? { 'Authorization': `Bearer ${config.apiToken}` } : {}),
      },
      body: JSON.stringify({
        content,
        language: document.getElementById('clipLang').value || '',
        hours: 24,
      }),
    });
    const data = await res.json();
    document.getElementById('clipUrl').textContent = data.url;
    document.getElementById('clipResult').style.display = 'block';
    document.getElementById('clipCopyBtn').onclick = () => {
      navigator.clipboard.writeText(data.url);
      document.getElementById('clipCopyBtn').textContent = 'Copied!';
      setTimeout(() => document.getElementById('clipCopyBtn').textContent = 'Copy link', 2000);
    };
  } catch (err) {
    showError('clipError', err.message);
  }
});

// ---- Helpers ----
function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i];
}
function showError(id, msg) {
  const el = document.getElementById(id);
  el.textContent = msg;
  el.style.display = 'block';
}

function hideError(id) {
  document.getElementById(id).style.display = 'none';
}
