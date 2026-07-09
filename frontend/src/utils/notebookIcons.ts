import simpleCalendar from '../assets/notebook-icons/simpleCalendar.png'
import simpleLight from '../assets/notebook-icons/simpleLight.png'
import simpleNote from '../assets/notebook-icons/simpleNote.png'
import simplePen from '../assets/notebook-icons/simplePen.png'
import simpleTask from '../assets/notebook-icons/simpleTask.png'

export const DEFAULT_NOTEBOOK_ICON = 'default:note'
export const USER_ICON_STORAGE_KEY = 'atlas-user-notebook-icons'
export const USER_ICON_MAX_BYTES = 1024 * 1024
export const USER_ICON_ACCEPT = 'image/png,image/jpeg,image/webp'

export interface NotebookIconOption {
  id: string
  label: string
  src: string
  source: 'default' | 'user'
}

export const defaultNotebookIcons: NotebookIconOption[] = [
  { id: 'default:note', label: 'Note', src: simpleNote, source: 'default' },
  { id: 'default:pen', label: 'Pen', src: simplePen, source: 'default' },
  { id: 'default:task', label: 'Task', src: simpleTask, source: 'default' },
  { id: 'default:calendar', label: 'Calendar', src: simpleCalendar, source: 'default' },
  { id: 'default:light', label: 'Light', src: simpleLight, source: 'default' },
]

const allowedUserIconTypes = new Set(['image/png', 'image/jpeg', 'image/webp'])

function readUserIcons(): NotebookIconOption[] {
  try {
    const raw = localStorage.getItem(USER_ICON_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Array<Partial<NotebookIconOption>>
    if (!Array.isArray(parsed)) return []

    return parsed
      .filter((icon): icon is NotebookIconOption => (
        typeof icon.id === 'string' &&
        icon.id.startsWith('user:') &&
        typeof icon.label === 'string' &&
        typeof icon.src === 'string' &&
        icon.src.startsWith('data:image/') &&
        icon.source === 'user'
      ))
  } catch (_) {
    return []
  }
}

function writeUserIcons(icons: NotebookIconOption[]) {
  localStorage.setItem(USER_ICON_STORAGE_KEY, JSON.stringify(icons))
}

export function getUserNotebookIcons() {
  return readUserIcons()
}

export function getNotebookIconOptions() {
  return [...defaultNotebookIcons, ...readUserIcons()]
}

export function removeUserNotebookIcon(iconId: string) {
  if (!iconId.startsWith('user:')) {
    return false
  }

  const icons = readUserIcons()
  const nextIcons = icons.filter(icon => icon.id !== iconId)
  if (nextIcons.length === icons.length) {
    return false
  }

  writeUserIcons(nextIcons)
  return true
}

export function resolveNotebookIcon(iconId?: string | null) {
  return getNotebookIconOptions().find(icon => icon.id === iconId) ?? defaultNotebookIcons[0]
}

export function isKnownNotebookIcon(iconId: string) {
  return getNotebookIconOptions().some(icon => icon.id === iconId)
}

export async function addUserNotebookIcon(file: File): Promise<NotebookIconOption> {
  if (!allowedUserIconTypes.has(file.type)) {
    throw new Error('PNG、JPEG、WebP形式の画像を選択してください')
  }
  if (file.size > USER_ICON_MAX_BYTES) {
    throw new Error('アイコン画像は1MB以下にしてください')
  }

  const src = await readFileAsDataUrl(file)
  const id = `user:${createIconId()}`
  const icon: NotebookIconOption = {
    id,
    label: file.name.replace(/\.[^.]+$/, '') || 'User icon',
    src,
    source: 'user',
  }

  const icons = readUserIcons()
  icons.push(icon)
  writeUserIcons(icons)

  return icon
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(new Error('アイコン画像の読み込みに失敗しました'))
    reader.readAsDataURL(file)
  })
}

function createIconId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}
