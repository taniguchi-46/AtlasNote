import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'markdownEditHistory.ts')
const continuationSourcePath = path.join(rootDir, 'src', 'utils', 'markdownListContinuation.ts')
const outDir = path.join(rootDir, '.tmp', 'markdown-edit-history-test')
const outFile = path.join(outDir, 'markdownEditHistory.mjs')
const continuationOutFile = path.join(outDir, 'markdownListContinuation.mjs')

await mkdir(outDir, { recursive: true })

try {
  const source = await readFile(sourcePath, 'utf8')
  const compilerOptions = {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  }
  const compiled = ts.transpileModule(source, {
    compilerOptions,
  })
  const continuationSource = await readFile(continuationSourcePath, 'utf8')
  const continuationCompiled = ts.transpileModule(continuationSource, {
    compilerOptions,
  })
  await writeFile(outFile, compiled.outputText, 'utf8')
  await writeFile(continuationOutFile, continuationCompiled.outputText, 'utf8')
  const { createMarkdownEditHistory } = await import(pathToFileURL(outFile).href)
  const { continueMarkdownList, createMarkdownLineBreakTracker } = await import(pathToFileURL(continuationOutFile).href)

  let time = 0
  const initial = { content: '', selectionStart: 0, selectionEnd: 0 }
  const history = createMarkdownEditHistory(initial, { now: () => time, groupDelayMs: 500 })

  const first = { content: 'a', selectionStart: 1, selectionEnd: 1 }
  history.record(initial, first, { group: 'insert-text' })
  time += 100
  const second = { content: 'ab', selectionStart: 2, selectionEnd: 2 }
  history.record(first, second, { group: 'insert-text' })
  assert.deepEqual(history.undo(), initial, 'nearby typing must undo as one group')
  assert.deepEqual(history.redo(), second, 'redo must restore grouped typing and selection')

  time += 1000
  const formatted = { content: '**ab**', selectionStart: 2, selectionEnd: 4 }
  history.record(second, formatted, { group: 'command', forceNewGroup: true })
  assert.deepEqual(history.undo(), second, 'toolbar changes must have their own undo step')

  const replacement = { content: 'replacement', selectionStart: 11, selectionEnd: 11 }
  history.record(second, replacement, { group: 'input', forceNewGroup: true })
  assert.equal(history.redo(), null, 'editing after undo must clear the redo branch')

  history.reset({ content: 'secret', selectionStart: 6, selectionEnd: 6 })
  assert.equal(history.undo(), null, 'reset must remove prior snapshots')
  assert.equal(history.redo(), null, 'reset must remove redo snapshots')

  const clamped = createMarkdownEditHistory({
    content: 'abc',
    selectionStart: -10,
    selectionEnd: 99,
  }).current()
  assert.deepEqual(clamped, { content: 'abc', selectionStart: 0, selectionEnd: 3 })

  const continueList = (content, cursor = content.length) => continueMarkdownList({
    content,
    selectionStart: cursor,
    selectionEnd: cursor,
  })
  assert.equal(continueList('- item')?.content, '- item\n- ')
  assert.equal(continueList('  * item')?.content, '  * item\n  * ')
  assert.equal(continueList('1. item')?.content, '1. item\n2. ')
  assert.equal(continueList('9. item')?.content, '9. item\n10. ')
  assert.equal(continueList('- [x] 完了')?.content, '- [x] 完了\n- [ ] ')
  assert.equal(continueList('- ')?.content, '\n')
  assert.equal(continueList('- [ ] ')?.content, '\n')
  assert.equal(continueList('- one two', 5)?.content, '- one\n-  two')
  assert.equal(continueList('---'), null)
  assert.equal(continueList('```\n- item', '```\n- item'.length), null)
  assert.equal(continueList('```\n- item\n```', '```\n- item\n```'.length - 3), null)
  assert.equal(continueMarkdownList({
    content: '- item',
    selectionStart: 1,
    selectionEnd: 2,
  }), null)
  assert.equal(continueList('- item\r\nnext', '- item'.length)?.content, '- item\r\n- \r\nnext')

  assert.match(
    await readFile(path.join(rootDir, 'src', 'components', 'NoteEditor.vue'), 'utf8'),
    /markdownLineBreakTracker\.handleKeydown\(event\)/,
    'the component must route keydown through the line-break tracker',
  )
  assert.match(
    await readFile(path.join(rootDir, 'src', 'components', 'NoteEditor.vue'), 'utf8'),
    /markdownLineBreakTracker\.shouldSkipListContinuation\(event\)/,
    'the component must route beforeinput through the line-break tracker',
  )
  assert.match(
    await readFile(path.join(rootDir, 'src', 'components', 'NoteEditor.vue'), 'utf8'),
    /function handleMarkdownInput\(event: Event\)\s*\{\s*markdownLineBreakTracker\.reset\(\)/,
    'the input event must clear a modifier decision if beforeinput was skipped',
  )

  const insertNativeLineBreak = (snapshot) => {
    const content = `${snapshot.content.slice(0, snapshot.selectionStart)}\n${snapshot.content.slice(snapshot.selectionEnd)}`
    const cursor = snapshot.selectionStart + 1
    return { content, selectionStart: cursor, selectionEnd: cursor }
  }
  const dispatchLineBreak = (tracker, before, keydown, beforeInput = {}) => {
    tracker.handleKeydown(keydown)
    const skipListContinuation = tracker.shouldSkipListContinuation({
      inputType: 'insertLineBreak',
      ...beforeInput,
    })
    if (!skipListContinuation && !beforeInput.isComposing) {
      const continued = continueMarkdownList(before)
      if (continued) return { action: 'continue-list', snapshot: continued }
    }
    return { action: 'native-line-break', snapshot: insertNativeLineBreak(before) }
  }
  const applyBeforeInputLineBreak = (tracker, before, beforeInput = {}) => {
    const skipListContinuation = tracker.shouldSkipListContinuation({
      inputType: 'insertLineBreak',
      ...beforeInput,
    })
    if (!skipListContinuation && !beforeInput.isComposing) {
      const continued = continueMarkdownList(before)
      if (continued) return { action: 'continue-list', snapshot: continued }
    }
    return { action: 'native-line-break', snapshot: insertNativeLineBreak(before) }
  }
  const listBefore = { content: '- item', selectionStart: 6, selectionEnd: 6 }
  const shiftTracker = createMarkdownLineBreakTracker()
  assert.deepEqual(
    dispatchLineBreak(shiftTracker, listBefore, {
      key: 'Enter',
      shiftKey: true,
      ctrlKey: false,
      altKey: false,
      metaKey: false,
      isComposing: false,
    }),
    {
      action: 'native-line-break',
      snapshot: { content: '- item\n', selectionStart: 7, selectionEnd: 7 },
    },
    'Shift+Enter must remain a normal newline in a list item',
  )

  const normalAfterShift = dispatchLineBreak(shiftTracker, listBefore, {
    key: 'Enter',
    shiftKey: false,
    ctrlKey: false,
    altKey: false,
    metaKey: false,
    isComposing: false,
  })
  assert.deepEqual(
    normalAfterShift,
    { action: 'continue-list', snapshot: { content: '- item\n- ', selectionStart: 9, selectionEnd: 9 } },
    'a following normal Enter must still continue the list',
  )

  const missingBeforeinputTracker = createMarkdownLineBreakTracker()
  missingBeforeinputTracker.handleKeydown({ key: 'Enter', shiftKey: true })
  assert.deepEqual(
    dispatchLineBreak(missingBeforeinputTracker, listBefore, {
      key: 'Enter',
      shiftKey: false,
      ctrlKey: false,
      altKey: false,
      metaKey: false,
      isComposing: false,
    }),
    normalAfterShift,
    'the next keydown must replace stale modifier state when beforeinput was skipped',
  )

  const inputFallbackTracker = createMarkdownLineBreakTracker()
  inputFallbackTracker.handleKeydown({ key: 'Enter', shiftKey: true })
  inputFallbackTracker.reset()
  assert.equal(
    applyBeforeInputLineBreak(inputFallbackTracker, listBefore).action,
    'continue-list',
    'a completed input event must clear stale modifier state before a later beforeinput-only Enter',
  )

  assert.equal(
    applyBeforeInputLineBreak(createMarkdownLineBreakTracker(), listBefore).action,
    'continue-list',
    'a beforeinput-only normal Enter path must continue the list',
  )
  assert.equal(
    applyBeforeInputLineBreak(createMarkdownLineBreakTracker(), listBefore, {
      getModifierState: (key) => key === 'Shift',
    }).action,
    'native-line-break',
    'beforeinput modifier state must suppress list continuation',
  )
  assert.equal(
    applyBeforeInputLineBreak(createMarkdownLineBreakTracker(), listBefore, { isComposing: true }).action,
    'native-line-break',
    'IME beforeinput must remain a normal input path',
  )

  const eventHistory = createMarkdownEditHistory(listBefore)
  const shifted = { content: '- item\n', selectionStart: 7, selectionEnd: 7 }
  eventHistory.record(listBefore, shifted, { group: 'insertLineBreak', forceNewGroup: true })
  assert.deepEqual(eventHistory.undo(), listBefore, 'one Undo must remove the Shift+Enter newline')
  assert.deepEqual(eventHistory.redo(), shifted, 'one Redo must restore the newline and selection')

  console.log('Markdown edit history tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
