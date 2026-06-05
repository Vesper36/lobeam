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
  emailTransfer: (id, data) => request(`/t/${id}/email`, { method: 'POST', body: data }),

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
  uploadToWebFolder: (token, file, onProgress) => {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('POST', `${API_BASE}/f/${token}/upload`);
      const token_auth = localStorage.getItem('access_token');
      if (token_auth) xhr.setRequestHeader('Authorization', `Bearer ${token_auth}`);
      xhr.setRequestHeader('X-File-Name', file.name);
      xhr.setRequestHeader('X-File-Size', String(file.size));
      xhr.setRequestHeader('X-Mime-Type', file.type || 'application/octet-stream');
      if (onProgress) {
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) onProgress(e.loaded / e.total);
        };
      }
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(JSON.parse(xhr.responseText));
        } else {
          reject(new Error('Upload failed'));
        }
      };
      xhr.onerror = () => reject(new Error('Upload failed'));
      xhr.send(file);
    });
  },
  getWebFolderDownloadUrl: (token, fileID) => `${API_BASE}/f/${token}/download/${fileID}`,

  // File Requests
  createFileRequest: (data) =>
    request('/file-requests', { method: 'POST', body: data }),
  listFileRequests: () => request('/file-requests'),
  getFileRequest: (id) => request(`/r/${id}`),
  uploadToFileRequest: (id, file, onProgress, meta) => {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('POST', `${API_BASE}/r/${id}/submit`);
      const token_auth = localStorage.getItem('access_token');
      if (token_auth) xhr.setRequestHeader('Authorization', `Bearer ${token_auth}`);
      xhr.setRequestHeader('X-File-Name', file.name);
      xhr.setRequestHeader('X-File-Size', String(file.size));
      xhr.setRequestHeader('X-Mime-Type', file.type || 'application/octet-stream');
      if (meta?.name) xhr.setRequestHeader('X-Uploader-Name', meta.name);
      if (meta?.email) xhr.setRequestHeader('X-Uploader-Email', meta.email);
      if (onProgress) {
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) onProgress(e.loaded / e.total);
        };
      }
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(JSON.parse(xhr.responseText));
        } else {
          reject(new Error('Upload failed'));
        }
      };
      xhr.onerror = () => reject(new Error('Upload failed'));
      xhr.send(file);
    });
  },

  // Brand
  getBrand: () => request('/brand'),
  updateBrand: (data) => request('/brand', { method: 'POST', body: data }),

  // Settings
  getSettings: () => request('/settings'),
  updateSettings: (data) => request('/settings', { method: 'POST', body: data }),

  // Admin
  listUsers: () => request('/admin/users'),
  deleteUser: (id) => request(`/admin/users/${id}`, { method: 'DELETE' }),
  getAuditLogs: () => request('/admin/logs'),

  // Profile
  getMe: () => request('/me'),
};