import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'

const rootDir = process.cwd()
const setupPath = path.join(rootDir, 'src', 'components', 'StorageLocationSetupScreen.vue')
const appPath = path.join(rootDir, 'src', 'App.vue')
const startupApiPath = path.join(rootDir, 'src', 'api', 'startup.ts')
const stylePath = path.join(rootDir, 'src', 'style.css')

const [setupSource, appSource, startupApiSource, styleSource] = await Promise.all([
  readFile(setupPath, 'utf8'),
  readFile(appPath, 'utf8'),
  readFile(startupApiPath, 'utf8'),
  readFile(stylePath, 'utf8'),
])

assert.match(setupSource, /mode\?: 'setup' \| 'recovery'/)
assert.match(setupSource, /const isRecovery = computed/)
assert.match(setupSource, /自動設定済み/)
assert.match(setupSource, /overflow-y: auto/)
assert.match(setupSource, /var\(--bg-app\)/)
assert.match(setupSource, /var\(--text-primary\)/)
assert.match(appSource, /phase === 'storage-recovery'/)
assert.match(appSource, /startupStatus && !startupStatus\.ready/)
assert.match(appSource, /phase !== 'storage-recovery'/)
assert.match(startupApiSource, /'storage-recovery'/)
assert.match(styleSource, /--text-muted: #8b949e/)
assert.match(styleSource, /--text-muted: #475569/)

console.log('storage location setup tests passed')
