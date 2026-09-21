import type { SettingsTab } from '../stores/useSettingsStore'

export type SettingsSearchItem = {
  id: string
  label: string
  category: string
  tab: SettingsTab
  anchor: string
  synonyms: readonly string[]
  displayText: readonly string[]
}

export type SettingsSearchResult = SettingsSearchItem & {
  matchedText: string
}

// Keep this index explicit. It covers text that is visible in settings panels
// without collecting DOM text or any runtime/user-entered values such as keys,
// URLs, paths, model ids, or diagnostic contents.
export const SETTINGS_SEARCH_INDEX: readonly SettingsSearchItem[] = [
  {
    id: 'theme',
    label: 'テーマ',
    category: '表示',
    tab: 'theme',
    anchor: 'theme',
    synonyms: ['外観', '色'],
    displayText: ['テーマ', 'アプリケーションテーマ', 'ライト', 'ダーク'],
  },
  {
    id: 'general.notebook-icon',
    label: 'ノートブックの既定アイコン',
    category: '一般',
    tab: 'general',
    anchor: 'general.notebook-icon',
    synonyms: ['アイコン', '初期アイコン'],
    displayText: ['ノートブック', '既定アイコン', 'ノートブックアイコン', 'アイコンを選択'],
  },
  {
    id: 'general.uninstall',
    label: 'アンインストールとインストール済みアプリ',
    category: '一般',
    tab: 'general',
    anchor: 'general.uninstall',
    synonyms: ['削除', 'Windows', '設定'],
    displayText: [
      'アンインストール',
      'アプリの削除',
      'Windows',
      'インストールされているアプリ',
      'Windowsのインストールされているアプリを開く',
    ],
  },
  {
    id: 'editor.font-family',
    label: 'フォント指定',
    category: 'エディター',
    tab: 'editor',
    anchor: 'editor.font-family',
    synonyms: ['文字', '書体'],
    displayText: ['タイポグラフィ', 'フォント指定', 'Meiryo', 'Yu Gothic UI', 'Noto Sans JP', 'BIZ UDPGothic'],
  },
  {
    id: 'editor.font-size',
    label: 'フォントサイズ指定',
    category: 'エディター',
    tab: 'editor',
    anchor: 'editor.font-size',
    synonyms: ['文字サイズ', '大きさ'],
    displayText: ['フォントサイズ指定', '12', '13', '14', '15', '16', '17', '18', '20', '22', '24', '26'],
  },
  {
    id: 'editor.first-line',
    label: '新規ノート1行目のスタイル',
    category: 'エディター',
    tab: 'editor',
    anchor: 'editor.first-line',
    synonyms: ['見出し', 'H1', 'H2', 'H3'],
    displayText: ['エディタ', '新規ノート1行目のスタイル', 'H1', 'H2', 'H3', '普通'],
  },
  {
    id: 'editor.line-length',
    label: '行の長さ',
    category: 'エディター',
    tab: 'editor',
    anchor: 'editor.line-length',
    synonyms: ['幅', '文字数'],
    displayText: ['行の長さ'],
  },
  {
    id: 'editor.line-height',
    label: '行間',
    category: 'エディター',
    tab: 'editor',
    anchor: 'editor.line-height',
    synonyms: ['高さ'],
    displayText: ['行間'],
  },
  {
    id: 'editor.paragraph-spacing',
    label: '段落の間隔',
    category: 'エディター',
    tab: 'editor',
    anchor: 'editor.paragraph-spacing',
    synonyms: ['段落', '余白'],
    displayText: ['段落の間隔'],
  },
  {
    id: 'editor.auto-save',
    label: '自動保存',
    category: 'エディター',
    tab: 'editor',
    anchor: 'editor.auto-save',
    synonyms: ['保存', 'Ctrl+S', '下書き', '手動保存'],
    displayText: ['自動保存を有効にする', 'OFF', 'Ctrl+S', '下書き', '明示操作'],
  },
  {
    id: 'shortcuts',
    label: 'ショートカット',
    category: '操作',
    tab: 'shortcuts',
    anchor: 'shortcuts',
    synonyms: ['キー', 'キーボード', 'Ctrl', '割り当て'],
    displayText: ['ショートカット', 'すべて初期化', '初期値に戻す', '解除'],
  },
  {
    id: 'sync',
    label: '同期',
    category: 'データ',
    tab: 'sync',
    anchor: 'sync',
    synonyms: ['WebDAV', '同期先', '保管庫'],
    displayText: ['同期', '初回設定', '同期先', 'WebDAV', '同期間隔', '詳細設定', '今すぐ同期'],
  },
  {
    id: 'sync.setup-mode',
    label: '初回設定方式',
    category: '同期',
    tab: 'sync',
    anchor: 'sync.setup-mode',
    synonyms: ['初期化', '取り込み', '再接続'],
    displayText: [
      '初回設定方式',
      'ローカルデータで新しい同期先を初期化',
      '既存の同期先を空のローカルへ取り込む',
      '同じ保管庫へ再接続',
    ],
  },
  {
    id: 'sync.target',
    label: '同期先',
    category: '同期',
    tab: 'sync',
    anchor: 'sync.target',
    synonyms: ['WebDAV'],
    displayText: ['同期先', 'WebDAV'],
  },
  {
    id: 'sync.interval',
    label: '同期間隔',
    category: '同期',
    tab: 'sync',
    anchor: 'sync.interval',
    synonyms: ['自動同期', '頻度'],
    displayText: ['同期間隔', '無効', '5分', '10分', '30分', '1時間', '12時間', '24時間'],
  },
  {
    id: 'sync.advanced',
    label: '同期の詳細設定',
    category: '同期',
    tab: 'sync',
    anchor: 'sync.advanced',
    synonyms: ['TLS', '証明書', 'プロキシ', '再アップロード', '再ダウンロード'],
    displayText: [
      '詳細設定',
      'TLS証明書のカスタマイズ',
      'プロキシURL',
      'プロキシのタイムアウト',
      'ローカルデータを同期先に再アップロードする',
      'ローカルデータを削除して同期先から再ダウンロードする',
    ],
  },
  {
    id: 'ai',
    label: 'AI設定',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai',
    synonyms: ['API', 'プロバイダー', 'モデル', '認証'],
    displayText: ['AI', 'AI設定', '適用', 'OK', '戻る'],
  },
  {
    id: 'ai.enabled',
    label: 'AI機能を有効にする',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai.enabled',
    synonyms: ['オン', 'オフ', '停止', '有効', '無効'],
    displayText: ['AI機能を有効にする', 'AIワークスペース', 'AI操作', 'OFF'],
  },
  {
    id: 'ai.workspace-placement',
    label: 'AIワークスペースの表示位置',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai.workspace-placement',
    synonyms: ['右側', '下側', '配置', '幅', '高さ'],
    displayText: ['AIワークスペース', '表示位置', '右側', '下側', '境界をドラッグして幅または高さを調整'],
  },
  {
    id: 'ai.agent-permission',
    label: 'Agentの本文編集権限',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai.agent-permission',
    synonyms: ['Agent', '自動適用', '提案'],
    displayText: [
      'Agentの本文編集権限',
      '提案のみ（適用前に確認）',
      '更新可能（生成後に自動適用）',
      '自動保存',
      'AIタイムライン',
      '変更はすぐに反映されます',
      '適用',
      'OK',
    ],
  },
  {
    id: 'ai.provider',
    label: 'AIプロバイダー',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai.provider',
    synonyms: ['プロバイダー', 'OpenRouter', 'Gemini'],
    displayText: ['プロバイダー', 'OpenRouter', 'Google Gemini'],
  },
  {
    id: 'ai.api-key',
    label: 'API Key',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai.api-key',
    synonyms: ['APIキー', '認証情報', '資格情報'],
    displayText: ['API Key', '保存済みの API Key', '認証を確認', 'モデル一覧を更新'],
  },
  {
    id: 'ai.model',
    label: '要約モデル',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai.model',
    synonyms: ['モデル', '生成', '要約'],
    displayText: ['要約モデル', 'モデルを選択してください', '生成を確認', '生成結果は保存していません'],
  },
  {
    id: 'ai.credentials',
    label: 'AI認証情報の削除',
    category: 'AI',
    tab: 'ai',
    anchor: 'ai.credentials',
    synonyms: ['認証', '資格情報', '削除'],
    displayText: ['AI 認証情報の削除', 'このプロバイダーの認証情報を削除', 'すべての AI 認証情報を削除'],
  },
  {
    id: 'storage-locations',
    label: '保存場所',
    category: 'データ',
    tab: 'storage-locations',
    anchor: 'storage-locations',
    synonyms: ['保管庫', '保存先', '移動'],
    displayText: ['保存場所', '保存領域', 'バックアップ保存領域', 'フォルダを選択', '変更は次回起動時に適用'],
  },
  /* {
    id: 'storage-locations.diagnostics',
    label: '診断情報',
    category: '保存場所',
    tab: 'storage-locations',
    anchor: 'storage-locations.diagnostics',
    synonyms: ['ログ', 'コピー', '保存', '調査'],
    displayText: ['診断情報', '安全な情報だけを表示', '診断情報をコピー'],
  }, */
  {
    id: 'storage-spaces',
    label: '保存空間',
    category: 'データ',
    tab: 'storage-locations',
    anchor: 'storage-spaces',
    synonyms: ['空間', 'ノートブック', '切り替え'],
    displayText: ['保存空間', '保存空間の一覧', '新しい保存空間', '切り替え'],
  },
  {
    id: 'backups',
    label: 'バックアップと復元',
    category: 'データ',
    tab: 'backups',
    anchor: 'backups',
    synonyms: ['復元', '自動バックアップ', '安全用'],
    displayText: ['バックアップ', 'バックアップと復元', '保存済みバックアップ', '自動バックアップ'],
  },
  {
    id: 'locks',
    label: 'ロック',
    category: 'セキュリティ',
    tab: 'locks',
    anchor: 'locks',
    synonyms: ['保護', '暗号化', '自動ロック'],
    displayText: ['ロック', '自動ロック', '現在設定されているロック', '暗号化'],
  },
  {
    id: 'help',
    label: 'ヘルプ',
    category: '案内',
    tab: 'help',
    anchor: 'help',
    synonyms: [],
    displayText: ['ヘルプ'],
  },
  /* {
    id: 'help.faq',
    label: 'よくある質問',
    category: 'ヘルプ',
    tab: 'help',
    anchor: 'help.faq',
    synonyms: ['使い方', '保存', 'AI', '履歴'],
    displayText: ['よくある質問', 'ノートが見つからない', '自動保存', '新しいチャット'],
  },
  {
    id: 'help.troubleshooting',
    label: '一般的な問題の切り分け',
    category: 'ヘルプ',
    tab: 'help',
    anchor: 'help.troubleshooting',
    synonyms: ['トラブルシューティング', '同期', '添付', '空き容量'],
    displayText: ['一般的な問題の切り分け', '起動', '同期', 'PNG', 'JPEG', '空き容量'],
  },
  {
    id: 'help.diagnostics-guide',
    label: '診断ログの保存方法',
    category: 'ヘルプ',
    tab: 'help',
    anchor: 'help.diagnostics-guide',
    synonyms: ['診断', 'ログ', 'ファイル', '問い合わせ'],
    displayText: ['診断ログを保存する前に', '診断ログを更新', 'ファイルに保存', 'コピー'],
  },
  {
    id: 'help.contact',
    label: '問い合わせ',
    category: 'ヘルプ',
    tab: 'help',
    anchor: 'help.contact',
    synonyms: ['連絡', 'サポート', '窓口', '診断情報'],
    displayText: ['問い合わせ', '問い合わせ窓口', '診断情報をコピー', 'APIキー', '本文', 'プロンプト'],
  }, */
  /* {
    id: 'help.diagnostics',
    label: '診断ログ',
    category: 'ヘルプ',
    tab: 'help',
    anchor: 'help.diagnostics',
    synonyms: ['診断情報', 'ログ', 'コピー', '保存', '安全'],
    displayText: ['診断ログ', '診断ログを更新', 'コピー', 'ファイルに保存', '本文', 'APIキー', 'プロンプト', 'ファイルパス'],
  }, */
]

function normalizeSearchText(value: string) {
  return value.normalize('NFKC').trim().toLocaleLowerCase()
}

function matchedDisplayText(item: SettingsSearchItem, terms: readonly string[]) {
  const matches = item.displayText.filter((displayText) => {
    const normalized = normalizeSearchText(displayText)
    return terms.some((term) => normalized.includes(term))
  })
  return (matches.length > 0 ? matches.slice(0, 3) : [item.label]).join(' / ')
}

export function searchSettings(query: string): SettingsSearchResult[] {
  const terms = normalizeSearchText(query).split(/\s+/).filter(Boolean)
  if (terms.length === 0) return []

  return SETTINGS_SEARCH_INDEX
    .filter((item) => {
      const searchable = normalizeSearchText([
        item.id,
        item.label,
        item.category,
        ...item.displayText,
        ...item.synonyms,
      ].join(' '))
      return terms.every((term) => searchable.includes(term))
    })
    .map((item) => ({
      ...item,
      matchedText: matchedDisplayText(item, terms),
    }))
}
