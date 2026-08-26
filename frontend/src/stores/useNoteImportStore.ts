import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  importNotes,
  type NoteImportError,
  type NoteImportInput,
  type NoteImportResult,
} from '../api/noteImport'

const unavailableError: NoteImportError = {
  code: 'NOTE_IMPORT_UNAVAILABLE',
  message: 'インポートを開始できませんでした。',
  retryable: true,
}

export const useNoteImportStore = defineStore('note-import', () => {
  const isBusy = ref(false)
  const error = ref<NoteImportError | null>(null)
  const lastResult = ref<NoteImportResult | null>(null)

  async function run(input: NoteImportInput): Promise<NoteImportResult | null> {
    if (isBusy.value) return null

    isBusy.value = true
    error.value = null
    lastResult.value = null
    try {
      const result = await importNotes(input)
      lastResult.value = result
      error.value = result.error ?? null
      return result
    } catch {
      error.value = unavailableError
      return null
    } finally {
      isBusy.value = false
    }
  }

  function reset() {
    error.value = null
    lastResult.value = null
  }

  return {
    isBusy,
    error,
    lastResult,
    run,
    reset,
  }
})
