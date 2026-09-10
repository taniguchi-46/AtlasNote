// Run from a blank page served by Vite (no Wails/application startup):
// await (await import('/scripts/mermaid-browser-checks.mjs')).runMermaidBrowserChecks()
import { renderMermaidDiagram } from '../src/utils/mermaidRenderer.ts'
import mermaid from 'mermaid'
import { unsafeSources } from './mermaid-unsafe-sources.mjs'

export async function runMermaidBrowserChecks() {
  const check = (value, message) => { if (!value) throw new Error(message) }
  let apiCalls = 0, imageAttempts = 0
  const OriginalImage = window.Image
  const srcDescriptor = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'src')
  // Fail closed locally even if input validation regresses; never issue a request.
  window.Image = class { constructor() { imageAttempts++; throw new Error('Forbidden Image') } }
  Object.defineProperty(HTMLImageElement.prototype, 'src', {
    configurable: true,
    get: srcDescriptor.get,
    set() { imageAttempts++; throw new Error('Forbidden image src') },
  })
  try {
    const guardedApi = Object.fromEntries(['initialize', 'parse', 'render'].map(name => [name, (...args) => {
      apiCalls++
      return mermaid[name](...args)
    }]))
    for (const source of unsafeSources) {
      const result = await renderMermaidDiagram(source, { mermaid: guardedApi })
      check(!result.ok && result.code === 'unsafe-syntax', 'Unsafe source reached renderer')
    }
    check(apiCalls === 0 && imageAttempts === 0, 'Pre-render rejection failed')
  } finally {
    window.Image = OriginalImage
    Object.defineProperty(HTMLImageElement.prototype, 'src', srcDescriptor)
  }
  const dimensions = []
  for (const theme of ['light', 'dark']) {
    for (const source of ['flowchart TD\n A[Start] --> B[End]', 'sequenceDiagram\n A->>B: Hello']) {
      const result = await renderMermaidDiagram(source, { theme })
      check(result.ok, `Real render failed: ${result.code}`)
      const xml = new DOMParser().parseFromString(result.svg, 'image/svg+xml')
      check(xml.documentElement.namespaceURI === 'http://www.w3.org/2000/svg', 'Missing SVG namespace')
      check(!xml.querySelector('parsererror'), 'Invalid SVG XML')
      const url = URL.createObjectURL(new Blob([result.svg], { type: 'image/svg+xml' }))
      try {
        const img = new Image()
        img.src = url
        await img.decode()
        check(img.naturalWidth > 0 && img.naturalHeight > 0, 'Blob image did not load')
        dimensions.push([theme, img.naturalWidth, img.naturalHeight])
        img.style.width = `${img.naturalWidth}px`
        img.style.height = `${img.naturalHeight}px`
        img.alt = `${theme}: ${source.split('\n')[0]}`
        document.body.append(img)
      } finally {
        URL.revokeObjectURL(url)
      }
    }
  }
  return { rejected: unsafeSources.length, apiCalls, imageAttempts, dimensions }
}
