export const RICH_MARKDOWN_OPTIONS = {
  html: false,
  linkify: true,
  breaks: true,
} as const

type MarkdownBlockToken = {
  level?: number
  map?: [number, number] | null
  nesting?: number
  type?: string
}

type MarkdownParserWithTokens = {
  md?: {
    parse(markdown: string, environment: Record<string, never>): MarkdownBlockToken[]
  }
}

export function preserveMarkdownBlankLines(
  markdown: string,
  html: string,
  parser: MarkdownParserWithTokens,
): string {
  const blocks = getTopLevelMarkdownBlocks(markdown, parser)
  if (blocks.length === 0) return html

  const container = document.createElement('div')
  container.innerHTML = html
  const elements = [...container.children]
  if (elements.length !== blocks.length) return html

  const emptyParagraph = () => document.createElement('p')
  const firstBlockStart = blocks[0].map![0]
  for (let index = 0; index < firstBlockStart; index += 1) {
    container.insertBefore(emptyParagraph(), elements[0])
  }

  for (let index = blocks.length - 1; index > 0; index -= 1) {
    const currentBlockStart = blocks[index].map![0]
    const previousBlockEnd = blocks[index - 1].map![1]
    const blankLineCount = currentBlockStart - previousBlockEnd

    for (let blankLine = 0; blankLine < blankLineCount; blankLine += 1) {
      container.insertBefore(emptyParagraph(), elements[index])
    }
  }

  const lines = markdown.split(/\r\n|\r|\n/)
  const trailingBlankLineCount = lines.length - blocks[blocks.length - 1].map![1]
  for (let index = 0; index < trailingBlankLineCount; index += 1) {
    container.appendChild(emptyParagraph())
  }

  return container.innerHTML
}

function getTopLevelMarkdownBlocks(markdown: string, parser: MarkdownParserWithTokens) {
  const tokens = parser.md?.parse(markdown, {}) ?? []

  return tokens.filter((token) => {
    if (token.level !== 0 || !token.map) return false

    return token.nesting === 1
      || token.type === 'fence'
      || token.type === 'code_block'
      || token.type === 'hr'
  })
}
