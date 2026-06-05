const API_BASE = '/api';

async function request(path, options = {}) {
  const token = localStorage.getItem('access_token');
  const headers = { ...options.headers };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  if (options.body && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
    options.body = JSON.stringify(options.body);
  }

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (res.status === 401) {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(err.error || 'Request failed');
  }

  return res.json();
}

export const api = {
  // Auth
  login: (username, password) =>
    request('/auth/login', { method: 'POST', body: { username, password } }),
  register: (username, email, password) =>
    request('/auth/register', { method: 'POST', body: { username, email, password } }),

  // Upload
  initUpload: (data) =>
    request('/upload/init', { method: 'POST', body: data }),
  uploadChunk: async (transferId, fileId, chunkIndex, totalChunks, fileName, fileSize, mimeType, chunkData, chunkHash) => {
    const token = localStorage.getItem('access_token');
    const res = await fetch(`${API_BASE}/upload/chunk`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'X-Transfer-ID': transferId,
        'X-File-ID': fileId || '',
        'X-Chunk-Index': String(chunkIndex),
        'X-Total-Chunks': String(totalChunks),
        'X-File-Name': fileName,
        'X-File-Size': String(fileSize),
        'X-Mime-Type': mimeType,
        'X-Chunk-Hash': chunkHash || '',
      },
      body: chunkData,
    });
    if (!res.ok) throw new Error('Chunk upload failed');
    return res.json();
  },
  completeUpload: (transferId) =>
    request(`/upload/complete/${transferId}`, { method: 'POST' }),

  // Transfer
  getTransfer: (id) => request(`/t/${id}`),
  getTransferFiles: (id) => request(`/t/${id}/files`),
  listTransfers: () => request('/transfers'),
  deleteTransfer: (id) => request(`/transfers/${id}`, { method: 'DELETE' }),

  // Clipboard
  createClipboard: (data) =>
    request('/clipboard', { method: 'POST', body: data }),
  getClipboard: (id) => request(`/clipboard/${id}`),

  // P2P
  createP2P: () => request('/p2p/create', { method: 'POST' }),
  getP2P: (code) => request(`/p2p/${code}`),
};
