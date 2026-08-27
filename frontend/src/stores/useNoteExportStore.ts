import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  exportNote,
  type NoteExportError,
  type NoteExportInput,
  type NoteExportResult,
} from '../api/noteExport'

const unavailableError: NoteExportError = {
  code: 'NOTE_EXPORT_UNAVAILABLE',
  message: 'エクスポートを開始できませんでした。',
  retryable: true,
}

export const useNoteExportStore = defineStore('note-export', () => {
  const isBusy = ref(false)
  const error = ref<NoteExportError | null>(null)
  const lastResult = ref<NoteExportResult | null>(null)

  async function runPrepared(
    prepare: () => Promise<NoteExportInput | null>,
  ): Promise<NoteExportResult | null> {
    if (isBusy.value) return null

    isBusy.value = true
    try {
      const input = await prepare()
      if (input === null) return null

      error.value = null
      lastResult.value = null
      const result = await exportNote(input)
      lastResult.value = result
      error.value = result.error ?? null
      return result
    } catch {
      const result: NoteExportResult = {
        cancelled: false,
        error: unavailableError,
      }
      lastResult.value = result
      error.value = result.error ?? null
      return result
    } finally {
      isBusy.value = false
    }
  }

  function run(input: NoteExportInput): Promise<NoteExportResult | null> {
    return runPrepared(async () => input)
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
    runPrepared,
    reset,
  }
})
