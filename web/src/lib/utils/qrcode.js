export async function generateQR(text, size = 128) {
  try {
    const QRCode = await import('qrcode');
    return await QRCode.default.toDataURL(text, {
      width: size,
      margin: 1,
      color: { dark: '#ffffff', light: '#00000000' },
    });
  } catch {
    // Fallback: generate as opaque
    const QRCode = await import('qrcode');
    return await QRCode.default.toDataURL(text, { width: size, margin: 1 });
  }
}

export default { generateQR };
