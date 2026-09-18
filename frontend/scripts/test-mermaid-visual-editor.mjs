import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import ts from 'typescript'
import { pathToFileURL } from 'node:url'

const rootDir = process.cwd()
const outDir = path.join(rootDir, '.tmp', 'mermaid-visual-editor-test')
const outFile = path.join(outDir, 'mermaidVisualEditor.mjs')
await mkdir(outDir, { recursive: true })

try {
  const source = await readFile(path.join(rootDir, 'src', 'utils', 'mermaidVisualEditor.ts'), 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2020,
      importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
    },
  }).outputText
  await writeFile(outFile, compiled, 'utf8')

  const visualEditor = await import(pathToFileURL(outFile))
  const {
    MERMAID_DIAGRAM_CATALOG,
    createMermaidElement,
    detectMermaidDiagramType,
    flowchartConnectionCount,
    generateMermaidSource,
    isMermaidEditorValueSafe,
    parseMermaidVisualSource,
  } = visualEditor

  assert.ok(MERMAID_DIAGRAM_CATALOG.length >= 30, 'the catalog must cover the Mermaid 11.17.2 diagrams')
  const seenTypes = new Set()

  for (const definition of MERMAID_DIAGRAM_CATALOG) {
    seenTypes.add(definition.type)
    assert.equal(detectMermaidDiagramType(definition.sample), definition.type, definition.type)
    const parsed = parseMermaidVisualSource(definition.sample)
    assert.ok(parsed, `${definition.type} sample must have a visual model`)
    assert.equal(generateMermaidSource(parsed), definition.sample, `${definition.type} must round-trip unchanged`)
  }

  assert.equal(seenTypes.size, MERMAID_DIAGRAM_CATALOG.length, 'diagram types must not be duplicated')

  const flowchart = parseMermaidVisualSource('flowchart TD\n  A[開始] --> B[終了]')
  assert.ok(flowchart)
  assert.equal(flowchartConnectionCount(flowchart, 'A'), 1)
  const node = createMermaidElement('flowchart', 'node', 2)
  node.fields.id = 'C'
  node.fields.label = '判断'
  node.fields.shape = 'diamond'
  flowchart.elements.push(node)
  assert.match(generateMermaidSource(flowchart), /C\{判断\}/)

  const doubleCircle = parseMermaidVisualSource('flowchart TD\n  A(((開始))) --> B[終了]')
  assert.ok(doubleCircle)
  assert.equal(doubleCircle.elements.find((item) => item.kind === 'edge')?.fields.fromShape, 'doublecircle')

  const sequence = parseMermaidVisualSource('sequenceDiagram\n  participant A as 利用者\n  participant B as システム\n  A->>B: 依頼')
  assert.ok(sequence)
  const message = sequence.elements.find((item) => item.kind === 'message')
  assert.ok(message)
  message.fields.message = '更新'
  message.dirty = true
  assert.match(generateMermaidSource(sequence), /更新/)

  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  A[開始] --> B[終了]'), true)
  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  click A handler'), false)
  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  A@{ shape: rect, label: "開始" }'), true)
  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  A@{ img: "https://example.test/a.png" }'), false)
  assert.equal(isMermaidEditorValueSafe('%%{init: {"theme":"dark"}}%%\nflowchart TD'), false)
  console.log('Mermaid visual editor model tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
