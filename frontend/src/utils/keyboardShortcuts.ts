export type ShortcutActionId =
  | 'editor.undo'
  | 'editor.redo'
  | 'note.new'
  | 'search.focus'
  | 'settings.open'
  | 'sync.run'
  | 'note.import'
  | 'window.toggleAlwaysOnTop'
  | 'theme.toggle'
  | 'editor.toggleMode'
  | 'ai.toggleWorkspace'

export type ShortcutScope = 'app' | 'editor'

export type ShortcutBinding = {
  code: string
  ctrl: boolean
  shift: boolean
  alt: boolean
  meta: boolean
}

export type ShortcutBindingSlot = 0 | 1
export type ShortcutBindingSlots = [ShortcutBinding | null, ShortcutBinding | null]
export type ShortcutBindings = Record<ShortcutActionId, ShortcutBindingSlots>

export type ShortcutActionDefinition = {
  id: ShortcutActionId
  label: string
  scope: ShortcutScope
  defaultBinding: ShortcutBinding | null
}

export type ShortcutEventLike = {
  code: string
  ctrlKey: boolean
  shiftKey: boolean
  altKey: boolean
  metaKey: boolean
  isComposing?: boolean
  getModifierState?: (key: string) => boolean
}

export type ShortcutValidationResult =
  | { ok: true; binding: ShortcutBinding | null }
  | {
      ok: false
      code: 'UNKNOWN_ACTION' | 'INVALID_BINDING' | 'RESERVED_BINDING' | 'DUPLICATE_BINDING'
      message: string
      conflictingActionId?: ShortcutActionId
      conflictingSlot?: ShortcutBindingSlot
    }

export const SHORTCUT_STORAGE_KEY = 'atlas-keybindings-v2'
export const SHORTCUT_LEGACY_STORAGE_KEY = 'atlas-keybindings-v1'
export const SHORTCUT_STORAGE_VERSION = 2
export const SHORTCUT_LEGACY_STORAGE_VERSION = 1

const createBinding = (
  code: string,
  modifiers: Partial<Omit<ShortcutBinding, 'code'>> = {},
): ShortcutBinding => ({
  code,
  ctrl: modifiers.ctrl ?? false,
  shift: modifiers.shift ?? false,
  alt: modifiers.alt ?? false,
  meta: modifiers.meta ?? false,
})

export const SHORTCUT_ACTIONS: readonly ShortcutActionDefinition[] = [
  { id: 'editor.undo', label: '元に戻す', scope: 'editor', defaultBinding: createBinding('KeyZ', { ctrl: true }) },
  { id: 'editor.redo', label: 'やり直す', scope: 'editor', defaultBinding: createBinding('KeyY', { ctrl: true }) },
  { id: 'note.new', label: '新しいノート', scope: 'app', defaultBinding: createBinding('KeyN', { ctrl: true }) },
  { id: 'search.focus', label: '検索欄へ移動', scope: 'app', defaultBinding: createBinding('KeyF', { ctrl: true }) },
  { id: 'settings.open', label: '設定を開く', scope: 'app', defaultBinding: createBinding('Comma', { ctrl: true }) },
  { id: 'sync.run', label: '同期', scope: 'app', defaultBinding: null },
  { id: 'note.import', label: 'ノートをインポート', scope: 'app', defaultBinding: null },
  { id: 'window.toggleAlwaysOnTop', label: '常に最前面を切り替える', scope: 'app', defaultBinding: null },
  { id: 'theme.toggle', label: 'テーマを切り替える', scope: 'app', defaultBinding: null },
  { id: 'editor.toggleMode', label: 'Markdown／リッチテキストを切り替える', scope: 'app', defaultBinding: null },
  { id: 'ai.toggleWorkspace', label: 'AIワークスペースを切り替える', scope: 'app', defaultBinding: null },
]

const shortcutActionById = new Map(SHORTCUT_ACTIONS.map((action) => [action.id, action]))

const codeLabels: Record<string, string> = {
  Backquote: '`',
  Minus: '-',
  Equal: '=',
  BracketLeft: '[',
  BracketRight: ']',
  Backslash: '\\',
  Semicolon: ';',
  Quote: "'",
  Comma: ',',
  Period: '.',
  Slash: '/',
  Space: 'Space',
  Enter: 'Enter',
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
  Home: 'Home',
  End: 'End',
  PageUp: 'PageUp',
  PageDown: 'PageDown',
}

const supportedNamedCodes = new Set(Object.keys(codeLabels))
const reservedEditingCodes = new Set(['KeyA', 'KeyC', 'KeyX', 'KeyV', 'KeyB', 'KeyI', 'KeyU'])

function cloneBinding(binding: ShortcutBinding | null): ShortcutBinding | null {
  return binding ? { ...binding } : null
}

function createBindingSlots(
  first: ShortcutBinding | null,
  second: ShortcutBinding | null = null,
): ShortcutBindingSlots {
  return [cloneBinding(first), cloneBinding(second)]
}

function createEmptyShortcutBindings(): ShortcutBindings {
  return Object.fromEntries(
    SHORTCUT_ACTIONS.map((action) => [action.id, createBindingSlots(null)]),
  ) as ShortcutBindings
}

function isShortcutActionId(value: string): value is ShortcutActionId {
  return shortcutActionById.has(value as ShortcutActionId)
}

function isSupportedShortcutCode(code: string) {
  return /^Key[A-Z]$/.test(code)
    || /^Digit[0-9]$/.test(code)
    || /^F(?:[1-9]|1[0-2])$/.test(code)
    || supportedNamedCodes.has(code)
}

function hasRequiredModifier(binding: ShortcutBinding) {
  return binding.ctrl || binding.alt || binding.meta || /^F(?:[1-9]|1[0-2])$/.test(binding.code)
}

function isReservedEditingBinding(binding: ShortcutBinding) {
  const usesPrimaryModifier = binding.ctrl || binding.meta
  if (!usesPrimaryModifier || binding.alt) return false
  if (reservedEditingCodes.has(binding.code)) return true
  return binding.code === 'Enter'
}

function normalizeBinding(value: unknown): ShortcutBinding | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const candidate = value as Partial<ShortcutBinding>
  if (
    typeof candidate.code !== 'string'
    || typeof candidate.ctrl !== 'boolean'
    || typeof candidate.shift !== 'boolean'
    || typeof candidate.alt !== 'boolean'
    || typeof candidate.meta !== 'boolean'
  ) return null

  const binding = createBinding(candidate.code, candidate)
  if (!isSupportedShortcutCode(binding.code) || !hasRequiredModifier(binding)) return null
  return binding
}

export function createDefaultShortcutBindings(): ShortcutBindings {
  return Object.fromEntries(
    SHORTCUT_ACTIONS.map((action) => [action.id, createBindingSlots(action.defaultBinding)]),
  ) as ShortcutBindings
}

export function getShortcutActionDefinition(actionId: ShortcutActionId) {
  return shortcutActionById.get(actionId) ?? null
}

export function shortcutBindingsEqual(
  left: ShortcutBinding | null,
  right: ShortcutBinding | null,
) {
  if (!left || !right) return left === right
  return left.code === right.code
    && left.ctrl === right.ctrl
    && left.shift === right.shift
    && left.alt === right.alt
    && left.meta === right.meta
}

export function bindingFromKeyboardEvent(event: ShortcutEventLike): ShortcutBinding | null {
  if (event.isComposing || event.getModifierState?.('AltGraph')) return null
  const binding = createBinding(event.code, {
    ctrl: event.ctrlKey,
    shift: event.shiftKey,
    alt: event.altKey,
    meta: event.metaKey,
  })
  if (!isSupportedShortcutCode(binding.code) || !hasRequiredModifier(binding)) return null
  return binding
}

export function formatShortcutBinding(binding: ShortcutBinding | null) {
  if (!binding) return '未割り当て'
  const modifiers = [
    binding.ctrl ? 'Ctrl' : '',
    binding.alt ? 'Alt' : '',
    binding.shift ? 'Shift' : '',
    binding.meta ? 'Meta' : '',
  ].filter(Boolean)
  const key = binding.code.startsWith('Key')
    ? binding.code.slice(3)
    : binding.code.startsWith('Digit')
      ? binding.code.slice(5)
      : codeLabels[binding.code] ?? binding.code
  return [...modifiers, key].join('+')
}

export function matchesShortcut(event: ShortcutEventLike, binding: ShortcutBinding | null) {
  if (!binding || event.isComposing || event.getModifierState?.('AltGraph')) return false
  return event.code === binding.code
    && event.ctrlKey === binding.ctrl
    && event.shiftKey === binding.shift
    && event.altKey === binding.alt
    && event.metaKey === binding.meta
}

export function isNativeHistoryShortcut(event: ShortcutEventLike) {
  if (event.isComposing || event.getModifierState?.('AltGraph') || event.altKey) return false
  if (!event.ctrlKey && !event.metaKey) return false
  return event.code === 'KeyZ' || event.code === 'KeyY'
}

export function findMatchingShortcutAction(
  event: ShortcutEventLike,
  bindings: ShortcutBindings,
  scope?: ShortcutScope,
): ShortcutActionId | null {
  for (const action of SHORTCUT_ACTIONS) {
    if (scope && action.scope !== scope) continue
    for (const binding of bindings[action.id]) {
      if (matchesShortcut(event, binding)) return action.id
    }
  }
  return null
}

export function validateShortcutBinding(
  actionId: ShortcutActionId,
  binding: ShortcutBinding | null,
  bindings: ShortcutBindings,
  slot: ShortcutBindingSlot = 0,
): ShortcutValidationResult {
  if (!shortcutActionById.has(actionId)) {
    return { ok: false, code: 'UNKNOWN_ACTION', message: '不明なショートカット操作です。' }
  }
  if (binding === null) return { ok: true, binding: null }

  const normalized = normalizeBinding(binding)
  if (!normalized) {
    return {
      ok: false,
      code: 'INVALID_BINDING',
      message: 'Ctrl、Alt、Metaのいずれか、またはF1〜F12を含むキーを指定してください。',
    }
  }
  if (isReservedEditingBinding(normalized)) {
    return {
      ok: false,
      code: 'RESERVED_BINDING',
      message: 'このキーは文字入力やAI送信で使用するため割り当てできません。',
    }
  }

  for (const action of SHORTCUT_ACTIONS) {
    for (const conflictSlot of [0, 1] as const) {
      if (action.id === actionId && conflictSlot === slot) continue
      if (!shortcutBindingsEqual(bindings[action.id]?.[conflictSlot] ?? null, normalized)) continue
      return {
        ok: false,
        code: 'DUPLICATE_BINDING',
        message: `「${action.label}」と同じキーは割り当てできません。`,
        conflictingActionId: action.id,
        conflictingSlot: conflictSlot,
      }
    }
  }
  return { ok: true, binding: normalized }
}

function sanitizeShortcutBindings(value: unknown): ShortcutBindings {
  const result = createEmptyShortcutBindings()
  const source = value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {}

  for (const action of SHORTCUT_ACTIONS) {
    const hasStoredValue = Object.prototype.hasOwnProperty.call(source, action.id)
    const storedSlots = hasStoredValue && Array.isArray(source[action.id])
      ? source[action.id] as unknown[]
      : null

    for (const slot of [0, 1] as const) {
      const hasStoredSlot = storedSlots ? slot in storedSlots : false
      const storedValue = hasStoredSlot
        ? storedSlots?.[slot]
        : slot === 0
          ? action.defaultBinding
          : null
      let candidate = storedValue === null ? null : normalizeBinding(storedValue)
      if (storedValue !== null && !candidate) {
        candidate = slot === 0 ? cloneBinding(action.defaultBinding) : null
      }

      let validation = validateShortcutBinding(action.id, candidate, result, slot)
      if (!validation.ok && slot === 0 && candidate !== null) {
        candidate = cloneBinding(action.defaultBinding)
        validation = validateShortcutBinding(action.id, candidate, result, slot)
      }
      result[action.id][slot] = validation.ok ? cloneBinding(validation.binding) : null
    }
  }
  return result
}

function parseStoredDocument(raw: string | null) {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null
    }
    return parsed as Record<string, unknown>
  } catch {
    return null
  }
}

function convertLegacyShortcutBindings(value: unknown) {
  const source = value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {}
  return Object.fromEntries(
    SHORTCUT_ACTIONS.map((action) => [
      action.id,
      [
        Object.prototype.hasOwnProperty.call(source, action.id)
          ? source[action.id]
          : action.defaultBinding,
        null,
      ],
    ]),
  )
}

export function parseStoredShortcutBindings(
  raw: string | null,
  legacyRaw: string | null = null,
): ShortcutBindings {
  const currentRecord = parseStoredDocument(raw)
  if (currentRecord?.version === SHORTCUT_STORAGE_VERSION) {
    return sanitizeShortcutBindings(currentRecord.bindings)
  }

  const legacyRecord = parseStoredDocument(legacyRaw)
  if (legacyRecord?.version === SHORTCUT_LEGACY_STORAGE_VERSION) {
    return sanitizeShortcutBindings(convertLegacyShortcutBindings(legacyRecord.bindings))
  }

  return createDefaultShortcutBindings()
}

export function serializeShortcutBindings(bindings: ShortcutBindings) {
  return JSON.stringify({
    version: SHORTCUT_STORAGE_VERSION,
    bindings: sanitizeShortcutBindings(bindings),
  })
}

export function isKnownShortcutActionId(value: string): value is ShortcutActionId {
  return isShortcutActionId(value)
}
