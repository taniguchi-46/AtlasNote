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
assert.match(noteListSource, /const isAllNotesList = computed/, 'the list must distinguish the all-notes view')
assert.match(noteListSource, /v-if="!isAllNotesList"/, 'every non-all-notes view must expose one return action')
assert.match(noteListSource, /aria-label="すべてのノート一覧に戻る"/, 'the return action must be an accessible icon button')
assert.match(noteListSource, /notebookStore\.activeNotebookId = null/, 'returning to all notes must clear notebook filtering')
assert.match(noteListSource, /async function deleteNoteFromList/, 'each note row must provide a direct delete action')
assert.match(noteListSource, /class="note-item-delete-button"/)
assert.match(noteListSource, /noteStore\.trashNotes\(\[item\.id\]\)/, 'normal notes must move to trash from the direct action')
assert.match(noteListSource, /noteStore\.permanentlyDeleteNotes\(\[item\.id\]\)/, 'trash items must use the existing permanent-delete path')

console.log('note list view tests passed')
