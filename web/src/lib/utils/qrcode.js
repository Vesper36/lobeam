import QRCode from 'qrcode';

export async function generateQR(text, size = 128) {
  try {
    return await QRCode.toDataURL(text, {
      width: size,
      margin: 1,
      color: { dark: '#ffffff', light: '#00000000' },
    });
  } catch {
    // Fallback: generate as opaque
    return await QRCode.toDataURL(text, { width: size, margin: 1 });
  }
}

export default { generateQR };