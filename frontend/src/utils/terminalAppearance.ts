import type { ITheme } from '@xterm/xterm'

export const TERMINAL_FONT_SIZES = [12, 13, 14, 15, 16, 18, 20] as const
export const TERMINAL_FONT_FAMILIES = ['default', 'Consolas', 'Cascadia Mono', 'monospace'] as const
export const TERMINAL_CURSOR_STYLES = ['block', 'bar', 'underline'] as const
export const TERMINAL_SCROLLBACK_OPTIONS = [1000, 2000, 5000, 10000] as const
export function terminalFontFamily(value: typeof TERMINAL_FONT_FAMILIES[number]) {
  const fallback = 'Consolas, "Cascadia Mono", "Courier New", monospace'
  return value === 'default' ? fallback : value === 'monospace' ? value : `"${value}", ${fallback}`
}
export const TERMINAL_THEME_IDS = ['atlas-dark', 'vscode-dark', 'light', 'high-contrast'] as const
export type TerminalThemeId = typeof TERMINAL_THEME_IDS[number]
export const TERMINAL_THEMES: Record<TerminalThemeId, { label: string; theme: ITheme }> = {
  'atlas-dark': {
    label: 'Atlas Dark',
    theme: {
      background: '#0d1117', foreground: '#e6edf3', cursor: '#e6edf3', cursorAccent: '#0d1117', selectionBackground: '#264f78',
      black: '#484f58', red: '#ff7b72', green: '#3fb950', yellow: '#d29922',
      blue: '#58a6ff', magenta: '#bc8cff', cyan: '#39c5cf', white: '#b1bac4',
      brightBlack: '#6e7681', brightRed: '#ffa198', brightGreen: '#56d364', brightYellow: '#e3b341',
      brightBlue: '#79c0ff', brightMagenta: '#d2a8ff', brightCyan: '#56d4dd', brightWhite: '#ffffff',
    },
  },
  'vscode-dark': {
    label: 'VS Code Dark',
    theme: {
      background: '#1e1e1e', foreground: '#cccccc', cursor: '#ffffff', cursorAccent: '#1e1e1e', selectionBackground: '#264f78',
      black: '#000000', red: '#cd3131', green: '#0dbc79', yellow: '#e5e510',
      blue: '#2472c8', magenta: '#bc3fbc', cyan: '#11a8cd', white: '#e5e5e5',
      brightBlack: '#666666', brightRed: '#f14c4c', brightGreen: '#23d18b', brightYellow: '#f5f543',
      brightBlue: '#3b8eea', brightMagenta: '#d670d6', brightCyan: '#29b8db', brightWhite: '#ffffff',
    },
  },
  light: {
    label: 'Light',
    theme: {
      background: '#ffffff', foreground: '#0f172a', cursor: '#0f172a', cursorAccent: '#ffffff', selectionBackground: '#bfdbfe',
      black: '#000000', red: '#a31515', green: '#16713a', yellow: '#806000',
      blue: '#0451a5', magenta: '#881798', cyan: '#006b75', white: '#555555',
      brightBlack: '#666666', brightRed: '#c42020', brightGreen: '#187b3d', brightYellow: '#906b00',
      brightBlue: '#0066bf', brightMagenta: '#a029ad', brightCyan: '#007a85', brightWhite: '#333333',
    },
  },
  'high-contrast': {
    label: 'High Contrast',
    theme: {
      background: '#000000', foreground: '#ffffff', cursor: '#ffffff', cursorAccent: '#000000', selectionBackground: '#005f87',
      black: '#808080', red: '#ff6b6b', green: '#7fff00', yellow: '#ffff00',
      blue: '#69b7ff', magenta: '#ff80ff', cyan: '#00ffff', white: '#ffffff',
      brightBlack: '#b0b0b0', brightRed: '#ff9999', brightGreen: '#b3ff80', brightYellow: '#ffff80',
      brightBlue: '#a3d3ff', brightMagenta: '#ffb3ff', brightCyan: '#80ffff', brightWhite: '#ffffff',
    },
  },
}
