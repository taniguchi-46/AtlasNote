import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'

const rootDir = process.cwd()
const appStoreSource = await readFile(path.join(rootDir, 'src', 'stores', 'useAppStore.ts'), 'utf8')
const noteStoreSource = await readFile(path.join(rootDir, 'src', 'stores', 'useNoteStore.ts'), 'utf8')
const searchStoreSource = await readFile(path.join(rootDir, 'src', 'stores', 'useSearchStore.ts'), 'utf8')
const noteListSource = await readFile(path.join(rootDir, 'src', 'components', 'NoteList.vue'), 'utf8')

assert.match(appStoreSource, /NOTE_SORT_OPTIONS/)
assert.match(appStoreSource, /updatedAt:desc/)
assert.match(appStoreSource, /createdAt:asc/)
assert.match(appStoreSource, /title:desc/)
assert.match(noteStoreSource, /todayOnly/)
assert.match(noteStoreSource, /sortSummaries/)
assert.match(searchStoreSource, /parseNoteSortOption\(appStore\.sortOption\)/)
assert.match(noteListSource, /ArrowDownUpIcon/)
assert.match(noteListSource, /DropdownMenuTrigger/)
assert.match(noteListSource, /DropdownMenuRadioGroup/)
assert.match(noteListSource, /aria-label="並び替え"/)
assert.doesNotMatch(noteListSource, /<select/)
assert.match(noteListSource, /case 'recent': list = noteStore\.activeNotes/)

console.log('note list view tests passed')
