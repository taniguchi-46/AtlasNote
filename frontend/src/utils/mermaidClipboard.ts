export type MermaidClipboardPayload =
  | {
      kind: 'markdown'
      markdown: string
    }
  | {
      kind: 'source'
      source: string
    }

const MERMAID_FENCE_LINE = /^[ \t]{0,3}(`{3,}|~{3,})[ \t]*mermaid(?:[ \t]+[^\r\n]*)?[ \t]*$/i
const MERMAID_START_PATTERNS = [
  /^(?:flowchart|graph)\s+(?:tb|td|bt|rl|lr)\b/i,
  /^(?:sequenceDiagram|classDiagram(?:-v2)?|stateDiagram(?:-v2)?|erDiagram|journey|gantt|pie|quadrantChart|requirementDiagram|gitGraph|mindmap|timeline|zenuml|sankey|xychart-beta|block-beta|architecture-beta|c4context|packet-beta|kanban|radar|treemap)(?:\s|$)/i,
]

function hasClipboardType(data: DataTransfer, type: string) {
  return Array.from(data.types ?? []).some((entry) => entry.toLowerCase() === type)
}

function getClipboardData(data: DataTransfer, type: string) {
  if (!hasClipboardType(data, type)) return ''
  try {
    return data.getData(type)
  } catch {
    return ''
  }
}

function hasMermaidFence(markdown: string) {
  return markdown.split(/\r\n|\n|\r/).some((line) => MERMAID_FENCE_LINE.test(line))
}

function hasMermaidStart(source: string) {
  const normalized = source.replace(/^\uFEFF/, '').trimStart()
  return MERMAID_START_PATTERNS.some((pattern) => pattern.test(normalized))
}

function sourcePayload(source: string): MermaidClipboardPayload | null {
  if (!source.trim() || !hasMermaidStart(source)) return null
  return { kind: 'source', source }
}

type ParsedHtmlMermaidSource = {
  source: string | null
  allowPlainSource: boolean
}

const VOID_HTML_ELEMENTS = new Set([
  'area',
  'base',
  'br',
  'col',
  'embed',
  'hr',
  'img',
  'input',
  'link',
  'meta',
  'param',
  'source',
  'track',
  'wbr',
])

function removeEmptyHtmlContainers(element: Element) {
  for (const child of Array.from(element.children)) {
    removeEmptyHtmlContainers(child)
  }

  for (const child of Array.from(element.children)) {
    const tagName = child.tagName.toLowerCase()
    if (VOID_HTML_ELEMENTS.has(tagName)) continue
    if (child.children.length === 0 && !(child.textContent ?? '').trim()) {
      child.remove()
    }
  }
}

function isStandaloneMermaidCode(document: Document, codeElement: HTMLElement) {
  const body = document.body.cloneNode(true) as HTMLElement
  const codeElements = Array.from(body.querySelectorAll('code'))
  const codeIndex = Array.from(document.querySelectorAll('code')).indexOf(codeElement)
  const clonedCode = codeElements[codeIndex]
  if (!clonedCode) return false

  clonedCode.remove()
  removeEmptyHtmlContainers(body)
  return !body.querySelector('*') && !(body.textContent ?? '').trim()
}

function normalizeClipboardComparisonText(source: string) {
  return source.replace(/\r\n?/g, '\n').replace(/\u00a0/g, ' ').trim()
}

function parseHtmlMermaidSource(html: string, plainText: string): ParsedHtmlMermaidSource {
  if (typeof DOMParser === 'undefined') return { source: null, allowPlainSource: true }

  const document = new DOMParser().parseFromString(html, 'text/html')
  if (document.querySelector('svg, .mermaid')) return { source: null, allowPlainSource: false }

  const allCodeElements = Array.from(document.querySelectorAll('code'))
  const codeElements = allCodeElements.filter((element) => {
    const languageClass = Array.from(element.classList)
      .some((className) => className.toLowerCase() === 'language-mermaid')
    const languageAttribute = element.getAttribute('data-language')?.trim().toLowerCase()
    return languageClass || languageAttribute === 'mermaid'
  })
  if (codeElements.length === 0) {
    if (allCodeElements.length > 0) return { source: null, allowPlainSource: false }

    // If rich HTML contains surrounding text, accepting the plain-text source
    // would silently drop that text. Only allow the plain-text classifier when
    // the HTML carries the same text and no code element needs preserving.
    return {
      source: null,
      allowPlainSource: normalizeClipboardComparisonText(document.body.textContent ?? '')
        === normalizeClipboardComparisonText(plainText),
    }
  }

  if (codeElements.length !== 1 || !isStandaloneMermaidCode(document, codeElements[0])) {
    return { source: null, allowPlainSource: false }
  }

  const source = codeElements[0].textContent ?? ''
  if (!source.trim() || !hasMermaidStart(source)) {
    return { source: null, allowPlainSource: false }
  }

  return {
    source,
    allowPlainSource: false,
  }
}

export function readMermaidClipboardPayload(data: DataTransfer | null | undefined) {
  if (!data) return null

  const text = getClipboardData(data, 'text/plain')
  if (text && hasMermaidFence(text)) {
    return { kind: 'markdown', markdown: text } satisfies MermaidClipboardPayload
  }

  const html = getClipboardData(data, 'text/html')
  if (html) {
    const parsed = parseHtmlMermaidSource(html, text)
    if (!parsed.allowPlainSource) {
      if (parsed.source === null) return null
      return { kind: 'source', source: parsed.source } satisfies MermaidClipboardPayload
    }
    if (parsed.source !== null) {
      return { kind: 'source', source: parsed.source } satisfies MermaidClipboardPayload
    }
  }

  if (!text) return null
  return sourcePayload(text)
}

function longestBacktickRun(source: string) {
  let longest = 0
  for (const match of source.matchAll(/`+/g)) {
    longest = Math.max(longest, match[0].length)
  }
  return longest
}

export function createMermaidFence(source: string, language = 'mermaid') {
  const fence = '`'.repeat(Math.max(3, longestBacktickRun(source) + 1))
  return `${fence}${language}\n${source}\n${fence}`
}
