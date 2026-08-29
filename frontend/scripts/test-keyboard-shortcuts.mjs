import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'keyboardShortcuts.ts')
const outDir = path.join(rootDir, '.tmp', 'keyboard-shortcuts-test')
const outFile = path.join(outDir, 'keyboardShortcuts.mjs')

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
  const shortcuts = await import(pathToFileURL(outFile).href)

  const defaults = shortcuts.createDefaultShortcutBindings()
  assert.equal(shortcuts.formatShortcutBinding(defaults['editor.undo']), 'Ctrl+Z')
  assert.equal(shortcuts.formatShortcutBinding(defaults['editor.redo']), 'Ctrl+Y')
  assert.equal(defaults['sync.run'], null)
  assert.equal(defaults['theme.toggle'], null)

  const ctrlZEvent = {
    code: 'KeyZ',
    ctrlKey: true,
    shiftKey: false,
    altKey: false,
    metaKey: false,
  }
  assert.equal(shortcuts.findMatchingShortcutAction(ctrlZEvent, defaults, 'editor'), 'editor.undo')
  assert.equal(shortcuts.findMatchingShortcutAction(ctrlZEvent, defaults, 'app'), null)
  assert.equal(shortcuts.isNativeHistoryShortcut(ctrlZEvent), true)
  assert.equal(shortcuts.isNativeHistoryShortcut({ ...ctrlZEvent, altKey: true }), false)

  const duplicate = shortcuts.validateShortcutBinding('editor.redo', defaults['editor.undo'], defaults)
  assert.equal(duplicate.ok, false)
  assert.equal(duplicate.code, 'DUPLICATE_BINDING')

  const reserved = shortcuts.validateShortcutBinding(
    'note.new',
    { code: 'KeyC', ctrl: true, shift: false, alt: false, meta: false },
    defaults,
  )
  assert.equal(reserved.ok, false)
  assert.equal(reserved.code, 'RESERVED_BINDING')

  const missingModifier = shortcuts.bindingFromKeyboardEvent({
    code: 'KeyQ',
    ctrlKey: false,
    shiftKey: false,
    altKey: false,
    metaKey: false,
  })
  assert.equal(missingModifier, null)

  const customized = {
    ...defaults,
    'editor.undo': { code: 'KeyU', ctrl: true, shift: false, alt: true, meta: false },
    'editor.redo': null,
    'sync.run': { code: 'F6', ctrl: false, shift: false, alt: false, meta: false },
  }
  const restored = shortcuts.parseStoredShortcutBindings(
    shortcuts.serializeShortcutBindings(customized),
  )
  assert.deepEqual(restored, customized)
  assert.equal(
    shortcuts.findMatchingShortcutAction(ctrlZEvent, restored, 'editor'),
    null,
    'the old Ctrl+Z binding must stop working after undo is reassigned',
  )
  assert.equal(
    shortcuts.findMatchingShortcutAction({
      code: 'KeyU',
      ctrlKey: true,
      shiftKey: false,
      altKey: true,
      metaKey: false,
    }, restored, 'editor'),
    'editor.undo',
  )
  assert.deepEqual(
    shortcuts.parseStoredShortcutBindings('{"version":999,"bindings":{}}'),
    defaults,
  )
  assert.deepEqual(shortcuts.parseStoredShortcutBindings('broken json'), defaults)

  const [noteEditorSource, appSource, settingsSource, aiWorkspaceSource] = await Promise.all([
    readFile(path.join(rootDir, 'src', 'components', 'NoteEditor.vue'), 'utf8'),
    readFile(path.join(rootDir, 'src', 'App.vue'), 'utf8'),
    readFile(path.join(rootDir, 'src', 'components', 'ShortcutSettingsPanel.vue'), 'utf8'),
    readFile(path.join(rootDir, 'src', 'components', 'AIWorkspace.vue'), 'utf8'),
  ])
  assert.match(noteEditorSource, /undoRedo:\s*false/, 'Tiptap fixed undo keymap must be disabled')
  assert.match(noteEditorSource, /findMatchingShortcutAction[\s\S]*'editor'/, 'editor keys must use settings')
  assert.match(noteEditorSource, /@beforeinput="handleMarkdownBeforeInput"/, 'Markdown input must record history')
  assert.match(appSource, /addEventListener\('keydown', handleGlobalShortcut, true\)/, 'app shortcuts must use capture dispatch')
  assert.match(settingsSource, /Esc、Delete、Backspace/, 'shortcut capture must document every key that clears a binding')
  assert.match(
    settingsSource,
    /event\.code === 'Escape'[\s\S]*settingsStore\.setShortcutBinding\(actionId, null\)/,
    'Escape must clear the current shortcut binding',
  )
  assert.match(aiWorkspaceSource, /通常のノート編集の「元に戻す」対象にはなりません/, 'Agent apply must not promise local undo')

  console.log('keyboard shortcut tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
