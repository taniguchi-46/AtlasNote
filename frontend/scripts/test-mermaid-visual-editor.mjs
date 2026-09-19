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
    sanitizeMermaidStyleValue,
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
  assert.equal(flowchart.elements.filter((item) => item.kind === 'node').length, 2,
    'flowchart edge endpoints must become two editable GUI nodes')
  assert.equal(flowchart.elements.filter((item) => item.kind === 'edge').length, 1,
    'flowchart connection must remain one edge rather than a card')
  assert.equal(generateMermaidSource(flowchart), 'flowchart TD\n  A[開始] --> B[終了]',
    'derived flowchart nodes must not add source lines')
  const node = createMermaidElement('flowchart', 'node', 2)
  node.fields.id = 'C'
  node.fields.label = '判断'
  node.fields.shape = 'diamond'
  flowchart.elements.push(node)
  assert.match(generateMermaidSource(flowchart), /C\{判断\}/)

  const classDiagram = parseMermaidVisualSource('classDiagram')
  assert.ok(classDiagram)
  const classRelation = createMermaidElement('class', 'relation', 0)
  classRelation.fields.from = 'User'
  classRelation.fields.to = 'Account'
  classRelation.dirty = true
  classDiagram.elements.push(classRelation)
  assert.match(generateMermaidSource(classDiagram), /User --> Account/)

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

  const sequenceDetails = parseMermaidVisualSource('sequenceDiagram\n  actor A as 利用者\n  participant B as システム\n  Note left of A: 注意\n  A->>B: 依頼\n  alt 条件\n    B-->>A: 応答\n  else 別条件\n    B-->>A: 別の応答\n  end')
  assert.ok(sequenceDetails)
  assert.equal(sequenceDetails.elements.filter((item) => item.kind === 'participant')[0]?.fields.participantType, 'actor')
  assert.equal(sequenceDetails.elements.find((item) => item.kind === 'note')?.fields.position, 'left')
  const branchStart = sequenceDetails.elements.find((item) => item.kind === 'control' && item.fields.command === 'alt')
  const branchEnd = sequenceDetails.elements.find((item) => item.kind === 'control' && item.fields.command === 'end')
  const branchMessage = sequenceDetails.elements.find((item) => item.kind === 'message' && item.fields.message === '応答')
  assert.ok(branchStart?.blockId)
  assert.equal(branchEnd?.blockId, branchStart?.blockId, 'sequence control blocks must retain their matching end')
  assert.equal(branchMessage?.blockId, branchStart?.blockId, 'sequence block members must retain their enclosing block')
  assert.equal(generateMermaidSource(sequenceDetails), 'sequenceDiagram\n  actor A as 利用者\n  participant B as システム\n  Note left of A: 注意\n  A->>B: 依頼\n  alt 条件\n    B-->>A: 応答\n  else 別条件\n    B-->>A: 別の応答\n  end')

  const styledFlowchart = parseMermaidVisualSource('flowchart TD\n  A[開始]\n  style A fill:#eef2ff,stroke:#7c8ff5,stroke-width:5px,stroke-dasharray:5 5,color:#172554')
  assert.ok(styledFlowchart)
  assert.equal(styledFlowchart.elements.find((item) => item.kind === 'style')?.fields.target, 'A')
  assert.equal(generateMermaidSource(styledFlowchart), 'flowchart TD\n  A[開始]\n  style A fill:#eef2ff,stroke:#7c8ff5,stroke-width:5px,stroke-dasharray:5 5,color:#172554')
  const parsedStyle = styledFlowchart.elements.find((item) => item.kind === 'style')
  parsedStyle.fields.fill = '#112233'
  parsedStyle.dirty = true
  assert.match(generateMermaidSource(styledFlowchart), /fill:#112233,stroke:#7c8ff5,stroke-width:5px,stroke-dasharray:5 5,color:#172554/,
    'changing a style color must retain non-color declarations')

  const subgraph = parseMermaidVisualSource('flowchart TD\n  subgraph Cluster\n    A[開始] --> B[終了]\n  end')
  assert.ok(subgraph)
  const subgraphStart = subgraph.elements.find((item) => item.kind === 'group' && item.fields.groupEnd !== 'true')
  const subgraphEnd = subgraph.elements.find((item) => item.kind === 'group' && item.fields.groupEnd === 'true')
  assert.ok(subgraphStart?.blockId)
  assert.equal(subgraphEnd?.blockId, subgraphStart?.blockId, 'subgraph boundaries must be paired')
  assert.equal(generateMermaidSource(subgraph), 'flowchart TD\n  subgraph Cluster\n    A[開始] --> B[終了]\n  end')
  assert.equal(sanitizeMermaidStyleValue('#abc'), '#abc')
  assert.equal(sanitizeMermaidStyleValue('url(https://example.test/a.png)', '#eef2ff'), '#eef2ff')

  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  A[開始] --> B[終了]'), true)
  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  click A handler'), false)
  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  A@{ shape: rect, label: "開始" }'), true)
  assert.equal(isMermaidEditorValueSafe('flowchart TD\n  A@{ img: "https://example.test/a.png" }'), false)
  assert.equal(isMermaidEditorValueSafe('%%{init: {"theme":"dark"}}%%\nflowchart TD'), false)
  console.log('Mermaid visual editor model tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
