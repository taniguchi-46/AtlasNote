import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import MarkdownIt from 'markdown-it'
import ts from 'typescript'

const rootDir = process.cwd()
const optionsPath = path.join(rootDir, 'src', 'utils', 'markdownSecurity.ts')
const outDir = path.join(rootDir, '.tmp', 'markdown-safety-test')
const outFile = path.join(outDir, 'markdownSecurity.mjs')

await mkdir(outDir, { recursive: true })

try {
  const source = await readFile(optionsPath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })

  await writeFile(outFile, compiled.outputText, 'utf8')

  const { RICH_MARKDOWN_OPTIONS } = await import(pathToFileUrl(outFile))
  const markdown = new MarkdownIt(RICH_MARKDOWN_OPTIONS)

  const cases = [
    {
      name: 'multiline raw HTML',
      input: '<div\n onclick="alert(1)">\nunsafe\n</div>',
    },
    {
      name: 'event handler attributes',
      input: '<img src="x" onerror="alert(1)">\n<button onclick="alert(1)">unsafe</button>',
    },
    {
      name: 'dangerous URL in raw HTML',
      input: '<a href="javascript:alert(1)">unsafe</a>',
    },
    {
      name: 'script element',
      input: '<script>alert(1)</script>',
    },
  ]

  for (const testCase of cases) {
    const rendered = markdown.render(testCase.input)
    assert.equal(containsExecutableHtml(rendered), false, testCase.name)
  }

  const dangerousLink = markdown.render('[unsafe](javascript:alert(1))')
  assert.doesNotMatch(dangerousLink, /href\s*=\s*["']\s*javascript:/i)

  const dangerousImage = markdown.render('![unsafe](javascript:alert(1))')
  assert.doesNotMatch(dangerousImage, /src\s*=\s*["']\s*javascript:/i)

  const safeLink = markdown.render('[safe](https://example.com)')
  assert.match(safeLink, /href="https:\/\/example\.com"/)

  const codeFence = markdown.render('```html\n<button onclick="alert(1)">unsafe</button>\n```')
  assert.match(codeFence, /&lt;button onclick=&quot;alert\(1\)&quot;&gt;/)
  assert.equal(containsExecutableHtml(codeFence), false)
} finally {
  await rm(outDir, { recursive: true, force: true })
}

function containsExecutableHtml(html) {
  return /<(?:script|iframe|object|embed|style)\b/i.test(html)
    || /<[^>]+\son[a-z]+\s*=/i.test(html)
    || /(?:href|src)\s*=\s*["']\s*javascript:/i.test(html)
}

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}
