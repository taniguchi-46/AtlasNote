import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'contentLockAutoLock.ts')
const outDir = path.join(rootDir, '.tmp', 'content-lock-auto-lock-test')
const outFile = path.join(outDir, 'contentLockAutoLock.mjs')

await mkdir(outDir, { recursive: true })
try {
  const source = await readFile(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
  })
  await writeFile(outFile, compiled.outputText, 'utf8')
  const { createContentLockAutoLock } = await import(pathToFileURL(outFile).href)

  let currentTime = 0
  let nextTimerID = 0
  const timers = new Map()
  const dueCalls = []
  let allowLock = true
  const scheduler = createContentLockAutoLock(
    async (targets) => {
      dueCalls.push(structuredClone(targets))
      return allowLock
    },
    {
      now: () => currentTime,
      setTimer: (callback, delayMs) => {
        const id = ++nextTimerID
        timers.set(id, { callback, delayMs })
        return id
      },
      clearTimer: (id) => timers.delete(id),
      retryDelayMs: 10_000,
    },
  )

  const noteLock = {
    id: 'note-lock',
    targetType: 'note',
    targetId: 'note-1',
    targetName: 'テストノート',
    unlocked: true,
    createdAt: '',
    updatedAt: '',
  }

  scheduler.update({
    minutes: 5,
    locks: [noteLock],
    unlockedAt: { 'note:note-1': 0 },
  })
  assert.equal(Array.from(timers.values()).at(-1).delayMs, 300_000, 'deadline is measured from unlock time')

  currentTime = 299_999
  await scheduler.check()
  assert.equal(dueCalls.length, 0, 'ordinary interaction does not extend or prematurely trigger the deadline')

  currentTime = 300_000
  await scheduler.check()
  assert.deepEqual(dueCalls, [[{ type: 'note', id: 'note-1' }]])

  // A successful lock is removed from the next snapshot. A later successful
  // unlock starts a new fixed interval even when the target is the same.
  scheduler.update({ minutes: 5, locks: [{ ...noteLock, unlocked: false }], unlockedAt: {} })
  currentTime = 420_000
  scheduler.update({
    minutes: 1,
    locks: [noteLock],
    unlockedAt: { 'note:note-1': 420_000 },
  })
  currentTime = 480_000
  await scheduler.check()
  assert.equal(dueCalls.length, 2, 'a new unlock timestamp re-arms the same lock')

  // Reducing the selected duration re-evaluates the original timestamp rather
  // than starting a new timer from the settings interaction.
  scheduler.update({
    minutes: 15,
    locks: [{ ...noteLock, unlocked: false }],
    unlockedAt: {},
  })
  currentTime = 1_000_000
  scheduler.update({
    minutes: 15,
    locks: [noteLock],
    unlockedAt: { 'note:note-1': 600_000 },
  })
  scheduler.update({
    minutes: 5,
    locks: [noteLock],
    unlockedAt: { 'note:note-1': 600_000 },
  })
  await scheduler.check()
  assert.equal(dueCalls.length, 3, 'a shorter setting locks immediately when the original deadline passed')

  scheduler.update({ minutes: 1, locks: [{ ...noteLock, unlocked: false }], unlockedAt: {} })
  currentTime = 2_000_000
  allowLock = false
  scheduler.update({
    minutes: 1,
    locks: [noteLock],
    unlockedAt: { 'note:note-1': 1_900_000 },
  })
  await scheduler.check()
  assert.equal(dueCalls.length, 4)
  assert.equal(Array.from(timers.values()).at(-1).delayMs, 10_000, 'failed save retries without resetting the unlock timestamp')

  scheduler.dispose()
  assert.equal(timers.size, 0)
  console.log('content lock auto-lock tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
