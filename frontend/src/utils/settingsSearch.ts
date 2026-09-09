import type { SettingsTab } from '../stores/useSettingsStore'

export type SettingsSearchItem = {
  id: string
  label: string
  category: string
  tab: SettingsTab
  anchor: string
  synonyms: readonly string[]
}

export const SETTINGS_SEARCH_INDEX: readonly SettingsSearchItem[] = [
  { id: 'theme', label: 'テーマ', category: '表示', tab: 'theme', anchor: 'theme', synonyms: ['外観', 'ライト', 'ダーク', '色'] },
  { id: 'general.notebook-icon', label: 'ノートブックの既定アイコン', category: '一般', tab: 'general', anchor: 'general.notebook-icon', synonyms: ['アイコン', '初期アイコン'] },
  { id: 'general.ai-placement', label: 'AIワークスペースの表示位置', category: '一般', tab: 'general', anchor: 'general.ai-placement', synonyms: ['AI', '右側', '下側', '配置'] },
  { id: 'general.ai-agent-permission', label: 'Agentの本文編集権限', category: '一般', tab: 'general', anchor: 'general.ai-agent-permission', synonyms: ['AI', 'Agent', '自動適用', '提案'] },
  { id: 'general.uninstall', label: 'アンインストールとインストール済みアプリ', category: '一般', tab: 'general', anchor: 'general.uninstall', synonyms: ['削除', 'Windows', '設定'] },
  { id: 'editor.font-family', label: 'フォント指定', category: 'エディター', tab: 'editor', anchor: 'editor.font-family', synonyms: ['文字', '書体'] },
  { id: 'editor.font-size', label: 'フォントサイズ指定', category: 'エディター', tab: 'editor', anchor: 'editor.font-size', synonyms: ['文字サイズ', '大きさ'] },
  { id: 'editor.first-line', label: '新規ノート1行目のスタイル', category: 'エディター', tab: 'editor', anchor: 'editor.first-line', synonyms: ['見出し', 'H1', 'H2', 'H3'] },
  { id: 'editor.line-length', label: '行の長さ', category: 'エディター', tab: 'editor', anchor: 'editor.line-length', synonyms: ['幅', '文字数'] },
  { id: 'editor.line-height', label: '行間', category: 'エディター', tab: 'editor', anchor: 'editor.line-height', synonyms: ['高さ'] },
  { id: 'editor.paragraph-spacing', label: '段落の間隔', category: 'エディター', tab: 'editor', anchor: 'editor.paragraph-spacing', synonyms: ['段落', '余白'] },
  { id: 'shortcuts', label: 'ショートカット', category: '操作', tab: 'shortcuts', anchor: 'shortcuts', synonyms: ['キー', 'キーボード', 'Ctrl', '割り当て'] },
  { id: 'sync', label: '同期', category: 'データ', tab: 'sync', anchor: 'sync', synonyms: ['WebDAV', '同期先', '保管庫'] },
  { id: 'ai', label: 'AI設定', category: 'AI', tab: 'ai', anchor: 'ai', synonyms: ['API', 'プロバイダー', 'モデル', '認証'] },
  { id: 'ai.enabled', label: 'AI機能を有効にする', category: 'AI', tab: 'ai', anchor: 'ai.enabled', synonyms: ['オン', 'オフ', '停止', '有効', '無効'] },
  { id: 'storage-locations', label: '保存場所', category: 'データ', tab: 'storage-locations', anchor: 'storage-locations', synonyms: ['保管庫', '保存先', '移動'] },
  { id: 'backups', label: 'バックアップと復元', category: 'データ', tab: 'backups', anchor: 'backups', synonyms: ['復元', '自動バックアップ'] },
  { id: 'locks', label: 'ロック', category: 'セキュリティ', tab: 'locks', anchor: 'locks', synonyms: ['保護', '暗号化', '自動ロック'] },
  { id: 'help', label: 'ヘルプ', category: '案内', tab: 'help', anchor: 'help', synonyms: ['使い方', '操作', 'データ', 'Markdown'] },
  { id: 'help.contact', label: '問い合わせ', category: 'ヘルプ', tab: 'help', anchor: 'help.contact', synonyms: ['連絡', 'サポート', '窓口', '診断情報'] },
]

function normalizeSearchText(value: string) {
  return value.normalize('NFKC').trim().toLocaleLowerCase()
}

export function searchSettings(query: string) {
  const terms = normalizeSearchText(query).split(/\s+/).filter(Boolean)
  if (terms.length === 0) return []

  return SETTINGS_SEARCH_INDEX.filter((item) => {
    const searchable = normalizeSearchText([
      item.id,
      item.label,
      item.category,
      ...item.synonyms,
    ].join(' '))
    return terms.every((term) => searchable.includes(term))
  })
}
