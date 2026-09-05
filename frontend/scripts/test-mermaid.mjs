import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { JSDOM } from 'jsdom'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'mermaidRenderer.ts')
const outDir = path.join(rootDir, '.tmp', 'mermaid-test')
const outFile = path.join(outDir, 'mermaidRenderer.mjs')

const dom = new JSDOM('<!doctype html><html data-theme="light"><body></body></html>', {
  url: 'https://atlasnote.test/',
})
class CSSStyleSheetStub {
  constructor() {
    this.cssRules = []
  }

  insertRule() {
    this.cssRules.push({})
  }
}

Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  DOMParser: dom.window.DOMParser,
  Element: dom.window.Element,
  HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement,
  navigator: dom.window.navigator,
  CSSStyleSheet: CSSStyleSheetStub,
})
Object.defineProperty(document, 'adoptedStyleSheets', { value: [], writable: true })
dom.window.SVGElement.prototype.getComputedTextLength = function getComputedTextLength() {
  return (this.textContent ?? '').length * 8
}
dom.window.SVGElement.prototype.getBBox = function getBBox() {
  return {
    x: 0,
    y: 0,
    width: (this.textContent ?? '').length * 8,
    height: 20,
  }
}

await mkdir(outDir, { recursive: true })
const source = await readFile(sourcePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
    importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
  },
})
await writeFile(outFile, compiled.outputText, 'utf8')

const {
  MERMAID_LIMITS,
  renderMermaidDiagram,
  validateMermaidSource,
} = await import(pathToFileUrl(outFile))

assert.equal(validateMermaidSource('')?.code, 'empty')
assert.equal(validateMermaidSource('x'.repeat(MERMAID_LIMITS.maxTextSize + 1))?.code, 'too-large')
assert.equal(validateMermaidSource('%%{init: {"theme":"dark"}}%%\nflowchart TD')?.code, 'unsafe-syntax')
assert.equal(validateMermaidSource('flowchart TD\n  A --> B\n  click A callMe')?.code, 'unsafe-syntax')
assert.equal(validateMermaidSource('flowchart TD\n  A@{ img: "https://example.test/a.png" }')?.code, 'unsafe-syntax')
assert.equal(validateMermaidSource('flowchart TD\n  A@{ icon: "fa:user" }')?.code, 'unsafe-syntax')
assert.equal(validateMermaidSource('flowchart TD\n  link A "https://example.test"')?.code, 'unsafe-syntax')
assert.equal(validateMermaidSource('flowchart TD\n  A --> B'), null)

const initializedConfigs = []
const parsedSources = []
const renderedSources = []
const fakeMermaid = {
  initialize(config) {
    initializedConfigs.push(config)
  },
  async parse(source) {
    parsedSources.push(source)
    return source !== 'invalid'
  },
  async render(id, source) {
    renderedSources.push({ id, source })
    return {
      svg: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 80 30">'
        + '<title>開始から終了</title><desc>処理の流れ</desc>'
        + '<defs><marker id="arrow" style="marker-end:url(#arrow)"></marker></defs>'
        + '<rect width="80" height="30" fill="#fff"></rect></svg>',
    }
  },
}

const success = await renderMermaidDiagram('flowchart TD\n  A --> B', {
  theme: 'light',
  mermaid: fakeMermaid,
})
assert.equal(success.ok, true)
assert.match(success.svg, /^<svg\b/)
assert.equal(success.altText, '開始から終了 — 処理の流れ')
assert.equal(initializedConfigs[0].startOnLoad, false)
assert.equal(initializedConfigs[0].securityLevel, 'strict')
assert.equal(initializedConfigs[0].htmlLabels, false)
assert.equal(initializedConfigs[0].suppressErrorRendering, true)
assert.equal(initializedConfigs[0].maxTextSize, MERMAID_LIMITS.maxTextSize)
assert.equal(initializedConfigs[0].maxEdges, MERMAID_LIMITS.maxEdges)
assert.equal(initializedConfigs[0].theme, 'default')
assert.equal(parsedSources.length, 1)
assert.equal(renderedSources.length, 1)

const oversized = await renderMermaidDiagram('x'.repeat(MERMAID_LIMITS.maxTextSize + 1), {
  mermaid: fakeMermaid,
})
assert.equal(oversized.ok, false)
assert.equal(oversized.code, 'too-large')
assert.equal(parsedSources.length, 1)

const invalid = await renderMermaidDiagram('invalid', { mermaid: fakeMermaid })
assert.equal(invalid.ok, false)
assert.equal(invalid.code, 'invalid')

const unsafeOutputApi = {
  ...fakeMermaid,
  async render() {
    return {
      svg: '<svg onload="alert(1)"><script>alert(1)</script>'
        + '<foreignObject><div>unsafe</div></foreignObject></svg>',
    }
  },
}
const unsafeOutput = await renderMermaidDiagram('flowchart TD\n  A --> B', {
  mermaid: unsafeOutputApi,
})
assert.equal(unsafeOutput.ok, false)
assert.equal(unsafeOutput.code, 'unsafe-output')

const queueEvents = []
const queueIds = []
const queueApi = {
  initialize() {},
  async parse() {},
  async render(id, source) {
    queueIds.push(id)
    queueEvents.push(`start:${source}`)
    await new Promise(resolve => setTimeout(resolve, source === 'slow' ? 20 : 0))
    queueEvents.push(`end:${source}`)
    return { svg: `<svg data-source="${source}" id="${id}"></svg>` }
  },
}
const [slow, fast] = await Promise.all([
  renderMermaidDiagram('slow', { mermaid: queueApi }),
  renderMermaidDiagram('fast', { mermaid: queueApi }),
])
assert.equal(slow.ok, true)
assert.equal(fast.ok, true)
assert.deepEqual(queueEvents, ['start:slow', 'end:slow', 'start:fast', 'end:fast'])
assert.notEqual(queueIds[0], queueIds[1])

const realMermaid = (await import('mermaid')).default
const real = await renderMermaidDiagram('flowchart TD\n  A[Start] --> B[End]', {
  theme: 'light',
  mermaid: realMermaid,
})
assert.equal(real.ok, true)
assert.match(real.svg, /^<svg\b/)
assert.doesNotMatch(real.svg, /<foreignObject\b|<script\b/i)

await rm(outDir, { recursive: true, force: true })
console.log('Mermaid renderer tests passed')

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}
