import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'

const rootDir = process.cwd()
const setupPath = path.join(rootDir, 'src', 'components', 'StorageLocationSetupScreen.vue')
const appPath = path.join(rootDir, 'src', 'App.vue')
const startupApiPath = path.join(rootDir, 'src', 'api', 'startup.ts')
const storageApiPath = path.join(rootDir, 'src', 'api', 'storageLocations.ts')
const stylePath = path.join(rootDir, 'src', 'style.css')

const [setupSource, appSource, startupApiSource, storageApiSource, styleSource] = await Promise.all([
  readFile(setupPath, 'utf8'),
  readFile(appPath, 'utf8'),
  readFile(startupApiPath, 'utf8'),
  readFile(storageApiPath, 'utf8'),
  readFile(stylePath, 'utf8'),
])

assert.match(setupSource, /mode\?: 'setup' \| 'recovery'/)
assert.match(setupSource, /const isRecovery = computed/)
assert.match(setupSource, /自動設定済み/)
assert.match(setupSource, /元の保存場所に戻す/)
assert.match(setupSource, /同じ移行を再試行する/)
assert.match(setupSource, /別の空フォルダで開始する場合、元データは自動移行されません/)
assert.match(setupSource, /インストールされているアプリ/)
assert.match(setupSource, />終了<\/button>/)
assert.match(setupSource, /overflow-y: auto/)
assert.match(setupSource, /var\(--bg-app\)/)
assert.match(setupSource, /var\(--text-primary\)/)
assert.match(appSource, /phase === 'storage-recovery'/)
assert.match(appSource, /startupStatus && !startupStatus\.ready/)
assert.match(appSource, /phase !== 'storage-recovery'/)
assert.match(startupApiSource, /'storage-recovery'/)
assert.match(startupApiSource, /OpenInstalledApps/)
assert.match(storageApiSource, /cancelPendingStorageLocationMigration/)
assert.match(storageApiSource, /retryPendingStorageLocationMigration/)
assert.match(styleSource, /--text-muted: #8b949e/)
assert.match(styleSource, /--text-muted: #475569/)

console.log('storage location setup tests passed')
