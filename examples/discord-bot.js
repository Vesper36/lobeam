// LoBeam Discord Bot - /file-send command
// Add this to your Discord bot for file sharing via LoBeam

const { SlashCommandBuilder, AttachmentBuilder } = require('discord.js');
const fs = require('fs');
const https = require('https');

module.exports = {
  data: new SlashCommandBuilder()
    .setName('file-send')
    .setDescription('Upload a file to LoBeam and share the link')
    .addAttachmentOption(opt =>
      opt.setName('file')
        .setDescription('File to upload')
        .setRequired(true))
    .addIntegerOption(opt =>
      opt.setName('expiry')
        .setDescription('Hours until expiry (default: 24)')
        .setMinValue(1)
        .setMaxValue(720))
    .addStringOption(opt =>
      opt.setName('note')
        .setDescription('Note for recipient')),

  async execute(interaction, config) {
    await interaction.deferReply();

    const attachment = interaction.options.getAttachment('file');
    const expiry = interaction.options.getInteger('expiry') || 24;
    const note = interaction.options.getString('note') || '';

    try {
      // Download attachment to temp file
      const res = await fetch(attachment.url);
      const buffer = Buffer.from(await res.arrayBuffer());

      // Init upload
      const initRes = await fetch(`${config.server}/api/upload/init`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${config.token}`,
        },
        body: JSON.stringify({
          name: attachment.name,
          file_count: 1,
          encrypted: false,
          expiry_hours: expiry,
          note: note,
        }),
      });
      const init = await initRes.json();

      // Upload chunk
      await fetch(`${config.server}/api/upload/chunk`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${config.token}`,
          'X-Transfer-ID': init.transfer_id,
          'X-File-ID': 'null',
          'X-Chunk-Index': '0',
          'X-Total-Chunks': '1',
          'X-File-Name': attachment.name,
          'X-File-Size': String(buffer.length),
          'X-Mime-Type': attachment.contentType || 'application/octet-stream',
        },
        body: buffer,
      });

      // Complete
      const completeRes = await fetch(`${config.server}/api/upload/complete/${init.transfer_id}`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${config.token}` },
      });
      const complete = await completeRes.json();

      await interaction.editReply({
        content: `File uploaded: **${attachment.name}** (${(buffer.length / 1024 / 1024).toFixed(1)} MB)\nDownload: ${complete.download_url}\nExpires: ${expiry}h`,
      });
    } catch (err) {
      await interaction.editReply({ content: `Upload failed: ${err.message}` });
    }
  },
};
