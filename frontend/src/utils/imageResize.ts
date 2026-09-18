const IMAGE_WIDTH_TITLE_PREFIX = 'atlasnote-width:'

export function parseImageWidth(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null
  const width = Math.round(value)
  return width >= 32 && width <= 16384 ? width : null
}

export function parseImageWidthFromTitle(title: unknown): number | null {
  if (typeof title !== 'string' || !title.startsWith(IMAGE_WIDTH_TITLE_PREFIX)) return null
  return parseImageWidth(Number(title.slice(IMAGE_WIDTH_TITLE_PREFIX.length)))
}

export function createImageWidthTitle(width: number | null): string | null {
  const normalized = parseImageWidth(width)
  return normalized === null ? null : `${IMAGE_WIDTH_TITLE_PREFIX}${normalized}`
}
