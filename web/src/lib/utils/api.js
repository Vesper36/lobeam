const API_BASE = '/api';
const CHUNK_SIZE = 5 * 1024 * 1024;

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
    throw new Error('unauthorized');
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(err.error || 'Request failed');
  }

  return res.json();
}

async function sha256Hex(buffer) {
  if (!globalThis.crypto?.subtle) {
    return '';
  }
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer);
  return Array.from(new Uint8Array(hashBuffer))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

async function uploadRawChunk(path, upload, file, chunkIndex, chunkData, chunkHash) {
  const token = localStorage.getItem('access_token');
  const headers = {
    'X-Transfer-ID': upload.transfer_id,
    'X-File-ID': upload.file_id,
    'X-Chunk-Index': String(chunkIndex),
    'X-File-Name': file.name,
    'X-File-Size': String(file.size),
    'X-Mime-Type': file.type || 'application/octet-stream',
    'X-Chunk-Hash': chunkHash || '',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: chunkData,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Chunk upload failed' }));
    throw new Error(err.error || 'Chunk upload failed');
  }
  return res.json();
}

async function uploadChunkedFile(file, paths, meta = {}, onProgress) {
  const totalChunks = Math.max(1, Math.ceil(file.size / CHUNK_SIZE));
  const upload = await request(paths.init, {
    method: 'POST',
    body: {
      name: file.name,
      size: file.size,
      mime_type: file.type || 'application/octet-stream',
      total_chunks: totalChunks,
      chunk_size: CHUNK_SIZE,
      ...meta,
    },
  });

  let uploadedBytes = 0;
  for (let i = 0; i < totalChunks; i++) {
    const start = i * CHUNK_SIZE;
    const end = Math.min(start + CHUNK_SIZE, file.size);
    const chunk = file.slice(start, end);
    const buffer = await chunk.arrayBuffer();
    const hash = await sha256Hex(buffer);

    await uploadRawChunk(paths.chunk, upload, file, i, buffer, hash);
    uploadedBytes += chunk.size;
    if (onProgress) {
      onProgress(file.size === 0 ? 1 : uploadedBytes / file.size);
    }
  }

  const completePath = paths.complete(upload.transfer_id);
  return request(completePath, { method: 'POST' });
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
    const headers = {
      'X-Transfer-ID': transferId,
      'X-File-ID': fileId || '',
      'X-Chunk-Index': String(chunkIndex),
      'X-Total-Chunks': String(totalChunks),
      'X-File-Name': fileName,
      'X-File-Size': String(fileSize),
      'X-Mime-Type': mimeType,
      'X-Chunk-Hash': chunkHash || '',
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const res = await fetch(`${API_BASE}/upload/chunk`, {
      method: 'POST',
      headers,
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
  emailTransfer: (id, data) => request(`/t/${id}/email`, { method: 'POST', body: data }),
  updateMagnet: (id, magnetURI) => request(`/t/${id}/magnet`, { method: 'PUT', body: { magnet_uri: magnetURI } }),

  // Clipboard
  createClipboard: (data) =>
    request('/clipboard', { method: 'POST', body: data }),
  getClipboard: (id) => request(`/clipboard/${id}`),

  // P2P
  createP2P: () => request('/p2p/create', { method: 'POST' }),
  getP2P: (code) => request(`/p2p/${code}`),

  // Web Folders
  createWebFolder: (data) =>
    request('/folders', { method: 'POST', body: data }),
  listWebFolders: () => request('/folders'),
  getWebFolder: (token) => request(`/f/${token}`),
  getWebFolderFiles: (token) => request(`/f/${token}/files`),
  uploadToWebFolder: (token, file, onProgress, meta = {}) =>
    uploadChunkedFile(file, {
      init: `/f/${token}/uploads/init`,
      chunk: `/f/${token}/uploads/chunk`,
      complete: (transferId) => `/f/${token}/uploads/${transferId}/complete`,
    }, meta, onProgress),
  getWebFolderDownloadUrl: (token, fileID) => `${API_BASE}/f/${token}/download/${fileID}`,

  // File Requests
  createFileRequest: (data) =>
    request('/file-requests', { method: 'POST', body: data }),
  listFileRequests: () => request('/file-requests'),
  getFileRequest: (id) => request(`/r/${id}`),
  uploadToFileRequest: (id, file, onProgress, meta = {}) =>
    uploadChunkedFile(file, {
      init: `/r/${id}/uploads/init`,
      chunk: `/r/${id}/uploads/chunk`,
      complete: (transferId) => `/r/${id}/uploads/${transferId}/complete`,
    }, {
      uploader_name: meta.name || '',
      uploader_email: meta.email || '',
      message: meta.message || '',
    }, onProgress),

  // Brand
  getBrand: () => request('/brand'),
  updateBrand: (data) => request('/brand', { method: 'POST', body: data }),

  // Settings
  getSettings: () => request('/settings'),
  updateSettings: (data) => request('/settings', { method: 'POST', body: data }),

  // Admin
  listUsers: () => request('/admin/users'),
  deleteUser: (id) => request(`/admin/users/${id}`, { method: 'DELETE' }),
  updateUser: (id, data) => request(`/admin/users/${id}`, { method: 'PUT', body: data }),
  getAuditLogs: () => request('/admin/logs'),

  // Profile
  getMe: () => request('/me'),
};
