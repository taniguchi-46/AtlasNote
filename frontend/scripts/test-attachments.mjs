import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const outDir = path.join(rootDir, '.tmp', 'attachments-test')
await mkdir(outDir, { recursive: true })

async function compile(sourceName) {
  const sourcePath = path.join(rootDir, 'src', 'utils', sourceName)
  const outFile = path.join(outDir, sourceName.replace(/\.ts$/, '.mjs'))
  const source = await readFile(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  await writeFile(outFile, compiled.outputText, 'utf8')
  return import(pathToFileURL(outFile).href)
}

try {
  const reference = await compile('attachmentReference.ts')
  assert.equal(reference.createAttachmentReference('note-1', 'file_1'), 'atlasnote-attachment://note-1/file_1')
  assert.deepEqual(reference.parseAttachmentReference('atlasnote-attachment://note-1/file_1'), {
    noteId: 'note-1',
    attachmentId: 'file_1',
  })
  assert.equal(reference.parseAttachmentReference('file:///C:/secret.png'), null)
  assert.equal(reference.parseAttachmentReference('atlasnote-attachment://note/../file'), null)

  globalThis.btoa ??= (value) => Buffer.from(value, 'binary').toString('base64')
  const clipboard = await compile('attachmentClipboard.ts')
  const png = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
    'base64',
  )
  const file = {
    name: 'paste.png',
    type: 'image/png',
    size: png.length,
    arrayBuffer: async () => png.buffer.slice(png.byteOffset, png.byteOffset + png.byteLength),
  }
  const payload = await clipboard.readClipboardImage(file)
  assert.equal(payload.mimeType, 'image/png')
  assert.equal(payload.width, 1)
  assert.equal(payload.height, 1)
  assert.equal(clipboard.encodeBase64(payload.data), png.toString('base64'))
  await assert.rejects(
    clipboard.readClipboardImage({ ...file, type: 'image/gif' }),
    /unsupported-image-type/,
  )
  await assert.rejects(
    clipboard.readClipboardImage({ ...file, size: clipboard.MAX_ATTACHMENT_BYTES + 1 }),
    /image-too-large/,
  )

  const editorSource = await readFile(path.join(rootDir, 'src', 'components', 'NoteEditor.vue'), 'utf8')
  assert.match(editorSource, /@paste="handleMarkdownPaste"/)
  assert.match(editorSource, /getClipboardImage\(event\)/)
  assert.match(editorSource, /setImage\(\{\s*src: attachment\.reference/)
  assert.match(editorSource, /replaceMarkdownRange\(/)
  assert.match(editorSource, /saveNoteAttachments\(/)
  assert.match(editorSource, /imagePasteGeneration/)
  assert.match(editorSource, /guardEditorInput: true/)
  assert.match(editorSource, /allowPlaintextProtected/)
  assert.match(editorSource, /flushPendingDraft\(\)/)
  assert.doesNotMatch(editorSource, /data:image\/png;base64/)
  assert.doesNotMatch(editorSource, /data:image\/jpeg;base64/)

  console.log('attachment tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
