export const MAX_ATTACHMENT_BYTES = 10 * 1024 * 1024
export const MAX_IMAGE_WIDTH = 8192
export const MAX_IMAGE_HEIGHT = 8192
export const MAX_IMAGE_PIXELS = 40_000_000

export type ClipboardImagePayload = {
  mimeType: 'image/png' | 'image/jpeg'
  name: string
  data: Uint8Array
  width: number
  height: number
}

export function getClipboardImage(event: ClipboardEvent): File | null {
  const items = event.clipboardData?.items
  if (!items) return null
  for (const item of Array.from(items)) {
    if (item.kind !== 'file' || !isSupportedImageMime(item.type)) continue
    const file = item.getAsFile()
    if (file) return file
  }
  return null
}

export async function readClipboardImage(file: File, mimeHint = file.type): Promise<ClipboardImagePayload> {
  const mimeType = normalizeImageMime(mimeHint)
  if (!mimeType) throw new Error('unsupported-image-type')
  if (file.size <= 0 || file.size > MAX_ATTACHMENT_BYTES) throw new Error('image-too-large')

  const data = new Uint8Array(await file.arrayBuffer())
  if (data.length > MAX_ATTACHMENT_BYTES || !hasImageSignature(data, mimeType)) {
    throw new Error('invalid-image')
  }

  const dimensions = readImageDimensions(data, mimeType)
    ?? await readImageDimensionsWithBitmap(data, mimeType)
  if (!dimensions) throw new Error('invalid-image-dimensions')
  if (
    dimensions.width < 1
    || dimensions.height < 1
    || dimensions.width > MAX_IMAGE_WIDTH
    || dimensions.height > MAX_IMAGE_HEIGHT
    || dimensions.width * dimensions.height > MAX_IMAGE_PIXELS
  ) {
    throw new Error('invalid-image-dimensions')
  }

  return {
    mimeType,
    name: normalizeImageName(file.name, mimeType),
    data,
    width: dimensions.width,
    height: dimensions.height,
  }
}

export function encodeBase64(data: Uint8Array) {
  let binary = ''
  const chunkSize = 0x8000
  for (let offset = 0; offset < data.length; offset += chunkSize) {
    binary += String.fromCharCode(...data.subarray(offset, offset + chunkSize))
  }
  return btoa(binary)
}

export function normalizeImageMime(value: string): 'image/png' | 'image/jpeg' | null {
  if (value === 'image/png') return value
  if (value === 'image/jpeg') return value
  return null
}

function isSupportedImageMime(value: string) {
  return normalizeImageMime(value) !== null
}

function hasImageSignature(data: Uint8Array, mimeType: 'image/png' | 'image/jpeg') {
  if (mimeType === 'image/png') {
    return data.length >= 8
      && data[0] === 0x89 && data[1] === 0x50 && data[2] === 0x4e && data[3] === 0x47
      && data[4] === 0x0d && data[5] === 0x0a && data[6] === 0x1a && data[7] === 0x0a
  }
  return data.length >= 3 && data[0] === 0xff && data[1] === 0xd8 && data[2] === 0xff
}

function readImageDimensions(data: Uint8Array, mimeType: 'image/png' | 'image/jpeg') {
  if (mimeType === 'image/png' && data.length >= 24) {
    const width = readUint32(data, 16)
    const height = readUint32(data, 20)
    return { width, height }
  }
  if (mimeType !== 'image/jpeg') return null

  let offset = 2
  while (offset + 4 <= data.length) {
    while (offset < data.length && data[offset] !== 0xff) offset += 1
    while (offset < data.length && data[offset] === 0xff) offset += 1
    if (offset >= data.length) return null
    const marker = data[offset]
    offset += 1
    if (marker === 0xd8 || marker === 0xd9) continue
    if (offset + 2 > data.length) return null
    const segmentLength = (data[offset] << 8) | data[offset + 1]
    if (segmentLength < 2 || offset + segmentLength > data.length) return null
    if (
      (marker >= 0xc0 && marker <= 0xc3)
      || (marker >= 0xc5 && marker <= 0xc7)
      || (marker >= 0xc9 && marker <= 0xcb)
      || (marker >= 0xcd && marker <= 0xcf)
    ) {
      if (segmentLength < 7) return null
      return {
        height: (data[offset + 3] << 8) | data[offset + 4],
        width: (data[offset + 5] << 8) | data[offset + 6],
      }
    }
    offset += segmentLength
  }
  return null
}

async function readImageDimensionsWithBitmap(data: Uint8Array, mimeType: 'image/png' | 'image/jpeg') {
  if (typeof createImageBitmap !== 'function') return null
  const copy = new Uint8Array(data.byteLength)
  copy.set(data)
  const bitmap = await createImageBitmap(new Blob([copy.buffer], { type: mimeType }))
  try {
    return { width: bitmap.width, height: bitmap.height }
  } finally {
    bitmap.close()
  }
}

function readUint32(data: Uint8Array, offset: number) {
  return (data[offset] * 0x1000000)
    + (data[offset + 1] << 16)
    + (data[offset + 2] << 8)
    + data[offset + 3]
}

function normalizeImageName(value: string, mimeType: 'image/png' | 'image/jpeg') {
  const fallback = mimeType === 'image/png' ? 'image.png' : 'image.jpg'
  const name = value.trim().replace(/[\\/\u0000-\u001f\u007f]/g, '_')
  return name && name !== '.' && name !== '..' ? name : fallback
}
