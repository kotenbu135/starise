import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

// ICO形式の最小限の実装 (1x16x16 mono)
function createSimpleICO(width = 16, height = 16) {
  // ICO header
  const reserved = Buffer.alloc(2);
  const type = Buffer.alloc(2);
  type.writeUInt16LE(1, 0); // 1 = ICO

  const numImages = Buffer.alloc(2);
  numImages.writeUInt16LE(1, 0);

  const iconDir = Buffer.concat([reserved, type, numImages]);

  // Icon directory entry
  const dirWidth = Buffer.alloc(1);
  dirWidth.writeUInt8(width, 0);

  const dirHeight = Buffer.alloc(1);
  dirHeight.writeUInt8(height, 0);

  const colorCount = Buffer.alloc(1); // 0 = no palette
  const dirReserved = Buffer.alloc(1);
  const planes = Buffer.alloc(2);
  planes.writeUInt16LE(1, 0);

  const bitsPerPixel = Buffer.alloc(2);
  bitsPerPixel.writeUInt16LE(32, 0); // 32-bit RGBA

  // Create a simple star icon in RGBA (16x16)
  const imageData = createStarImage(width, height);
  const imageSize = Buffer.alloc(4);
  imageSize.writeUInt32LE(imageData.length, 0);

  const imageOffset = Buffer.alloc(4);
  imageOffset.writeUInt32LE(22, 0); // Offset after headers (6 + 16)

  const dirEntry = Buffer.concat([
    dirWidth,
    dirHeight,
    colorCount,
    dirReserved,
    planes,
    bitsPerPixel,
    imageSize,
    imageOffset
  ]);

  return Buffer.concat([iconDir, dirEntry, imageData]);
}

function createStarImage(width = 16, height = 16) {
  // Create a 16x16 RGBA image (32 bits per pixel)
  const image = Buffer.alloc(width * height * 4);

  // Draw a simple star on transparent background
  const centerX = width / 2;
  const centerY = height / 2;
  const radius = 6;

  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const dx = x - centerX;
      const dy = y - centerY;
      const dist = Math.sqrt(dx * dx + dy * dy);

      const pixelIdx = (y * width + x) * 4;

      // Simple circle for star
      if (dist < radius) {
        image[pixelIdx] = 26;     // R
        image[pixelIdx + 1] = 26; // G
        image[pixelIdx + 2] = 26; // B
        image[pixelIdx + 3] = 255; // A
      } else {
        image[pixelIdx + 3] = 0; // Transparent
      }
    }
  }

  return image;
}

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.join(__dirname, '..', 'public');

const icoBuffer = createSimpleICO(16, 16);
const icoPath = path.join(publicDir, 'favicon.ico');

fs.writeFileSync(icoPath, icoBuffer);
console.log(`Generated ${icoPath}`);
