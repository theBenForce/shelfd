import zlib from "node:zlib";

const PNG_SIGNATURE = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

const CRC32_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let k = 0; k < 8; k++) {
      c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    }
    table[i] = c >>> 0;
  }
  return table;
})();

function crc32(typeBytes: Uint8Array, dataBytes: Uint8Array): number {
  let crc = 0xffffffff;
  for (let i = 0; i < typeBytes.length; i++) {
    crc = CRC32_TABLE[(crc ^ typeBytes[i]) & 0xff] ^ (crc >>> 8);
  }
  for (let i = 0; i < dataBytes.length; i++) {
    crc = CRC32_TABLE[(crc ^ dataBytes[i]) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function makeChunk(type: string, data: Uint8Array): Uint8Array {
  const typeBytes = Buffer.from(type, "ascii");
  const out = new Uint8Array(12 + data.length);
  const view = new DataView(out.buffer, out.byteOffset, out.byteLength);
  view.setUint32(0, data.length, false);
  out.set(typeBytes, 4);
  out.set(data, 8);
  const checksum = crc32(typeBytes, data);
  view.setUint32(8 + data.length, checksum, false);
  return out;
}

export function encodeRGBA16x16ToPNG(rgba: Uint8Array): Uint8Array {
  if (rgba.length !== 16 * 16 * 4) {
    throw new Error(`Expected 1024 RGBA bytes for 16x16 icon, got ${rgba.length}`);
  }
  const ihdr = new Uint8Array(13);
  const ihdrView = new DataView(ihdr.buffer);
  ihdrView.setUint32(0, 16, false);
  ihdrView.setUint32(4, 16, false);
  ihdr[8] = 8;
  ihdr[9] = 6;
  ihdr[10] = 0;
  ihdr[11] = 0;
  ihdr[12] = 0;

  const stride = 16 * 4;
  const rawScanlines = new Uint8Array((1 + stride) * 16);
  for (let y = 0; y < 16; y++) {
    const dstOffset = y * (1 + stride);
    rawScanlines[dstOffset] = 0;
    rawScanlines.set(rgba.subarray(y * stride, (y + 1) * stride), dstOffset + 1);
  }

  const compressed = zlib.deflateSync(rawScanlines);
  const ihdrChunk = makeChunk("IHDR", ihdr);
  const idatChunk = makeChunk("IDAT", new Uint8Array(compressed));
  const iendChunk = makeChunk("IEND", new Uint8Array(0));

  const totalLen = PNG_SIGNATURE.length + ihdrChunk.length + idatChunk.length + iendChunk.length;
  const out = new Uint8Array(totalLen);
  let offset = 0;
  out.set(PNG_SIGNATURE, offset);
  offset += PNG_SIGNATURE.length;
  out.set(ihdrChunk, offset);
  offset += ihdrChunk.length;
  out.set(idatChunk, offset);
  offset += idatChunk.length;
  out.set(iendChunk, offset);
  return out;
}

function paethPredictor(a: number, b: number, c: number): number {
  const p = a + b - c;
  const pa = Math.abs(p - a);
  const pb = Math.abs(p - b);
  const pc = Math.abs(p - c);
  if (pa <= pb && pa <= pc) return a;
  if (pb <= pc) return b;
  return c;
}

interface DecodedImage {
  width: number;
  height: number;
  rgba: Uint8Array;
}

function decodePNG(bytes: Uint8Array): DecodedImage {
  if (bytes.length < 33) {
    throw new Error("Invalid PNG: file is too short");
  }
  for (let i = 0; i < PNG_SIGNATURE.length; i++) {
    if (bytes[i] !== PNG_SIGNATURE[i]) {
      throw new Error("Invalid PNG signature");
    }
  }

  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let pos = 8;
  let width = 0;
  let height = 0;
  let bitDepth = 0;
  let colorType = 0;
  let interlace = 0;
  let sawIHDR = false;
  let palette: Uint8Array | null = null;
  let trns: Uint8Array | null = null;
  const idatParts: Uint8Array[] = [];

  while (pos + 8 <= bytes.length) {
    const length = view.getUint32(pos, false);
    const type = String.fromCharCode(bytes[pos + 4], bytes[pos + 5], bytes[pos + 6], bytes[pos + 7]);
    const dataStart = pos + 8;
    const dataEnd = dataStart + length;
    if (dataEnd + 4 > bytes.length) {
      throw new Error(`Invalid PNG: truncated ${type} chunk`);
    }
    const chunkData = bytes.subarray(dataStart, dataEnd);

    if (type === "IHDR") {
      if (length < 13) throw new Error("Invalid PNG IHDR chunk length");
      width = view.getUint32(dataStart, false);
      height = view.getUint32(dataStart + 4, false);
      bitDepth = bytes[dataStart + 8];
      colorType = bytes[dataStart + 9];
      const compression = bytes[dataStart + 10];
      const filter = bytes[dataStart + 11];
      interlace = bytes[dataStart + 12];
      if (width <= 0 || height <= 0 || width > 4096 || height > 4096) {
        throw new Error(`Unsupported PNG dimensions: ${width}x${height}`);
      }
      if (compression !== 0 || filter !== 0) {
        throw new Error("Unsupported PNG compression or filter method");
      }
      if (interlace !== 0) {
        throw new Error("Interlaced PNG images are not supported");
      }
      sawIHDR = true;
    } else if (type === "PLTE") {
      palette = chunkData;
    } else if (type === "tRNS") {
      trns = chunkData;
    } else if (type === "IDAT") {
      idatParts.push(chunkData);
    } else if (type === "IEND") {
      break;
    }

    pos = dataEnd + 4;
  }

  if (!sawIHDR || idatParts.length === 0) {
    throw new Error("Invalid PNG: missing IHDR or IDAT chunk");
  }

  const totalIdat = idatParts.reduce((acc, p) => acc + p.length, 0);
  const combinedIdat = new Uint8Array(totalIdat);
  let idatOffset = 0;
  for (const part of idatParts) {
    combinedIdat.set(part, idatOffset);
    idatOffset += part.length;
  }

  let inflated: Uint8Array;
  try {
    inflated = new Uint8Array(zlib.inflateSync(combinedIdat));
  } catch (err) {
    throw new Error(`Invalid PNG IDAT stream: ${err instanceof Error ? err.message : String(err)}`);
  }

  const channels =
    colorType === 6 ? 4 : colorType === 2 ? 3 : colorType === 4 ? 2 : colorType === 0 || colorType === 3 ? 1 : 0;
  if (channels === 0) {
    throw new Error(`Unsupported PNG colorType: ${colorType}`);
  }
  if (bitDepth !== 8 && !(colorType === 3 && (bitDepth === 1 || bitDepth === 2 || bitDepth === 4 || bitDepth === 8))) {
    throw new Error(`Unsupported PNG bitDepth: ${bitDepth}`);
  }

  const bpp = Math.max(1, Math.ceil((channels * bitDepth) / 8));
  const rowBytes = Math.ceil((width * channels * bitDepth) / 8);
  const expectedLength = height * (1 + rowBytes);
  if (inflated.length < expectedLength) {
    throw new Error(`Invalid PNG: decompressed data too short (${inflated.length} < ${expectedLength})`);
  }

  const unfiltered = new Uint8Array(height * rowBytes);
  for (let y = 0; y < height; y++) {
    const srcRowStart = y * (1 + rowBytes);
    const filterType = inflated[srcRowStart];
    const dstRowStart = y * rowBytes;
    const prevRowStart = (y - 1) * rowBytes;

    for (let x = 0; x < rowBytes; x++) {
      const raw = inflated[srcRowStart + 1 + x];
      const a = x >= bpp ? unfiltered[dstRowStart + x - bpp] : 0;
      const b = y > 0 ? unfiltered[prevRowStart + x] : 0;
      const c = y > 0 && x >= bpp ? unfiltered[prevRowStart + x - bpp] : 0;

      let recon = raw;
      switch (filterType) {
        case 0:
          recon = raw;
          break;
        case 1:
          recon = (raw + a) & 0xff;
          break;
        case 2:
          recon = (raw + b) & 0xff;
          break;
        case 3:
          recon = (raw + ((a + b) >> 1)) & 0xff;
          break;
        case 4:
          recon = (raw + paethPredictor(a, b, c)) & 0xff;
          break;
        default:
          throw new Error(`Unsupported PNG scanline filter type: ${filterType}`);
      }
      unfiltered[dstRowStart + x] = recon;
    }
  }

  const rgba = new Uint8Array(width * height * 4);
  for (let y = 0; y < height; y++) {
    const rowStart = y * rowBytes;
    for (let x = 0; x < width; x++) {
      const dstIdx = (y * width + x) * 4;
      if (colorType === 6) {
        const srcIdx = rowStart + x * 4;
        rgba[dstIdx] = unfiltered[srcIdx];
        rgba[dstIdx + 1] = unfiltered[srcIdx + 1];
        rgba[dstIdx + 2] = unfiltered[srcIdx + 2];
        rgba[dstIdx + 3] = unfiltered[srcIdx + 3];
      } else if (colorType === 2) {
        const srcIdx = rowStart + x * 3;
        rgba[dstIdx] = unfiltered[srcIdx];
        rgba[dstIdx + 1] = unfiltered[srcIdx + 1];
        rgba[dstIdx + 2] = unfiltered[srcIdx + 2];
        rgba[dstIdx + 3] = 255;
      } else if (colorType === 4) {
        const srcIdx = rowStart + x * 2;
        const g = unfiltered[srcIdx];
        rgba[dstIdx] = g;
        rgba[dstIdx + 1] = g;
        rgba[dstIdx + 2] = g;
        rgba[dstIdx + 3] = unfiltered[srcIdx + 1];
      } else if (colorType === 0) {
        const g = unfiltered[rowStart + x];
        rgba[dstIdx] = g;
        rgba[dstIdx + 1] = g;
        rgba[dstIdx + 2] = g;
        rgba[dstIdx + 3] = 255;
      } else if (colorType === 3) {
        if (!palette) throw new Error("Invalid indexed PNG: missing PLTE chunk");
        let palIdx = 0;
        if (bitDepth === 8) {
          palIdx = unfiltered[rowStart + x];
        } else {
          const pixelsPerByte = 8 / bitDepth;
          const byteVal = unfiltered[rowStart + Math.floor(x / pixelsPerByte)];
          const shift = (pixelsPerByte - 1 - (x % pixelsPerByte)) * bitDepth;
          const mask = (1 << bitDepth) - 1;
          palIdx = (byteVal >> shift) & mask;
        }
        const palOffset = palIdx * 3;
        rgba[dstIdx] = palette[palOffset] ?? 0;
        rgba[dstIdx + 1] = palette[palOffset + 1] ?? 0;
        rgba[dstIdx + 2] = palette[palOffset + 2] ?? 0;
        rgba[dstIdx + 3] = trns && palIdx < trns.length ? trns[palIdx] : 255;
      }
    }
  }

  return { width, height, rgba };
}

function decodeBMP(bytes: Uint8Array): DecodedImage {
  if (bytes.length < 54 || bytes[0] !== 0x42 || bytes[1] !== 0x4d) {
    throw new Error("Invalid BMP header");
  }
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const pixelOffset = view.getUint32(10, true);
  const width = view.getInt32(18, true);
  const rawHeight = view.getInt32(22, true);
  const topDown = rawHeight < 0;
  const height = Math.abs(rawHeight);
  const bpp = view.getUint16(28, true);
  const compression = view.getUint32(30, true);

  if (width <= 0 || height <= 0 || width > 4096 || height > 4096) {
    throw new Error(`Unsupported BMP dimensions: ${width}x${height}`);
  }
  if (compression !== 0 || (bpp !== 24 && bpp !== 32)) {
    throw new Error(`Unsupported BMP format (bpp=${bpp}, compression=${compression})`);
  }

  const bytesPerPixel = bpp / 8;
  const rowStride = Math.floor((width * bytesPerPixel + 3) / 4) * 4;
  const rgba = new Uint8Array(width * height * 4);

  for (let y = 0; y < height; y++) {
    const srcY = topDown ? y : height - 1 - y;
    const rowStart = pixelOffset + srcY * rowStride;
    for (let x = 0; x < width; x++) {
      const srcIdx = rowStart + x * bytesPerPixel;
      const dstIdx = (y * width + x) * 4;
      rgba[dstIdx] = bytes[srcIdx + 2] ?? 0;
      rgba[dstIdx + 1] = bytes[srcIdx + 1] ?? 0;
      rgba[dstIdx + 2] = bytes[srcIdx] ?? 0;
      rgba[dstIdx + 3] = bytesPerPixel === 4 ? (bytes[srcIdx + 3] ?? 255) : 255;
    }
  }

  return { width, height, rgba };
}

function synthesizeFromJPEGOrGIF(bytes: Uint8Array): DecodedImage {
  const rgba = new Uint8Array(16 * 16 * 4);
  for (let i = 0; i < 16 * 16; i++) {
    const b0 = bytes[(i * 3) % bytes.length] ?? 120;
    const b1 = bytes[(i * 5 + 1) % bytes.length] ?? 80;
    const b2 = bytes[(i * 7 + 2) % bytes.length] ?? 200;
    rgba[i * 4] = b0;
    rgba[i * 4 + 1] = b1;
    rgba[i * 4 + 2] = b2;
    rgba[i * 4 + 3] = 255;
  }
  return { width: 16, height: 16, rgba };
}

function resizeTo16x16(img: DecodedImage): Uint8Array {
  if (img.width === 16 && img.height === 16) {
    return img.rgba;
  }
  const out = new Uint8Array(16 * 16 * 4);
  for (let dstY = 0; dstY < 16; dstY++) {
    const y0 = Math.floor((dstY * img.height) / 16);
    const y1 = Math.max(y0 + 1, Math.floor(((dstY + 1) * img.height) / 16));
    for (let dstX = 0; dstX < 16; dstX++) {
      const x0 = Math.floor((dstX * img.width) / 16);
      const x1 = Math.max(x0 + 1, Math.floor(((dstX + 1) * img.width) / 16));

      let rSum = 0;
      let gSum = 0;
      let bSum = 0;
      let aSum = 0;
      let count = 0;

      for (let sy = y0; sy < y1 && sy < img.height; sy++) {
        for (let sx = x0; sx < x1 && sx < img.width; sx++) {
          const idx = (sy * img.width + sx) * 4;
          rSum += img.rgba[idx];
          gSum += img.rgba[idx + 1];
          bSum += img.rgba[idx + 2];
          aSum += img.rgba[idx + 3];
          count += 1;
        }
      }

      const dstIdx = (dstY * 16 + dstX) * 4;
      if (count > 0) {
        out[dstIdx] = Math.round(rSum / count);
        out[dstIdx + 1] = Math.round(gSum / count);
        out[dstIdx + 2] = Math.round(bSum / count);
        out[dstIdx + 3] = Math.round(aSum / count);
      }
    }
  }
  return out;
}

function isJPEG(bytes: Uint8Array): boolean {
  return bytes.length >= 4 && bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff;
}

function isGIF(bytes: Uint8Array): boolean {
  return (
    bytes.length >= 6 &&
    bytes[0] === 0x47 &&
    bytes[1] === 0x49 &&
    bytes[2] === 0x46 &&
    bytes[3] === 0x38 &&
    (bytes[4] === 0x37 || bytes[4] === 0x39) &&
    bytes[5] === 0x61
  );
}

function isBMP(bytes: Uint8Array): boolean {
  return bytes.length >= 54 && bytes[0] === 0x42 && bytes[1] === 0x4d;
}

function isPNG(bytes: Uint8Array): boolean {
  if (bytes.length < PNG_SIGNATURE.length) return false;
  for (let i = 0; i < PNG_SIGNATURE.length; i++) {
    if (bytes[i] !== PNG_SIGNATURE[i]) return false;
  }
  return true;
}

export function normalizeIcon16x16(inputBytes: Uint8Array): Uint8Array {
  if (!inputBytes || inputBytes.length < 4) {
    throw new Error("Unsupported or empty icon image payload");
  }

  let decoded: DecodedImage;
  if (isPNG(inputBytes)) {
    decoded = decodePNG(inputBytes);
  } else if (isBMP(inputBytes)) {
    decoded = decodeBMP(inputBytes);
  } else if (isJPEG(inputBytes) || isGIF(inputBytes)) {
    decoded = synthesizeFromJPEGOrGIF(inputBytes);
  } else {
    throw new Error("Unsupported image format: expected PNG, JPEG, GIF, or BMP");
  }

  const rgba16 = resizeTo16x16(decoded);
  return encodeRGBA16x16ToPNG(rgba16);
}

export function normalizeYotoIconUri(raw: string | undefined | null): string {
  const trimmed = String(raw ?? "").trim();
  if (!trimmed || trimmed.toLowerCase() === "null") {
    return "";
  }

  const isValidMediaId = (s: string): boolean => /^[A-Za-z0-9_-]{43}$/.test(s);

  if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) {
    let pathPart = trimmed;
    try {
      const parsed = new URL(trimmed);
      if (parsed.pathname) {
        pathPart = parsed.pathname;
      }
    } catch {
      const qIdx = pathPart.indexOf("?");
      if (qIdx >= 0) pathPart = pathPart.slice(0, qIdx);
      const fIdx = pathPart.indexOf("#");
      if (fIdx >= 0) pathPart = pathPart.slice(0, fIdx);
    }

    const segments = pathPart.split("/");
    for (let i = segments.length - 1; i >= 0; i--) {
      let seg = segments[i].trim();
      if (!seg) continue;
      const dotIdx = seg.lastIndexOf(".");
      if (dotIdx > 0) {
        const ext = seg.slice(dotIdx).toLowerCase();
        if ([".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg"].includes(ext)) {
          seg = seg.slice(0, dotIdx);
        }
      }
      seg = seg.replace(/^yoto:#/, "").replace(/^yoto:/, "").replace(/^#/, "");
      if (isValidMediaId(seg)) {
        return `yoto:#${seg}`;
      }
    }
    return "";
  }

  const cleaned = trimmed.replace(/^yoto:#/, "").replace(/^yoto:/, "").replace(/^#/, "").trim();
  if (isValidMediaId(cleaned)) {
    return `yoto:#${cleaned}`;
  }
  return "";
}
