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
  assert.equal(settingsSearch.searchSettings('フォント').some((item) => item.id === 'editor.font-family'), true)
  assert.equal(settingsSearch.searchSettings('AI オフ').some((item) => item.id === 'ai.enabled'), true)
  assert.equal(settingsSearch.searchSettings('問い合わせ').some((item) => item.id === 'help.contact'), true)
  assert.equal(settingsSearch.searchSettings('Windows 削除').some((item) => item.id === 'general.uninstall'), true)
  assert.equal(settingsSearch.searchSettings('存在しない設定').length, 0)

  const [settingsSource, helpSource] = await Promise.all([
    readFile(path.join(rootDir, 'src', 'components', 'SettingsModal.vue'), 'utf8'),
    readFile(path.join(rootDir, 'src', 'components', 'HelpSettingsPanel.vue'), 'utf8'),
  ])
  assert.match(settingsSource, /v-model="settingsQuery"/)
  assert.match(settingsSource, /selectSearchResult\(item\)/)
  assert.match(settingsSource, /openInstalledApps/)
  assert.match(settingsSource, /<HelpSettingsPanel \/>/)
  assert.match(helpSource, /問い合わせ窓口は未設定・未公開です。/)
  assert.match(helpSource, /data-settings-anchor="help.contact"/)

  console.log('settings search tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
