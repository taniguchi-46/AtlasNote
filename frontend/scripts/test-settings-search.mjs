import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'settingsSearch.ts')
const outDir = path.join(rootDir, '.tmp', 'settings-search-test')
const outFile = path.join(outDir, 'settingsSearch.mjs')

await mkdir(outDir, { recursive: true })

try {
  const source = await readFile(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  await writeFile(outFile, compiled.outputText, 'utf8')
  const settingsSearch = await import(pathToFileURL(outFile).href)

  assert.deepEqual(settingsSearch.searchSettings(''), [])
  const fontResults = settingsSearch.searchSettings('フォント')
  assert.equal(fontResults.some((item) => item.id === 'editor.font-family'), true)
  assert.equal(fontResults.every((item) => typeof item.matchedText === 'string' && item.matchedText.length > 0), true)
  const aiResults = settingsSearch.searchSettings('旧AIデータ')
  assert.equal(aiResults.some((item) => item.id === 'ai'), true)
  assert.equal(settingsSearch.searchSettings('プロバイダー').some((item) => item.tab === 'ai'), false)
  assert.equal(settingsSearch.searchSettings('API Key').some((item) => item.tab === 'ai'), false)
  assert.equal(settingsSearch.searchSettings('問い合わせ').some((item) => item.id === 'help.contact'), false)
  assert.equal(settingsSearch.searchSettings('診断情報').some((item) => item.id === 'storage-locations.diagnostics'), false)
  assert.equal(settingsSearch.searchSettings('診断情報').length, 0)
  assert.equal(settingsSearch.searchSettings('Windows 削除').some((item) => item.id === 'general.uninstall'), true)
  assert.equal(settingsSearch.searchSettings('存在しない設定').length, 0)
  assert.ok(settingsSearch.searchSettings('ターミナル 文字サイズ').some(item => item.id === 'terminal.font-size'))
  assert.ok(settingsSearch.searchSettings('High Contrast').some(item => item.id === 'terminal.theme'))

  const [settingsSource, helpSource] = await Promise.all([
    readFile(path.join(rootDir, 'src', 'components', 'SettingsModal.vue'), 'utf8'),
    readFile(path.join(rootDir, 'src', 'components', 'HelpSettingsPanel.vue'), 'utf8'),
  ])
  assert.match(settingsSource, /v-model="settingsQuery"/)
  assert.match(settingsSource, /selectSearchResult\(item\)/)
  assert.match(settingsSource, /openInstalledApps/)
  assert.match(settingsSource, /<HelpSettingsPanel \/>/)
  assert.match(settingsSource, /<LegacyAIRecordsPanel \/>/)
  assert.doesNotMatch(settingsSource, /<AISettingsPanel \/>/)
  assert.match(settingsSource, /<small v-if="item\.matchedText">一致: \{\{ item\.matchedText \}\}<\/small>/)
  assert.match(settingsSource, /closest\('details'\)/)
  assert.match(helpSource, /ヘルプ本文は一時的に非表示/)
  assert.match(helpSource, /<!-- ヘルプ本文は一時的に非表示/)
  const locationSource = await readFile(path.join(rootDir, 'src/components/StorageLocationSettingsPanel.vue'), 'utf8')
  const spacesSource = await readFile(path.join(rootDir, 'src/components/StorageSpaceSettingsPanel.vue'), 'utf8')
  const shortcutsSource = await readFile(path.join(rootDir, 'src/components/ShortcutSettingsPanel.vue'), 'utf8')
  const themeSource = await readFile(path.join(rootDir, 'src/style.css'), 'utf8')
  const declaredTokens = new Set([...themeSource.matchAll(/(--[\w-]+)\s*:/g)].map(match => match[1]))
  const terminalSettingsSource = await readFile(path.join(rootDir, 'src/components/TerminalSettingsPanel.vue'), 'utf8')
  const supportSource = await readFile(path.join(rootDir, 'src/components/SupportWorkspace.vue'), 'utf8')
  const terminalSource = await readFile(path.join(rootDir, 'src/components/TerminalComponent.vue'), 'utf8')
  const zIndex = (source, selector) => {
    const css = source.slice(source.indexOf('<style'))
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const rule = css.match(new RegExp(`${escaped}\\s*\\{([^}]+)\\}`))
    assert.ok(rule, `missing layer rule: ${selector}`)
    const declaration = rule[1].match(/z-index\s*:\s*(\d+)\s*;/)
    assert.ok(declaration, `missing numeric z-index: ${selector}`)
    return Number(declaration[1])
  }
  const floatingLayer = zIndex(supportSource, '.support-workspace.is-floating')
  const menuLayer = zIndex(terminalSource, '.terminal-menu')
  const settingsOverlay = zIndex(settingsSource, '.settings-modal-overlay')
  const settingsContent = zIndex(settingsSource, '.settings-modal-content')
  const nestedOverlay = zIndex(spacesSource, '.nested-dialog-overlay')
  const nestedContent = zIndex(spacesSource, '.nested-dialog-content')
  assert.equal(floatingLayer, 1300, 'floating layer is unchanged')
  assert.ok(settingsOverlay > floatingLayer, 'settings overlay covers floating workspace')
  assert.ok(settingsOverlay > menuLayer, 'settings overlay covers Terminal More')
  assert.ok(settingsContent > settingsOverlay, 'settings dialog is above its overlay')
  assert.ok(nestedOverlay > settingsContent, 'nested overlay covers parent settings dialog')
  assert.ok(nestedContent > nestedOverlay, 'nested dialog is above its overlay')
  // All settings dialogs use the default body Portal; creation, switching and lock
  // dialogs must share the nested layer rather than inherit the parent content layer.
  const parentPortals = [...settingsSource.matchAll(/<DialogPortal>([\s\S]*?)<\/DialogPortal>/g)]
  const nestedPortals = [...spacesSource.matchAll(/<DialogPortal>([\s\S]*?)<\/DialogPortal>/g)]
  assert.equal(parentPortals.length, 1)
  assert.equal(nestedPortals.length, 3)
  assert.match(parentPortals[0][1], /<DialogOverlay class="settings-modal-overlay"/)
  assert.match(parentPortals[0][1], /<DialogContent class="settings-modal-content"/)
  for (const [, portal] of nestedPortals) {
    assert.match(portal, /<DialogOverlay class="nested-dialog-overlay"/)
    assert.match(portal, /<DialogContent class="nested-dialog-content(?: lock-dialog-content)?"/)
  }
  for (const source of [locationSource, spacesSource, shortcutsSource, terminalSettingsSource, settingsSource]) {
    const css = source.split('<style scoped>')[1]
    assert.doesNotMatch(css, /var\(--color-(border|surface|text-muted|accent)|var\([^)]*,\s*#/i)
    for (const [, token] of css.matchAll(/var\((--[\w-]+)/g)) assert.ok(declaredTokens.has(token), `undeclared theme token: ${token}`)
    assert.doesNotMatch(css, /opacity: 0\.[0-6]/)
  }
  assert.match(settingsSource, /<TerminalSettingsPanel \/>/)
  assert.match(settingsSource, /:deep\(select\)[\s\S]*background: var\(--bg-sidebar\)/)
  assert.match(settingsSource, /:deep\(option\)[\s\S]*color: var\(--text-primary\)/)
  assert.doesNotMatch(shortcutsSource, /枠|shortcut-slot-label/)
  assert.match(shortcutsSource, /ショートカット\$\{slot \+ 1\}/)

  console.log('settings search tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
