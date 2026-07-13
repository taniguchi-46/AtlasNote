import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'stores', 'useNotificationStore.ts')
const outDir = path.join(rootDir, '.tmp', 'notification-store-test')
const outFile = path.join(outDir, 'useNotificationStore.mjs')

await mkdir(outDir, { recursive: true })

const source = await readFile(sourcePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')

try {
  setActivePinia(createPinia())
  const { useNotificationStore } = await import(pathToFileURL(outFile).href)
  const store = useNotificationStore()
  let retryCount = 0

  const notificationId = store.notify('temporary failure', {
    kind: 'error',
    source: 'test',
    code: 'TEST_OPERATION_FAILED',
    retryable: true,
    action: {
      label: 'retry',
      run: () => {
        retryCount += 1
      },
    },
  })

  assert.equal(store.notifications.length, 1)
  assert.equal(store.notifications[0].code, 'TEST_OPERATION_FAILED')
  await store.runAction(notificationId)
  assert.equal(retryCount, 1)
  assert.equal(store.notifications.length, 0)

  console.log('notification store tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
