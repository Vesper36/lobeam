# LoBeam Chrome Extension

Share files, images, links, and text snippets directly to your LoBeam instance from any webpage.

## Install (Developer Mode)

1. Open Chrome and navigate to `chrome://extensions/`
2. Enable **Developer mode** (top right toggle)
3. Click **Load unpacked**
4. Select the `extensions/chrome/` directory
5. The LoBeam icon appears in your toolbar

## Setup

1. Click the LoBeam icon in the toolbar
2. Go to the **Settings** tab
3. Enter your LoBeam server URL (e.g., `https://lobeam.demo.vesper36.cc`)
4. Optionally enter an API token for authenticated uploads
5. Click **Save settings**

## Features

### Quick Upload
- Click the toolbar icon and drag files into the popup
- Supports any file type, any size (chunked upload)
- Progress bar with upload speed
- Automatic link copy to clipboard

### Context Menu
Right-click on any webpage to access:
- **Share image via LoBeam** -- Upload and share images from any website
- **Share link via LoBeam** -- Create a clipboard entry with the link
- **Share selection via LoBeam** -- Share selected text as a clipboard entry
- **Share this page via LoBeam** -- Share the current page URL

### Clipboard
- Share text, code, or notes
- Optional syntax language tag
- Generates a short URL for easy sharing

## Permissions

| Permission | Reason |
|-----------|--------|
| `contextMenus` | Right-click menu entries |
| `activeTab` | Access current tab for sharing |
| `storage` | Save server URL and API token locally |

## Build

No build step required. This is a plain JavaScript Manifest V3 extension.

To package for Chrome Web Store:
```bash
cd extensions/chrome
zip -r lobeam-extension.zip .
```
