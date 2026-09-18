import createDOMPurify from 'dompurify'

export type MermaidTheme = 'light' | 'dark'

export const MERMAID_LIMITS = {
  maxTextSize: 50_000,
  maxEdges: 500,
} as const

export type MermaidRenderErrorCode =
  | 'empty'
  | 'too-large'
  | 'unsafe-syntax'
  | 'invalid'
  | 'unsafe-output'
  | 'unavailable'

export type MermaidRenderResult =
  | {
      ok: true
      svg: string
      altText: string
    }
  | {
      ok: false
      code: MermaidRenderErrorCode
      message: string
    }

export type MermaidApi = {
  initialize: (config: Record<string, unknown>) => void
  parse: (source: string, options?: { suppressErrors?: boolean }) => unknown | Promise<unknown>
  render: (id: string, source: string) => Promise<{ svg?: string }>
}

type MermaidRenderOptions = {
  theme?: MermaidTheme
  mermaid?: MermaidApi
}

const ERROR_MESSAGES: Record<MermaidRenderErrorCode, string> = {
  empty: 'Mermaidコードが空です。',
  'too-large': 'Mermaidコードが長すぎるため表示できません。',
  'unsafe-syntax': '安全上、このMermaid記法は表示できません。',
  invalid: 'Mermaidの構文を確認してください。',
  'unsafe-output': '図の出力を安全に表示できませんでした。',
  unavailable: 'Mermaidを読み込めないため表示できません。',
}

const UNSUPPORTED_SYNTAX = [
  // Mermaid interprets configuration/YAML/JSON containers before rendering.
  // The narrowly allowlisted flowchart shape metadata is checked separately.
  /%%\s*\{/,
  /^\uFEFF?\s*---\s*(?:[\r\n]|$)/,
  /(?:^|[;\r\n])\s*(?:sequenceDiagram\s+)?(?:click|callback|properties|details|links?)\b/i,
  /\b(?:iconify|icon\s*pack|icon\s*[:(])/i,
  /\b(?:service|group)\s+[\w-]+\s*\([^)]*:/i,
  /(?:https?:|data:|file:|javascript:|\/\/)/i,
  /\burl\s*\(|@import|!\[/i,
  /<\/?[a-z][^>]*>/i,
  // CSS escapes/comments can hide url(), @import and property names.
  /(?:^|[;\r\n])\s*(?:style|classDef|linkStyle)\b[^\r\n]*(?:\\|\/\*)/i,
]

const ALLOWED_FLOWCHART_SHAPES = new Set([
  'rect',
  'rounded',
  'stadium',
  'subroutine',
  'cylinder',
  'circle',
  'doublecircle',
  'diamond',
  'hexagon',
  'parallelogram',
  'trapezoid',
  'asymmetric',
  'lean-right',
  'lean-left',
  'delay',
  'cloud',
  'document',
  'stored-data',
  'tagged-document',
  'lined-document',
  'manual-file',
  'paper-tape',
  'divided-process',
  'lined-process',
  'card',
  'notched-rectangle',
  'small-circle',
  'framed-circle',
  'crossed-circle',
  'filled-circle',
  'fork',
])

const SVG_NAMESPACE = 'http://www.w3.org/2000/svg'

let mermaidPromise: Promise<MermaidApi> | null = null
let renderQueue: Promise<void> = Promise.resolve()
let renderId = 0
let svgPurifier: ReturnType<typeof createDOMPurify> | null = null

function failure(code: MermaidRenderErrorCode): MermaidRenderResult {
  return {
    ok: false,
    code,
    message: ERROR_MESSAGES[code],
  }
}

export function validateMermaidSource(source: string): MermaidRenderResult | null {
  if (source.trim().length === 0) return failure('empty')
  if (source.length > MERMAID_LIMITS.maxTextSize) return failure('too-large')
  if (UNSUPPORTED_SYNTAX.some(pattern => pattern.test(source)) || hasUnsafeShapeMetadata(source)) {
    return failure('unsafe-syntax')
  }
  return null
}

function hasUnsafeShapeMetadata(source: string) {
  const hasMetadata = /@\s*\{/.test(source)
  if (!hasMetadata) return false
  if (!/^\s*(?:flowchart|graph)\b/im.test(source)) return true

  const metadataPattern = /@\s*\{/g
  let match: RegExpExecArray | null
  while ((match = metadataPattern.exec(source)) !== null) {
    const closeIndex = source.indexOf('}', match.index + match[0].length)
    if (closeIndex < 0) return true
    const body = source.slice(match.index + match[0].length, closeIndex).trim()
    const entries = body.split(',').map(entry => entry.trim()).filter(Boolean)
    if (entries.length === 0 || entries.length > 2) return true

    const keys = new Set<string>()
    for (const entry of entries) {
      const separator = entry.indexOf(':')
      if (separator <= 0) return true
      const key = entry.slice(0, separator).trim().toLowerCase()
      const value = entry.slice(separator + 1).trim()
      if (keys.has(key) || !['shape', 'label'].includes(key)) return true
      keys.add(key)

      if (key === 'shape') {
        if (!/^[a-z][a-z0-9-]*$/i.test(value) || !ALLOWED_FLOWCHART_SHAPES.has(value.toLowerCase())) return true
        continue
      }

      if (!/^"[^"\r\n]*"|'[^'\r\n]*'$/.test(value)) return true
      if (/(?:https?:|data:|file:|javascript:|\/\/)|<\/?[a-z][^>]*>|!\[|@\s*\{/i.test(value)) return true
    }

    if (!keys.has('shape')) return true
    metadataPattern.lastIndex = closeIndex + 1
  }

  return false
}

function getTheme(theme?: MermaidTheme): MermaidTheme {
  if (theme) return theme
  if (typeof document !== 'undefined' && document.documentElement.dataset.theme === 'light') {
    return 'light'
  }
  return 'dark'
}

function getMermaidConfig(theme: MermaidTheme): Record<string, unknown> {
  return {
    startOnLoad: false,
    securityLevel: 'strict',
    htmlLabels: false,
    suppressErrorRendering: true,
    logLevel: 'fatal',
    maxTextSize: MERMAID_LIMITS.maxTextSize,
    maxEdges: MERMAID_LIMITS.maxEdges,
    theme: theme === 'dark' ? 'dark' : 'default',
    fontFamily: 'Meiryo, "Segoe UI", sans-serif',
    altFontFamily: 'Meiryo, "Segoe UI", sans-serif',
  }
}

async function loadMermaid(): Promise<MermaidApi> {
  if (!mermaidPromise) {
    mermaidPromise = import('mermaid').then(module => {
      const api = (module.default ?? module) as unknown as MermaidApi
      if (
        typeof api.initialize !== 'function'
        || typeof api.parse !== 'function'
        || typeof api.render !== 'function'
      ) {
        throw new Error('invalid mermaid module')
      }
      return api
    })
  }

  return mermaidPromise
}

class MermaidRenderFailure extends Error {
  constructor(readonly code: MermaidRenderErrorCode) {
    super(code)
  }
}

function throwFailure(code: MermaidRenderErrorCode): never {
  throw new MermaidRenderFailure(code)
}

function getSvgPurifier() {
  if (svgPurifier) return svgPurifier
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    throwFailure('unavailable')
  }

  svgPurifier = createDOMPurify(window)
  return svgPurifier
}

function hasOnlyInternalSvgUrls(value: string) {
  const urlPattern = /url\(\s*([^)]*)\)/gi
  let match: RegExpExecArray | null

  while ((match = urlPattern.exec(value)) !== null) {
    if (!/^#[A-Za-z0-9_.:-]+$/.test(match[1].trim())) return false
  }

  return true
}

function sanitizeSvg(svg: string) {
  if (
    /<\s*(?:script|image|iframe|object|embed)\b/i.test(svg)
    || /\bon[a-z-]+\s*=/i.test(svg)
    || /\b(?:href|xlink:href|src|srcset)\s*=\s*["'`]?(?:https?:|data:|file:|javascript:|\/\/)/i.test(svg)
  ) {
    throwFailure('unsafe-output')
  }

  const purifier = getSvgPurifier()
  const sanitized = purifier.sanitize(svg, {
    USE_PROFILES: { svg: true, svgFilters: false },
    ALLOW_DATA_ATTR: false,
    FORBID_TAGS: ['script', 'foreignObject', 'image', 'iframe', 'object', 'embed'],
    FORBID_ATTR: ['on*', 'href', 'xlink:href', 'src', 'srcset'],
    RETURN_TRUSTED_TYPE: false,
  }) as string

  const template = document.createElement('template')
  template.innerHTML = sanitized
  const root = template.content.firstElementChild
  if (!root || root.tagName.toLowerCase() !== 'svg' || template.content.children.length !== 1) {
    throwFailure('unsafe-output')
  }

  const elements = [root, ...Array.from(root.querySelectorAll<HTMLElement>('*'))]
  for (const element of elements) {
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase()
      const value = attribute.value.trim()

      // Namespace declarations identify XML elements; they are not fetch URLs.
      // Only the canonical default SVG namespace is needed in this image.
      if (name === 'xmlns' || name.startsWith('xmlns:')) {
        if (name !== 'xmlns' || value !== SVG_NAMESPACE) element.removeAttribute(attribute.name)
        continue
      }

      if (
        name.startsWith('on')
        || name === 'href'
        || name === 'xlink:href'
        || name === 'src'
        || name === 'srcset'
        || /^(?:https?:|data:|file:|javascript:|\/\/)/i.test(value)
        || /\\|\/\*/.test(value)
        || !hasOnlyInternalSvgUrls(value)
      ) {
        element.removeAttribute(attribute.name)
      }
    }
  }

  for (const style of Array.from(root.querySelectorAll('style'))) {
    const css = style.textContent ?? ''
    if (
      /@import/i.test(css)
      || /\\|\/\*/.test(css)
      || /(?:https?:|data:|file:|javascript:|\/\/)/i.test(css)
      || !hasOnlyInternalSvgUrls(css)
    ) {
      style.remove()
    }
  }

  if (root.querySelector('script,foreignObject,image,iframe,object,embed')) {
    throwFailure('unsafe-output')
  }

  root.setAttribute('xmlns', SVG_NAMESPACE)
  const serialized = root.outerHTML
  const xml = new DOMParser().parseFromString(serialized, 'image/svg+xml')
  if (xml.querySelector('parsererror') || xml.documentElement.namespaceURI !== SVG_NAMESPACE) {
    throwFailure('unsafe-output')
  }

  const altText = [
    root.querySelector('title')?.textContent?.trim(),
    root.querySelector('desc')?.textContent?.trim(),
  ]
    .filter((text): text is string => Boolean(text))
    .join(' — ')
    .slice(0, 240) || 'Mermaid図'

  return {
    svg: serialized,
    altText,
  }
}

async function renderMermaidDiagramNow(
  source: string,
  options: MermaidRenderOptions,
): Promise<MermaidRenderResult> {
  const validation = validateMermaidSource(source)
  if (validation) return validation

  let api: MermaidApi
  try {
    api = options.mermaid ?? await loadMermaid()
  } catch {
    return failure('unavailable')
  }

  try {
    api.initialize(getMermaidConfig(getTheme(options.theme)))
    const parsed = await api.parse(source, { suppressErrors: true })
    if (parsed === false) throwFailure('invalid')

    const rendered = await api.render(`atlas-mermaid-${++renderId}`, source)
    if (!rendered || typeof rendered.svg !== 'string' || rendered.svg.trim().length === 0) {
      throwFailure('invalid')
    }

    return {
      ok: true,
      ...sanitizeSvg(rendered.svg),
    }
  } catch (error) {
    if (error instanceof MermaidRenderFailure) return failure(error.code)
    return failure('invalid')
  }
}

export function renderMermaidDiagram(
  source: string,
  options: MermaidRenderOptions = {},
): Promise<MermaidRenderResult> {
  const job = () => renderMermaidDiagramNow(source, options)
  const result = renderQueue.then(job, job)
  renderQueue = result.then(() => undefined, () => undefined)
  return result
}
