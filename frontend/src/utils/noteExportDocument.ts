import { parseNoteLinkHref } from './noteLink'

const MAX_DOM_DEPTH = 100
const MAX_FONT_BYTES = 8 * 1024 * 1024
const PDF_CONTENT_WIDTH = 499
const FONT_REGULAR_FILE = 'NotoSansJP-Regular.otf'
const FONT_BOLD_FILE = 'NotoSansJP-Bold.otf'

const discardedElementNames = new Set([
  'audio',
  'base',
  'button',
  'canvas',
  'embed',
  'form',
  'iframe',
  'noscript',
  'object',
  'script',
  'select',
  'source',
  'style',
  'svg',
  'template',
  'textarea',
  'track',
  'video',
])

const blockElementNames = new Set([
  'article',
  'blockquote',
  'div',
  'figcaption',
  'figure',
  'footer',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'header',
  'hr',
  'main',
  'ol',
  'p',
  'pre',
  'section',
  'table',
  'ul',
])

const inlineExcludedElementNames = new Set(['ol', 'pre', 'table', 'ul'])

type PdfMargin = [number, number, number, number]

export type PdfInline = {
  text: string
  bold?: boolean
  italics?: boolean
  decoration?: 'lineThrough'
  link?: string
  style?: string
}

export type PdfTextContent = {
  text: string | PdfInline[]
  style?: string
  bold?: boolean
  margin?: PdfMargin
  preserveLeadingSpaces?: boolean
}

export type PdfStackContent = {
  stack: PdfContent[]
  style?: string
  margin?: PdfMargin
}

export type PdfListContent = {
  ul?: PdfListItem[]
  ol?: PdfListItem[]
  start?: number
  margin?: PdfMargin
}

export type PdfTableContent = {
  table: {
    headerRows: number
    widths: string[]
    body: PdfTextContent[][]
  }
  layout: 'lightHorizontalLines'
  margin?: PdfMargin
}

export type PdfCanvasContent = {
  canvas: Array<{
    type: 'line'
    x1: number
    y1: number
    x2: number
    y2: number
    lineWidth: number
    lineColor: string
  }>
  margin?: PdfMargin
}

export type PdfContent =
  | PdfTextContent
  | PdfStackContent
  | PdfListContent
  | PdfTableContent
  | PdfCanvasContent

export type PdfListItem = PdfTextContent | PdfStackContent | PdfListContent

export type PdfDocumentDefinition = {
  info: {
    title: string
    author: string
    creator: string
    producer: string
  }
  pageSize: 'A4'
  pageOrientation: 'portrait'
  pageMargins: PdfMargin
  defaultStyle: {
    font: 'NotoSansJP'
    fontSize: number
    lineHeight: number
    color: string
  }
  styles: Record<string, Record<string, unknown>>
  content: PdfContent[]
}

type InlineMarks = Omit<PdfInline, 'text'>
type PdfMakeInstance = (typeof import('pdfmake/build/pdfmake'))['default']

let pdfMakePromise: Promise<PdfMakeInstance> | null = null

export function createPdfDocumentDefinition(html: string, title: string): PdfDocumentDefinition {
  const parser = new DOMParser()
  const document = parser.parseFromString(html, 'text/html')
  const content = convertContainer(document.body, 0)

  return {
    info: {
      title: normalizeDocumentTitle(title),
      author: 'Atlas Note',
      creator: 'Atlas Note',
      producer: 'Atlas Note',
    },
    pageSize: 'A4',
    pageOrientation: 'portrait',
    pageMargins: [48, 48, 48, 48],
    defaultStyle: {
      font: 'NotoSansJP',
      fontSize: 10.5,
      lineHeight: 1.4,
      color: '#1f2937',
    },
    styles: {
      heading1: { fontSize: 24, bold: true, lineHeight: 1.25 },
      heading2: { fontSize: 20, bold: true, lineHeight: 1.25 },
      heading3: { fontSize: 17, bold: true, lineHeight: 1.3 },
      heading4: { fontSize: 14.5, bold: true, lineHeight: 1.3 },
      heading5: { fontSize: 12.5, bold: true, lineHeight: 1.35 },
      heading6: { fontSize: 11, bold: true, lineHeight: 1.35 },
      paragraph: { lineHeight: 1.4 },
      blockquote: { color: '#4b5563', italics: true },
      codeBlock: {
        background: '#f3f4f6',
        color: '#111827',
        fontSize: 9,
        lineHeight: 1.25,
      },
      inlineCode: { background: '#f3f4f6', color: '#111827', fontSize: 9.5 },
    },
    content: content.length > 0 ? content : [{ text: '', style: 'paragraph' }],
  }
}

export async function createPdfBase64FromHtml(html: string, title: string): Promise<string> {
  const documentDefinition = createPdfDocumentDefinition(html, title)
  const pdfMake = await getPdfMake()
  const buffer = await pdfMake.createPdf(documentDefinition).getBuffer()
  if (buffer.byteLength === 0) {
    throw new Error('PDFの生成結果が空です。')
  }
  return bytesToBase64(buffer)
}

function convertContainer(container: ParentNode, depth: number): PdfContent[] {
  assertDepth(depth)
  const content: PdfContent[] = []
  let pendingInline: PdfInline[] = []

  const flushInline = () => {
    const inline = trimInline(pendingInline)
    pendingInline = []
    if (inline.length > 0) {
      content.push({ text: inline, style: 'paragraph', margin: [0, 0, 0, 8] })
    }
  }

  for (const child of Array.from(container.childNodes)) {
    if (child.nodeType === 1) {
      const element = child as Element
      if (shouldDiscardElement(element)) continue
      if (blockElementNames.has(element.localName)) {
        flushInline()
        content.push(...convertBlockElement(element, depth + 1))
        continue
      }
    }
    pendingInline.push(...collectInline(child, {}, depth + 1))
  }

  flushInline()
  return content
}

function convertBlockElement(element: Element, depth: number): PdfContent[] {
  assertDepth(depth)
  const name = element.localName

  if (/^h[1-6]$/.test(name)) {
    const level = Number(name.slice(1))
    return [
      {
        text: trimInline(collectInlineChildren(element, {}, depth + 1)),
        style: `heading${level}`,
        margin: [0, level === 1 ? 0 : 8, 0, 6],
      },
    ]
  }

  if (name === 'p') {
    return [
      {
        text: trimInline(collectInlineChildren(element, {}, depth + 1)),
        style: 'paragraph',
        margin: [0, 0, 0, 8],
      },
    ]
  }

  if (name === 'blockquote') {
    const stack = convertContainer(element, depth + 1)
    return [
      {
        stack: stack.length > 0 ? stack : [{ text: '', style: 'paragraph' }],
        style: 'blockquote',
        margin: [16, 4, 8, 10],
      },
    ]
  }

  if (name === 'ul' || name === 'ol') {
    return [convertList(element, name === 'ol', depth + 1)]
  }

  if (name === 'pre') {
    return [
      {
        text: element.textContent ?? '',
        style: 'codeBlock',
        preserveLeadingSpaces: true,
        margin: [0, 4, 0, 10],
      },
    ]
  }

  if (name === 'hr') {
    return [
      {
        canvas: [
          {
            type: 'line',
            x1: 0,
            y1: 0,
            x2: PDF_CONTENT_WIDTH,
            y2: 0,
            lineWidth: 0.5,
            lineColor: '#9ca3af',
          },
        ],
        margin: [0, 8, 0, 10],
      },
    ]
  }

  if (name === 'table') {
    return [convertTable(element, depth + 1)]
  }

  return convertContainer(element, depth + 1)
}

function convertList(element: Element, ordered: boolean, depth: number): PdfListContent {
  assertDepth(depth)
  const items = Array.from(element.children)
    .filter((child) => child.localName === 'li')
    .map((item) => convertListItem(item, depth + 1))

  const list: PdfListContent = {
    margin: [12, 0, 0, 8],
    ...(ordered ? { ol: items } : { ul: items }),
  }
  if (ordered) {
    const start = Number(element.getAttribute('start'))
    if (Number.isSafeInteger(start) && start > 0) list.start = start
  }
  return list
}

function convertListItem(element: Element, depth: number): PdfListItem {
  assertDepth(depth)
  const stack: PdfContent[] = []
  let inline: PdfInline[] = []
  const checkbox = findOwnedCheckbox(element)
  const taskState = element.getAttribute('data-checked')
  if (checkbox !== null || taskState === 'true' || taskState === 'false') {
    const checked = checkbox?.hasAttribute('checked') === true || taskState === 'true'
    appendInline(inline, { text: checked ? '[x] ' : '[ ] ' })
  }

  const flushInline = () => {
    const text = trimInline(inline)
    inline = []
    if (text.length > 0) stack.push({ text, style: 'paragraph' })
  }

  for (const child of Array.from(element.childNodes)) {
    if (child.nodeType === 1) {
      const childElement = child as Element
      if (shouldDiscardElement(childElement)) continue
      if (childElement.localName === 'p') {
        inline.push(...collectInlineChildren(childElement, {}, depth + 1))
        flushInline()
        continue
      }
      if (childElement.localName === 'ul' || childElement.localName === 'ol') {
        flushInline()
        stack.push(convertList(childElement, childElement.localName === 'ol', depth + 1))
        continue
      }
      if (blockElementNames.has(childElement.localName)) {
        flushInline()
        stack.push(...convertBlockElement(childElement, depth + 1))
        continue
      }
    }
    inline.push(...collectInline(child, {}, depth + 1))
  }
  flushInline()
  return { stack: stack.length > 0 ? stack : [{ text: '', style: 'paragraph' }] }
}

function convertTable(element: Element, depth: number): PdfContent {
  assertDepth(depth)
  const rows = Array.from(element.querySelectorAll('tr')).filter(
    (row) => row.closest('table') === element,
  )
  const parsedRows = rows.map((row) =>
    Array.from(row.children).filter(
      (cell) => cell.localName === 'th' || cell.localName === 'td',
    ),
  )
  const columnCount = parsedRows[0]?.length ?? 0
  const isSimple =
    columnCount > 0 &&
    parsedRows.every(
      (row) =>
        row.length === columnCount &&
        row.every((cell) => isSingleSpan(cell, 'rowspan') && isSingleSpan(cell, 'colspan')),
    )

  if (!isSimple) {
    const lines = parsedRows.map((row) =>
      row
        .map((cell) => inlinePlainText(trimInline(collectInlineChildren(cell, {}, depth + 1))))
        .join(' | '),
    )
    return {
      text: lines.join('\n'),
      style: 'paragraph',
      margin: [0, 4, 0, 10],
    }
  }

  const body = parsedRows.map((row, rowIndex) =>
    row.map((cell) => ({
      text: trimInline(collectInlineChildren(cell, {}, depth + 1)),
      bold: rowIndex === 0 || cell.localName === 'th',
    })),
  )
  return {
    table: {
      headerRows: 1,
      widths: Array.from({ length: columnCount }, () => '*'),
      body,
    },
    layout: 'lightHorizontalLines',
    margin: [0, 4, 0, 10],
  }
}

function collectInlineChildren(
  parent: ParentNode,
  marks: InlineMarks,
  depth: number,
): PdfInline[] {
  const inline: PdfInline[] = []
  for (const child of Array.from(parent.childNodes)) {
    inline.push(...collectInline(child, marks, depth + 1))
  }
  return inline
}

function collectInline(node: Node, marks: InlineMarks, depth: number): PdfInline[] {
  assertDepth(depth)
  if (node.nodeType === 3) {
    const text = normalizeInlineWhitespace(node.nodeValue ?? '')
    return text === '' ? [] : [{ text, ...marks }]
  }
  if (node.nodeType !== 1) return []

  const element = node as Element
  if (shouldDiscardElement(element)) return []
  const name = element.localName
  if (inlineExcludedElementNames.has(name)) return []

  if (name === 'br') return [{ text: '\n', ...marks }]
  if (name === 'img') {
    const alt = normalizeInlineWhitespace(element.getAttribute('alt') ?? '').trim()
    return alt === '' ? [] : [{ text: alt, ...marks }]
  }
  if (name === 'input') return []

  if (name === 'strong' || name === 'b') {
    return collectInlineChildren(element, { ...marks, bold: true }, depth + 1)
  }
  if (name === 'em' || name === 'i') {
    return collectInlineChildren(element, { ...marks, italics: true }, depth + 1)
  }
  if (name === 'del' || name === 's' || name === 'strike') {
    return collectInlineChildren(element, { ...marks, decoration: 'lineThrough' }, depth + 1)
  }
  if (name === 'code') {
    return collectInlineChildren(element, { ...marks, style: 'inlineCode' }, depth + 1)
  }
  if (name === 'a') {
    const href = safeLinkHref(element.getAttribute('href') ?? '')
    const linkMarks = href === null ? marks : { ...marks, link: href }
    return collectInlineChildren(element, linkMarks, depth + 1)
  }

  return collectInlineChildren(element, marks, depth + 1)
}

function shouldDiscardElement(element: Element) {
  return element.hasAttribute('hidden') || discardedElementNames.has(element.localName)
}

function safeLinkHref(value: string): string | null {
  if (value === '' || value !== value.trim() || /[\u0000-\u001f\u007f\s<>"']/.test(value)) {
    return null
  }
  if (parseNoteLinkHref(value) !== null) return value
  if (/^#[A-Za-z0-9][A-Za-z0-9_.:~-]*$/.test(value)) return value
  if (/^(?:mailto|tel):[^\s<>"']+$/i.test(value)) return value
  if (!/^https?:/i.test(value)) return null

  try {
    const parsed = new URL(value)
    if (parsed.username !== '' || parsed.password !== '') return null
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? value : null
  } catch {
    return null
  }
}

function findOwnedCheckbox(element: Element): Element | null {
  return (
    Array.from(element.querySelectorAll('input[type="checkbox"]')).find(
      (input) => input.closest('li') === element,
    ) ?? null
  )
}

function isSingleSpan(element: Element, attribute: 'rowspan' | 'colspan') {
  const value = element.getAttribute(attribute)
  return value === null || value === '' || value === '1'
}

function normalizeInlineWhitespace(value: string) {
  return value.replace(/\u00a0/g, ' ').replace(/[\t\n\r\f ]+/g, ' ')
}

function normalizeDocumentTitle(value: string) {
  const normalized = value.replace(/[\u0000-\u001f\u007f]+/g, ' ').trim()
  return normalized === '' ? '無題' : normalized
}

function trimInline(value: PdfInline[]) {
  const result = value.map((item) => ({ ...item }))
  while (result.length > 0) {
    result[0].text = result[0].text.replace(/^\s+/, '')
    if (result[0].text !== '') break
    result.shift()
  }
  while (result.length > 0) {
    const last = result.length - 1
    result[last].text = result[last].text.replace(/\s+$/, '')
    if (result[last].text !== '') break
    result.pop()
  }
  return result
}

function appendInline(target: PdfInline[], item: PdfInline) {
  const previous = target[target.length - 1]
  if (previous && sameInlineStyle(previous, item)) {
    previous.text += item.text
    return
  }
  target.push(item)
}

function sameInlineStyle(left: PdfInline, right: PdfInline) {
  return (
    left.bold === right.bold &&
    left.italics === right.italics &&
    left.decoration === right.decoration &&
    left.link === right.link &&
    left.style === right.style
  )
}

function inlinePlainText(value: PdfInline[]) {
  return value.map((item) => item.text).join('')
}

function assertDepth(depth: number) {
  if (depth > MAX_DOM_DEPTH) {
    throw new Error('PDFへ変換するHTMLの入れ子が深すぎます。')
  }
}

async function getPdfMake(): Promise<PdfMakeInstance> {
  if (pdfMakePromise === null) {
    pdfMakePromise = initializePdfMake().catch((error: unknown) => {
      pdfMakePromise = null
      throw error
    })
  }
  return pdfMakePromise
}

async function initializePdfMake(): Promise<PdfMakeInstance> {
  const [pdfMakeModule, regularModule, boldModule, fontLicenseModule, pdfMakeLicenseModule] = await Promise.all([
    import('pdfmake/build/pdfmake'),
    import('../assets/fonts/NotoSansJP-Regular.otf?url'),
    import('../assets/fonts/NotoSansJP-Bold.otf?url'),
    import('../assets/fonts/OFL.txt?url'),
    import('../assets/licenses/pdfmake-MIT.txt?url'),
  ])
  void fontLicenseModule.default
  void pdfMakeLicenseModule.default

  const [regularBase64, boldBase64] = await Promise.all([
    fetchAssetBase64(regularModule.default),
    fetchAssetBase64(boldModule.default),
  ])
  const pdfMake = pdfMakeModule.default
  pdfMake.setUrlAccessPolicy(() => false)
  pdfMake.addVirtualFileSystem({
    [FONT_REGULAR_FILE]: regularBase64,
    [FONT_BOLD_FILE]: boldBase64,
  })
  pdfMake.addFonts({
    NotoSansJP: {
      normal: FONT_REGULAR_FILE,
      bold: FONT_BOLD_FILE,
      italics: FONT_REGULAR_FILE,
      bolditalics: FONT_BOLD_FILE,
    },
  })
  return pdfMake
}

async function fetchAssetBase64(url: string) {
  const response = await fetch(url, { credentials: 'same-origin', redirect: 'error' })
  if (!response.ok) throw new Error('PDF用フォントを読み込めませんでした。')

  const buffer = await response.arrayBuffer()
  if (buffer.byteLength === 0 || buffer.byteLength > MAX_FONT_BYTES) {
    throw new Error('PDF用フォントのサイズが不正です。')
  }
  return bytesToBase64(new Uint8Array(buffer))
}

function bytesToBase64(value: Uint8Array) {
  const chunkSize = 0x8000
  let binary = ''
  for (let offset = 0; offset < value.length; offset += chunkSize) {
    binary += String.fromCharCode(...value.subarray(offset, offset + chunkSize))
  }
  return btoa(binary)
}
