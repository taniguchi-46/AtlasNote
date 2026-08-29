import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { JSDOM } from 'jsdom'
import ts from 'typescript'

const rootDir = process.cwd()
const sourceDir = path.join(rootDir, 'src', 'utils')
const outDir = path.join(rootDir, '.tmp', 'ai-markup-safety-test')
const utilityOutFile = path.join(outDir, 'aiMarkupSecurity.mjs')
const noteLinkOutFile = path.join(outDir, 'noteLink.mjs')

await mkdir(outDir, { recursive: true })

const dom = new JSDOM('<!doctype html><html><body></body></html>', {
  url: 'https://atlasnote.local/',
})
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  Node: dom.window.Node,
})

try {
  await compileTypeScript(path.join(sourceDir, 'noteLink.ts'), noteLinkOutFile)
  await compileTypeScript(path.join(sourceDir, 'aiMarkupSecurity.ts'), utilityOutFile, [
    ["from './noteLink'", "from './noteLink.mjs'"],
  ])

  const { AI_MARKDOWN_OPTIONS, sanitizeAIHtml } = await import(pathToFileUrl(utilityOutFile))
  assert.equal(AI_MARKDOWN_OPTIONS.html, true)
  assert.equal(AI_MARKDOWN_OPTIONS.linkify, true)

  const safe = sanitizeAIHtml(`
    <h2>見出し</h2>
    <p><strong>重要</strong>な内容</p>
    <ul><li>項目1</li><li>項目2</li></ul>
    <blockquote>引用</blockquote>
    <pre><code>&lt;button onclick=&quot;alert(1)&quot;&gt;unsafe&lt;/button&gt;</code></pre>
  `)
  assert.match(safe, /<h2>見出し<\/h2>/)
  assert.match(safe, /<strong>重要<\/strong>/)
  assert.match(safe, /<ul>/)
  assert.match(safe, /<blockquote>引用<\/blockquote>/)
  assert.match(safe, /<code>&lt;button onclick="alert\(1\)"&gt;unsafe&lt;\/button&gt;<\/code>/)

  const mixed = sanitizeAIHtml('<h2>見出し</h2><p>Markdownと<strong>HTML</strong>の混在</p>')
  assert.match(mixed, /<h2>見出し<\/h2>/)
  assert.match(mixed, /Markdownと<strong>HTML<\/strong>の混在/)

  const hostile = sanitizeAIHtml(`
    <script>alert(1)</script>
    <style>body { background: url(https://attacker.example/track) }</style>
    <iframe src="https://attacker.example"></iframe>
    <svg><a href="javascript:alert(1)">svg</a></svg>
    <img src="https://attacker.example/track.png" onerror="alert(1)">
    <p style="background:url(https://attacker.example/track)">text</p>
    <a href="javascript:alert(1)" onclick="alert(1)">unsafe link</a>
  `)
  assert.doesNotMatch(hostile, /<(?:script|style|iframe|svg|img)\b/i)
  assert.doesNotMatch(hostile, /\son[a-z]+\s*=/i)
  assert.doesNotMatch(hostile, /\sstyle\s*=/i)
  assert.doesNotMatch(hostile, /(?:href|src)\s*=\s*["']\s*javascript:/i)
  assert.doesNotMatch(hostile, /attacker\.example/i)
  assert.match(hostile, /unsafe link/)

  const links = sanitizeAIHtml(`
    <a href="https://example.com">safe external</a>
    <a href="atlasnote://note/${'a'.repeat(32)}">safe internal</a>
    <a href="http://example.com">insecure external</a>
    <a href="data:text/html,<script>alert(1)</script>">data</a>
    <a href="javascript:alert(1)">javascript</a>
  `)
  assert.match(links, /href="https:\/\/example\.com"/)
  assert.match(links, /rel="noreferrer noopener"/)
  assert.match(links, /href="atlasnote:\/\/note\/[a-f0-9]{32}"/)
  assert.doesNotMatch(links, /href="(?:http:|data:|javascript:)/i)

  const malformed = sanitizeAIHtml('<h2>見出し<p>本文<script>bad()</script>')
  assert.match(malformed, /見出し/)
  assert.match(malformed, /本文/)
  assert.doesNotMatch(malformed, /<script\b/i)
  console.log('AI markup safety tests passed')
} finally {
  dom.window.close()
  await rm(outDir, { recursive: true, force: true })
}

async function compileTypeScript(sourceFile, outputFile, replacements = []) {
  let source = await readFile(sourceFile, 'utf8')
  for (const [from, to] of replacements) source = source.replaceAll(from, to)
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  await writeFile(outputFile, compiled.outputText, 'utf8')
}

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}
