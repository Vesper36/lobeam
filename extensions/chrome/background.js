// LoBeam Chrome Extension - Background Service Worker

// Create context menus on install
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: 'lobeam-share-image',
    title: 'Share image via LoBeam',
    contexts: ['image'],
  });
  chrome.contextMenus.create({
    id: 'lobeam-share-link',
    title: 'Share link via LoBeam',
    contexts: ['link'],
  });
  chrome.contextMenus.create({
    id: 'lobeam-share-selection',
    title: 'Share selection via LoBeam',
    contexts: ['selection'],
  });
  chrome.contextMenus.create({
    id: 'lobeam-share-page',
    title: 'Share this page via LoBeam',
    contexts: ['page'],
  });
});

// Handle context menu clicks
chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  const config = await getConfig();
  if (!config.serverUrl) {
    chrome.tabs.create({ url: chrome.runtime.getURL('popup.html') });
    return;
  }

  switch (info.menuItemId) {
    case 'lobeam-share-image':
      await shareImage(info.srcUrl, config);
      break;
    case 'lobeam-share-link':
      await shareClipboard(info.linkUrl, config);
      break;
    case 'lobeam-share-selection':
      await shareClipboard(info.selectionText, config);
      break;
    case 'lobeam-share-page':
      await shareClipboard(info.pageUrl, config);
      break;
  }
});

// Get saved config
function getConfig() {
  return new Promise((resolve) => {
    chrome.storage.local.get(['serverUrl', 'apiToken'], (result) => {
      resolve({
        serverUrl: result.serverUrl || '',
        apiToken: result.apiToken || '',
      });
    });
  });
}

// Share an image URL as a clipboard entry
async function shareClipboard(content, config) {
  try {
    const res = await fetch(`${config.serverUrl}/api/clipboard`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(config.apiToken ? { 'Authorization': `Bearer ${config.apiToken}` } : {}),
      },
      body: JSON.stringify({ content, hours: 24 }),
    });
    const data = await res.json();
    if (data.url) {
      await navigator.clipboard.writeText(data.url);
      showNotification('Link copied to clipboard', data.url);
    }
  } catch (err) {
    showNotification('Failed to share', err.message);
  }
}

// Share image by downloading and re-uploading
async function shareImage(imageUrl, config) {
  try {
    showNotification('Uploading image...', 'Please wait');

    // Fetch the image
    const imgRes = await fetch(imageUrl);
    const blob = await imgRes.blob();
    const fileName = imageUrl.split('/').pop().split('?')[0] || 'image.jpg';

    // Init transfer
    const initRes = await fetch(`${config.serverUrl}/api/upload/init`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(config.apiToken ? { 'Authorization': `Bearer ${config.apiToken}` } : {}),
      },
      body: JSON.stringify({
        name: fileName,
        file_count: 1,
        encrypted: false,
        expiry_hours: 168,
      }),
    });
    const initData = await initRes.json();

    // Upload chunk
    await fetch(`${config.serverUrl}/api/upload/chunk`, {
      method: 'POST',
      headers: {
        'X-Transfer-ID': initData.transfer_id,
        'X-Chunk-Index': '0',
        'X-Total-Chunks': '1',
        'X-File-Name': fileName,
        'X-File-Size': String(blob.size),
        'X-Mime-Type': blob.type || 'image/jpeg',
        ...(config.apiToken ? { 'Authorization': `Bearer ${config.apiToken}` } : {}),
      },
      body: blob,
    });

    // Complete
    await fetch(`${config.serverUrl}/api/upload/complete/${initData.transfer_id}`, {
      method: 'POST',
      ...(config.apiToken ? { headers: { 'Authorization': `Bearer ${config.apiToken}` } } : {}),
    });

    if (initData.download_url) {
      await navigator.clipboard.writeText(initData.download_url);
      showNotification('Image uploaded, link copied!', initData.download_url);
    }
  } catch (err) {
    showNotification('Upload failed', err.message);
  }
}

function showNotification(title, message) {
  chrome.notifications?.create({
    type: 'basic',
    iconUrl: 'icons/icon128.png',
    title,
    message: message || '',
  });
}
