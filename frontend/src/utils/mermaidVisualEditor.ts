export type MermaidDiagramType =
  | 'flowchart'
  | 'sequence'
  | 'class'
  | 'state'
  | 'er'
  | 'gantt'
  | 'journey'
  | 'info'
  | 'pie'
  | 'quadrant'
  | 'requirement'
  | 'gitGraph'
  | 'mindmap'
  | 'timeline'
  | 'sankey'
  | 'xychart'
  | 'block'
  | 'architecture'
  | 'c4'
  | 'packet'
  | 'kanban'
  | 'radar'
  | 'treemap'
  | 'treeView'
  | 'swimlane'
  | 'eventmodeling'
  | 'ishikawa'
  | 'venn'
  | 'wardley'
  | 'cynefin'
  | 'railroad'
  | 'railroad-ebnf'
  | 'railroad-abnf'
  | 'railroad-peg'

export type MermaidFieldType = 'text' | 'textarea' | 'select'

export type MermaidFieldDefinition = {
  key: string
  label: string
  type?: MermaidFieldType
  options?: string[]
}

export type MermaidElementKindDefinition = {
  kind: string
  label: string
  fields: MermaidFieldDefinition[]
}

export type MermaidDiagramDefinition = {
  type: MermaidDiagramType
  label: string
  description: string
  sample: string
  elementKinds: MermaidElementKindDefinition[]
}

export type MermaidVisualElement = {
  id: string
  kind: string
  raw: string
  fields: Record<string, string>
  editable: boolean
  dirty: boolean
  blockId?: string
  blockRole?: 'start' | 'member' | 'end'
}

export type MermaidVisualDocument = {
  type: MermaidDiagramType
  prefix: string[]
  header: string
  elements: MermaidVisualElement[]
  trailingNewline: boolean
  unknownCount: number
}

const text = (key: string, label: string, type: MermaidFieldType = 'text'): MermaidFieldDefinition => ({
  key,
  label,
  type,
})

const select = (key: string, label: string, options: string[]): MermaidFieldDefinition => ({
  key,
  label,
  type: 'select',
  options,
})

const textarea = (key: string, label: string): MermaidFieldDefinition => ({
  key,
  label,
  type: 'textarea',
})

const rawKind: MermaidElementKindDefinition = {
  kind: 'raw',
  label: 'その他の行',
  fields: [textarea('content', '内容')],
}

const flowchartShapes = [
  'rect',
  'rounded',
  'stadium',
  'subroutine',
  'cylinder',
  'circle',
  'doublecircle',
  'diamond',
  'hexagon',
  'parallelogram',
  'trapezoid',
  'asymmetric',
  'lean-right',
  'lean-left',
  'delay',
  'cloud',
  'document',
  'stored-data',
  'tagged-document',
  'lined-document',
  'manual-file',
  'paper-tape',
  'divided-process',
  'lined-process',
  'card',
  'notched-rectangle',
  'small-circle',
  'framed-circle',
  'crossed-circle',
  'filled-circle',
  'fork',
] as const

const flowchartElementKinds: MermaidElementKindDefinition[] = [
  {
    kind: 'node',
    label: '図形',
    fields: [
      text('id', '識別子'),
      text('label', '表示文字'),
      select('shape', '図形', [...flowchartShapes]),
    ],
  },
  {
    kind: 'edge',
    label: '接続',
    fields: [
      text('from', '接続元'),
      text('fromLabel', '元の文字'),
      select('fromShape', '元の図形', [...flowchartShapes]),
      text('to', '接続先'),
      text('toLabel', '先の文字'),
      select('toShape', '先の図形', [...flowchartShapes]),
      select('arrow', '線', ['-->', '---', '-.->', '==>', '--o', '--x']),
      text('label', '線ラベル'),
    ],
  },
  {
    kind: 'group',
    label: 'グループ',
    fields: [text('title', 'グループ名'), textarea('content', '中身')],
  },
  {
    kind: 'direction',
    label: '向き',
    fields: [select('direction', '向き', ['TB', 'TD', 'BT', 'RL', 'LR'])],
  },
]

const sequenceElementKinds: MermaidElementKindDefinition[] = [
  { kind: 'participant', label: '参加者', fields: [text('name', '識別子'), text('alias', '表示名')] },
  {
    kind: 'message',
    label: 'メッセージ',
    fields: [
      text('from', '送信元'),
      text('to', '送信先'),
      select('arrow', '矢印', ['->>', '-->>', '-x', '--x', '->', '-->']),
      text('message', '内容'),
    ],
  },
  { kind: 'note', label: '注釈', fields: [text('over', '対象'), textarea('message', '内容')] },
  {
    kind: 'control',
    label: '分岐／繰返し',
    fields: [select('command', '種類', ['alt', 'else', 'opt', 'loop', 'par', 'and', 'critical', 'break', 'end']), text('label', '条件・説明')],
  },
]

const classElementKinds: MermaidElementKindDefinition[] = [
  { kind: 'class', label: 'クラス', fields: [text('name', 'クラス名')] },
  { kind: 'attribute', label: '属性', fields: [text('owner', 'クラス'), text('visibility', '公開範囲'), text('name', '名前'), text('type', '型')] },
  { kind: 'method', label: 'メソッド', fields: [text('owner', 'クラス'), text('visibility', '公開範囲'), text('name', '名前'), text('parameters', '引数'), text('returnType', '戻り値')] },
  { kind: 'relation', label: '関係', fields: [text('from', '元'), text('to', '先'), text('arrow', '関係'), text('label', 'ラベル')] },
]

const erElementKinds: MermaidElementKindDefinition[] = [
  { kind: 'entity', label: 'エンティティ', fields: [text('name', '名前')] },
  { kind: 'field', label: '属性', fields: [text('owner', 'エンティティ'), text('type', '型'), text('name', '名前'), text('key', 'キー')] },
  { kind: 'relation', label: '関係', fields: [text('from', '元'), text('cardinalityFrom', '元の多重度'), text('cardinalityTo', '先の多重度'), text('to', '先'), text('label', 'ラベル')] },
]

const stateElementKinds: MermaidElementKindDefinition[] = [
  { kind: 'state', label: '状態', fields: [text('id', '識別子'), text('label', '表示名')] },
  { kind: 'transition', label: '遷移', fields: [text('from', '元'), text('to', '先'), text('label', 'ラベル')] },
  { kind: 'start-end', label: '開始／終了', fields: [select('direction', '向き', ['start', 'end']), text('to', '接続先')] },
  { kind: 'composite', label: '複合状態', fields: [text('name', '名前'), textarea('content', '中身')] },
]

const chartElementKinds: MermaidElementKindDefinition[] = [
  { kind: 'item', label: '項目', fields: [text('label', '項目名'), text('value', '値')] },
  { kind: 'series', label: '系列', fields: [text('command', '種類'), text('label', '系列名'), text('values', '値（カンマ区切り）')] },
  { kind: 'axis', label: '軸', fields: [text('axis', '軸'), text('label', '名前'), text('range', '範囲')] },
  { kind: 'annotation', label: '区分／注釈', fields: [text('command', '種類'), text('label', '内容')] },
]

const scheduleElementKinds: MermaidElementKindDefinition[] = [
  { kind: 'section', label: '区分', fields: [text('name', '区分名')] },
  { kind: 'item', label: '項目', fields: [text('name', '項目名'), text('status', '状態'), text('start', '開始'), text('duration', '期間'), text('actor', '担当')] },
  { kind: 'annotation', label: '注釈', fields: [textarea('content', '内容')] },
]

const genericElementKinds: MermaidElementKindDefinition[] = [
  { kind: 'item', label: '要素', fields: [textarea('content', 'Mermaidの行')] },
  { kind: 'relation', label: '関係／接続', fields: [textarea('content', 'Mermaidの行')] },
  { kind: 'annotation', label: '注釈', fields: [textarea('content', 'Mermaidの行')] },
]

const definitions: MermaidDiagramDefinition[] = [
  {
    type: 'flowchart',
    label: 'フローチャート',
    description: '処理の流れ、分岐、接続を表します。',
    sample: 'flowchart TD\n  A[開始] --> B[終了]',
    elementKinds: flowchartElementKinds,
  },
  {
    type: 'sequence',
    label: 'シーケンス図',
    description: '参加者間のメッセージ、注釈、分岐・繰返しを表します。',
    sample: 'sequenceDiagram\n  participant A as 利用者\n  participant B as システム\n  A->>B: 依頼\n  B-->>A: 応答',
    elementKinds: sequenceElementKinds,
  },
  { type: 'class', label: 'クラス図', description: 'クラス、属性、メソッド、関係を表します。', sample: 'classDiagram\n  class User {\n    +String name\n    +login()\n  }\n  User --> Account : owns', elementKinds: classElementKinds },
  { type: 'state', label: '状態図', description: '状態、遷移、開始・終了、複合状態を表します。', sample: 'stateDiagram-v2\n  [*] --> 待機\n  待機 --> 実行\n  実行 --> [*]', elementKinds: stateElementKinds },
  { type: 'er', label: 'ER図', description: 'エンティティ、属性、関係を表します。', sample: 'erDiagram\n  USER ||--o{ ORDER : places\n  USER {\n    string id\n    string name\n  }', elementKinds: erElementKinds },
  { type: 'gantt', label: 'ガントチャート', description: '区分、項目、日付、期間、担当を表します。', sample: 'gantt\n  title 開発計画\n  dateFormat YYYY-MM-DD\n  section 作業\n  設計 :done, des1, 2026-01-01, 3d\n  実装 :active, des2, after des1, 5d', elementKinds: scheduleElementKinds },
  { type: 'journey', label: 'Journey', description: '利用者の行動と評価を時系列で表します。', sample: 'journey\n    title 利用者の流れ\n    section ノート\n      ノートを開く: 5: 利用者\n      編集する: 4: 利用者', elementKinds: scheduleElementKinds },
  { type: 'info', label: 'Info', description: 'Mermaidのバージョン情報を表示します。', sample: 'info', elementKinds: genericElementKinds },
  { type: 'pie', label: '円グラフ', description: '項目と値の割合を表します。', sample: 'pie title 構成\n  "本文" : 70\n  "図" : 30', elementKinds: [chartElementKinds[0], rawKind] },
  { type: 'quadrant', label: 'クアドラントチャート', description: '2軸の領域と項目を表します。', sample: 'quadrantChart\n  title 優先度\n  x-axis 低 --> 高\n  y-axis 低 --> 高\n  quadrant-1 重要\n  quadrant-2 改善\n  quadrant-3 保留\n  quadrant-4 低優先\n  項目: [0.7, 0.8]', elementKinds: chartElementKinds },
  { type: 'requirement', label: '要求図', description: '要求、要素、検証関係を表します。', sample: 'requirementDiagram\n\nrequirement test_req {\n  id: 1\n  text: the test text.\n  risk: high\n  verifyMethod: test\n}\n\nelement test_entity {\n  type: simulation\n}\n\ntest_entity - verifies -> test_req', elementKinds: [...classElementKinds.slice(0, 1), ...genericElementKinds] },
  { type: 'gitGraph', label: 'Gitグラフ', description: 'コミット、ブランチ、マージを表します。', sample: 'gitGraph\n  commit\n  branch develop\n  checkout develop\n  commit\n  checkout main\n  merge develop', elementKinds: genericElementKinds },
  { type: 'mindmap', label: 'マインドマップ', description: '親子関係と階層を表します。', sample: 'mindmap\n  root((Atlas Note))\n    Notes\n      Markdown\n    Diagrams\n      Mermaid', elementKinds: [{ kind: 'node', label: '階層ノード', fields: [text('level', '階層'), text('label', '文字')] }, rawKind] },
  { type: 'timeline', label: 'タイムライン', description: '区分、日付、出来事を表します。', sample: 'timeline\n  title 更新履歴\n  2026-01 : 開始\n  2026-02 : 改善', elementKinds: scheduleElementKinds },
  { type: 'sankey', label: 'Sankey図', description: '流量と接続元・接続先を表します。', sample: 'sankey-beta\n  A,B,10\n  B,C,6\n  B,D,4', elementKinds: [{ kind: 'flow', label: '流れ', fields: [text('from', '元'), text('to', '先'), text('value', '値')] }, rawKind] },
  { type: 'xychart', label: 'XYチャート', description: '項目、値、系列、軸を表します。', sample: 'xychart-beta\n  title "進捗"\n  x-axis [1, 2, 3]\n  y-axis "値" 0 --> 10\n  bar [2, 5, 8]\n  line [1, 4, 9]', elementKinds: chartElementKinds },
  { type: 'block', label: 'ブロック図', description: 'ブロック、列、接続を表します。', sample: 'block-beta\n  columns 2\n  A["入力"] B["処理"]\n  A --> B', elementKinds: [...flowchartElementKinds.slice(0, 2), rawKind] },
  { type: 'architecture', label: 'アーキテクチャ図', description: 'サービス、グループ、接続を表します。', sample: 'architecture-beta\n  group api(cloud)[API]\n  service app(server)[App] in api\n  service db(database)[DB] in api\n  app:R --> L:db', elementKinds: [...flowchartElementKinds.slice(0, 2), rawKind] },
  { type: 'c4', label: 'C4図', description: '人物、システム、コンテナ、関係を表します。', sample: 'C4Context\nPerson(user, "User")\nSystem(app, "Atlas Note")\nRel(user, app, "Uses")', elementKinds: [...classElementKinds.slice(0, 1), { kind: 'relation', label: '関係', fields: [text('from', '元'), text('to', '先'), text('label', 'ラベル')] }, rawKind] },
  { type: 'packet', label: 'パケット図', description: 'ビット範囲とフィールドを表します。', sample: 'packet-beta\n  0-7: "Header"\n  8-15: "Body"', elementKinds: [{ kind: 'field', label: 'フィールド', fields: [text('range', '範囲'), text('label', '名前')] }, rawKind] },
  { type: 'kanban', label: 'カンバン', description: '区分とカードを表します。', sample: 'kanban\n  Todo\n    [設計]\n  Doing\n    [実装]', elementKinds: [scheduleElementKinds[0], { kind: 'card', label: 'カード', fields: [text('section', '区分'), text('label', 'カード')] }, rawKind] },
  { type: 'radar', label: 'レーダーチャート', description: '軸と系列の値を表します。', sample: 'radar-beta\n  title Skills\n  axis A, B, C, D\n  curve Current { 2, 3, 4, 3 }', elementKinds: chartElementKinds },
  { type: 'treemap', label: 'ツリーマップ', description: '親子の項目と値を表します。', sample: 'treemap\n"根"\n  "子A": 6\n  "子B": 4', elementKinds: [{ kind: 'node', label: '階層項目', fields: [text('level', '階層'), text('label', '項目'), text('value', '値')] }, rawKind] },
  { type: 'treeView', label: 'ツリービュー', description: 'インデントで階層を表します。', sample: 'treeView-beta\n  ルート\n    子項目', elementKinds: [{ kind: 'node', label: '階層ノード', fields: [text('level', '階層'), text('label', '文字')] }, rawKind] },
  { type: 'swimlane', label: 'スイムレーン図', description: 'レーン、項目、流れを表します。', sample: 'swimlane-beta\n  利用者[利用者] --> ノート[ノートを開く]', elementKinds: genericElementKinds },
  { type: 'eventmodeling', label: 'イベントモデリング', description: 'コマンド、イベント、人物を表します。', sample: 'eventmodeling\n  entity user\n  tf 1 ui user\n  tf 2 evt saved', elementKinds: genericElementKinds },
  { type: 'ishikawa', label: '特性要因図', description: '結果と原因の階層を表します。', sample: 'ishikawa-beta\n  title 原因分析\n  effect: 保存失敗\n  cause: 入力エラー', elementKinds: genericElementKinds },
  { type: 'venn', label: 'ベン図', description: '集合と重なりを表します。', sample: 'venn-beta\n  set A\n  set B\n  union A, B', elementKinds: chartElementKinds },
  { type: 'wardley', label: 'Wardleyマップ', description: '要素、位置、進化度を表します。', sample: 'wardley-beta\n  title 例\n  anchor User [0.1, 1.0]\n  component Product [0.5, 0.5]', elementKinds: genericElementKinds },
  { type: 'cynefin', label: 'Cynefin図', description: '判断領域と要素を表します。', sample: 'cynefin-beta\n  title 判断\n  clear\n    "例"', elementKinds: genericElementKinds },
  { type: 'railroad', label: 'Railroad図', description: '文法の規則を表します。', sample: 'railroad-beta\nrule = sequence(terminal("開始"), optional(terminal("終了")));', elementKinds: genericElementKinds },
  { type: 'railroad-ebnf', label: 'EBNF Railroad図', description: 'EBNF規則を表します。', sample: 'railroad-ebnf-beta\nrule = "開始", ["終了"];', elementKinds: genericElementKinds },
  { type: 'railroad-abnf', label: 'ABNF Railroad図', description: 'ABNF規則を表します。', sample: 'railroad-abnf-beta\nrule = "開始";', elementKinds: genericElementKinds },
  { type: 'railroad-peg', label: 'PEG Railroad図', description: 'PEG規則を表します。', sample: 'railroad-peg-beta\nrule <- "開始";', elementKinds: genericElementKinds },
]

const definitionMap = new Map(definitions.map((definition) => [definition.type, definition]))

const HEADER_PATTERNS: Array<[MermaidDiagramType, RegExp]> = [
  ['flowchart', /^\s*(?:flowchart|flowchart-elk|graph)\b/i],
  ['sequence', /^\s*sequenceDiagram\b/i],
  ['class', /^\s*classDiagram(?:-v2)?\b/i],
  ['state', /^\s*stateDiagram(?:-v2)?\b/i],
  ['er', /^\s*erDiagram\b/i],
  ['gantt', /^\s*gantt\b/i],
  ['journey', /^\s*journey\b/i],
  ['info', /^\s*(?:info|showInfo)\b/i],
  ['pie', /^\s*pie\b/i],
  ['quadrant', /^\s*quadrantChart\b/i],
  ['requirement', /^\s*requirement(?:Diagram)?\b/i],
  ['gitGraph', /^\s*gitGraph\b/i],
  ['mindmap', /^\s*mindmap\b/i],
  ['timeline', /^\s*timeline\b/i],
  ['sankey', /^\s*sankey(?:-beta)?\b/i],
  ['xychart', /^\s*xychart(?:-beta)?\b/i],
  ['block', /^\s*block(?:-beta)?\b/i],
  ['architecture', /^\s*architecture\b/i],
  ['c4', /^\s*C4(?:Context|Container|Component|Dynamic|Deployment)\b/],
  ['packet', /^\s*packet(?:-beta)?\b/i],
  ['kanban', /^\s*kanban\b/i],
  ['radar', /^\s*radar-beta\b/i],
  ['treemap', /^\s*treemap\b/i],
  ['treeView', /^\s*treeView-beta\b/i],
  ['swimlane', /^\s*swimlane-beta\b/i],
  ['eventmodeling', /^\s*eventmodeling\b/i],
  ['ishikawa', /^\s*ishikawa(?:-beta)?\b/i],
  ['venn', /^\s*venn-beta\b/i],
  ['wardley', /^\s*wardley-beta\b/i],
  ['cynefin', /^\s*cynefin-beta\b/i],
  ['railroad', /^\s*railroad-beta\b/i],
  ['railroad-ebnf', /^\s*railroad-ebnf-beta\b/i],
  ['railroad-abnf', /^\s*railroad-abnf-beta\b/i],
  ['railroad-peg', /^\s*railroad-peg-beta\b/i],
]

export const MERMAID_DIAGRAM_CATALOG = definitions as readonly MermaidDiagramDefinition[]

export function getMermaidDiagramDefinition(type: MermaidDiagramType) {
  return definitionMap.get(type) ?? definitions[0]
}

export function getMermaidElementKindDefinitions(type: MermaidDiagramType) {
  return getMermaidDiagramDefinition(type).elementKinds
}

export function detectMermaidDiagramType(source: string): MermaidDiagramType | null {
  const firstMeaningfulLine = source
    .replace(/^\uFEFF/, '')
    .split(/\r\n|\n|\r/)
    .find((line) => line.trim() && !line.trimStart().startsWith('%%'))
  if (!firstMeaningfulLine) return null

  return HEADER_PATTERNS.find(([, pattern]) => pattern.test(firstMeaningfulLine))?.[0] ?? null
}

function fieldValue(fields: Record<string, string>, key: string, fallback = '') {
  return fields[key] ?? fallback
}

function element(
  index: number,
  kind: string,
  raw: string,
  fields: Record<string, string>,
  editable = true,
): MermaidVisualElement {
  return {
    id: `mermaid-element-${index + 1}`,
    kind,
    raw,
    fields,
    editable,
    dirty: false,
  }
}

function unquote(value: string) {
  const trimmed = value.trim()
  if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
    return trimmed.slice(1, -1)
  }
  return trimmed
}

function escapeQuoted(value: string) {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\r?\n|\r/g, ' ')
}

function parseFlowEndpoint(value: string) {
  const trimmed = value.trim()
  const match = trimmed.match(/^([A-Za-z_][\w-]*)(.*)$/)
  if (!match) return { id: trimmed, label: '', shape: 'rect' }
  const id = match[1]
  const suffix = match[2].trim()
  if (!suffix) return { id, label: id, shape: 'rect' }
  if (suffix.startsWith('@{')) {
    const shape = suffix.match(/shape\s*:\s*([\w-]+)/i)?.[1] ?? 'rect'
    const label = suffix.match(/label\s*:\s*["']([^"']*)["']/i)?.[1] ?? id
    return { id, label, shape }
  }
  const pairs: Array<[string, RegExp]> = [
    ['doublecircle', /^\(\(\((.*)\)\)\)$/],
    ['circle', /^\(\((.*)\)\)$/],
    ['stadium', /^\(\[([^\]]*)\]\)$/],
    ['subroutine', /^\[\[([^\]]*)\]\]$/],
    ['cylinder', /^\[\((.*)\)\]$/],
    ['diamond', /^\{(.*)\}$/],
    ['hexagon', /^\{\{(.*)\}\}$/],
    ['parallelogram', /^\[\/(.*)\/\]$/],
    ['trapezoid', /^\[\\(.*)\\\/\]$/],
    ['rect', /^\[(.*)\]$/],
    ['rounded', /^\((.*)\)$/],
  ]
  for (const [shape, pattern] of pairs) {
    const label = suffix.match(pattern)?.[1]
    if (label !== undefined) return { id, label, shape }
  }
  return { id, label: suffix, shape: 'rect' }
}

function parseFlowLine(line: string, index: number, seenNodes: Set<string>) {
  const trimmed = line.trim()
  if (!trimmed || trimmed.startsWith('%%')) return element(index, 'raw', line, { content: line }, false)

  const edgeMatch = trimmed.match(/^(.+?)\s*(<-->|-->|---|-.->|==>|===|--o|--x|->|~>)\s*(?:\|([^|]*)\|\s*)?(.+)$/)
  if (edgeMatch) {
    const from = parseFlowEndpoint(edgeMatch[1])
    const to = parseFlowEndpoint(edgeMatch[4])
    seenNodes.add(from.id)
    seenNodes.add(to.id)
    return element(index, 'edge', line, {
      from: from.id,
      fromLabel: from.label,
      fromShape: from.shape,
      to: to.id,
      toLabel: to.label,
      toShape: to.shape,
      arrow: edgeMatch[2],
      label: edgeMatch[3]?.trim() ?? '',
    })
  }

  const nodeMatch = trimmed.match(/^([A-Za-z_][\w-]*)(.*)$/)
  if (nodeMatch && (nodeMatch[2].trim().startsWith('[') || nodeMatch[2].trim().startsWith('(') || nodeMatch[2].trim().startsWith('{') || nodeMatch[2].trim().startsWith('@{') || nodeMatch[2].trim().startsWith('>'))) {
    const node = parseFlowEndpoint(trimmed)
    seenNodes.add(node.id)
    return element(index, 'node', line, { id: node.id, label: node.label, shape: node.shape })
  }

  if (/^subgraph\b/i.test(trimmed)) return element(index, 'group', line, { title: trimmed.replace(/^subgraph\s*/i, ''), content: '' })
  if (/^direction\s+/i.test(trimmed)) return element(index, 'direction', line, { direction: trimmed.split(/\s+/)[1] ?? 'TD' })
  return element(index, 'raw', line, { content: line }, false)
}

function parseSequenceLine(line: string, index: number) {
  const trimmed = line.trim()
  let match = trimmed.match(/^(participant|actor)\s+([^\s]+)(?:\s+as\s+(.+))?$/i)
  if (match) return element(index, 'participant', line, { name: match[2], alias: match[3] ?? '' })
  match = trimmed.match(/^Note\s+(?:over|left of|right of)\s+([^:]+):\s*(.*)$/i)
  if (match) return element(index, 'note', line, { over: match[1].trim(), message: match[2] })
  match = trimmed.match(/^([^\s]+)\s*(-->>|->>|--x|-[xX]|-->|->|\-\-\)|\-\))\s*([^:]+):\s*(.*)$/)
  if (match) return element(index, 'message', line, { from: match[1], arrow: match[2], to: match[3].trim(), message: match[4] })
  match = trimmed.match(/^(alt|else|opt|loop|par|and|critical|break|end)\b\s*(.*)$/i)
  if (match) return element(index, 'control', line, { command: match[1], label: match[2] })
  return element(index, 'raw', line, { content: line }, false)
}

function parseClassLine(line: string, index: number, inClass: string | null) {
  const trimmed = line.trim()
  const classMatch = trimmed.match(/^class\s+([\w.-]+)(?:\s+as\s+(.+))?\s*\{?$/i)
  if (classMatch) return element(index, 'class', line, { name: classMatch[1], label: classMatch[2] ?? '' })
  const relation = trimmed.match(/^([\w.-]+)\s+([.o*<>|+\-]+)\s+([\w.-]+)(?:\s*:\s*(.*))?$/)
  if (relation) return element(index, 'relation', line, { from: relation[1], arrow: relation[2], to: relation[3], label: relation[4] ?? '' })
  if (inClass && /^[-+~#]/.test(trimmed)) {
    const member = trimmed.match(/^([-+~#])?\s*([\w.-]+)(?:\(([^)]*)\))?(?:\s*:\s*(.+))?$/)
    if (member) {
      if (member[3] !== undefined) return element(index, 'method', line, { owner: inClass, visibility: member[1] ?? '+', name: member[2], parameters: member[3], returnType: member[4] ?? '' })
      return element(index, 'attribute', line, { owner: inClass, visibility: member[1] ?? '+', name: member[2], type: member[4] ?? '' })
    }
  }
  return element(index, 'raw', line, { content: line }, false)
}

function parseErLine(line: string, index: number, inEntity: string | null) {
  const trimmed = line.trim()
  const relation = trimmed.match(/^([\w.-]+)\s+(\|\||o\{|\|o|\}o|[|o}{-]+)--([|o}{-]+)\s+([\w.-]+)\s*:\s*(.*)$/)
  if (relation) return element(index, 'relation', line, { from: relation[1], cardinalityFrom: relation[2], cardinalityTo: relation[3], to: relation[4], label: relation[5] })
  const entity = trimmed.match(/^([\w.-]+)\s*\{$/)
  if (entity) return element(index, 'entity', line, { name: entity[1] })
  if (inEntity) {
    const field = trimmed.match(/^([^\s]+)\s+([^\s]+)(?:\s+(.*))?$/)
    if (field) return element(index, 'field', line, { owner: inEntity, type: field[1], name: field[2], key: field[3] ?? '' })
  }
  return element(index, 'raw', line, { content: line }, false)
}

function parseStateLine(line: string, index: number) {
  const trimmed = line.trim()
  const transition = trimmed.match(/^(.+?)\s+-->\s+(.+?)(?:\s*:\s*(.*))?$/)
  if (transition) {
    const start = transition[1].trim()
    const end = transition[2].trim()
    if (start === '[*]' || end === '[*]') return element(index, 'start-end', line, { direction: start === '[*]' ? 'start' : 'end', to: start === '[*]' ? end : start })
    return element(index, 'transition', line, { from: start, to: end, label: transition[3] ?? '' })
  }
  const named = trimmed.match(/^state\s+"([^"]+)"\s+as\s+([\w.-]+)/i)
  if (named) return element(index, 'state', line, { id: named[2], label: named[1] })
  if (/^state\s+\w+\s*\{$/i.test(trimmed)) return element(index, 'composite', line, { name: trimmed.replace(/^state\s+|\s*\{$/gi, ''), content: '' })
  return element(index, 'raw', line, { content: line }, false)
}

function parseGenericLine(type: MermaidDiagramType, line: string, index: number) {
  const trimmed = line.trim()
  if (!trimmed || trimmed.startsWith('%%')) return element(index, 'raw', line, { content: line }, false)

  if (type === 'pie') {
    const match = trimmed.match(/^(?:"([^"]+)"|'([^']+)'|([^:]+))\s*:\s*(-?\d+(?:\.\d+)?)$/)
    if (match) return element(index, 'item', line, { label: match[1] ?? match[2] ?? match[3].trim(), value: match[4] })
  }
  if (type === 'sankey') {
    const match = trimmed.match(/^([^,]+),([^,]+),(.+)$/)
    if (match) return element(index, 'flow', line, { from: match[1].trim(), to: match[2].trim(), value: match[3].trim() })
  }
  if (type === 'mindmap') {
    const match = line.match(/^(\s*)(.*)$/)
    if (match && match[2].trim() && !match[2].trim().startsWith('::')) return element(index, 'node', line, { level: String(Math.floor(match[1].length / 2)), label: match[2].trim() })
  }
  if (type === 'timeline') {
    const match = trimmed.match(/^([^:]+):\s*(.*)$/)
    if (match) return element(index, 'item', line, { name: match[1].trim(), label: match[2], start: match[1].trim(), duration: '', actor: '' })
  }
  if (type === 'gantt' || type === 'journey') {
    if (/^section\b/i.test(trimmed)) return element(index, 'section', line, { name: trimmed.replace(/^section\s*/i, '') })
    const match = trimmed.match(/^(.+?):\s*(.*)$/)
    if (match) return element(index, 'item', line, { name: match[1].trim(), label: match[1].trim(), content: match[2], status: '', start: '', duration: '', actor: '' })
  }
  if (type === 'xychart') {
    const match = trimmed.match(/^(bar|line)\s*\[([^]]*)\]$/i)
    if (match) return element(index, 'series', line, { command: match[1], label: '', values: match[2] })
    const axis = trimmed.match(/^(x-axis|y-axis)\s+(.+)$/i)
    if (axis) return element(index, 'axis', line, { axis: axis[1], label: axis[2], range: axis[2] })
  }
  if (type === 'packet') {
    const match = trimmed.match(/^([^:]+):\s*(.*)$/)
    if (match) return element(index, 'field', line, { range: match[1], label: unquote(match[2]) })
  }
  if (type === 'kanban') {
    const card = trimmed.match(/^\[(.*)\]$/)
    if (card) return element(index, 'card', line, { section: '', label: card[1] })
    if (!/^(kanban|title|dateFormat|axis|curve|columns|section|requirement|element|Person|System|Rel|anchor|component|effect|cause|rule|choice|terminal|start|end|lane|task|person|command|event):?/i.test(trimmed)) return element(index, 'section', line, { name: trimmed })
  }
  if (type === 'class' || type === 'c4') return parseClassLine(line, index, null)
  return element(index, 'item', line, { content: line })
}

function parseLine(type: MermaidDiagramType, line: string, index: number, state: { inClass: string | null; inEntity: string | null; seenNodes: Set<string> }) {
  if (type === 'flowchart' || type === 'block' || type === 'architecture') return parseFlowLine(line, index, state.seenNodes)
  if (type === 'sequence') return parseSequenceLine(line, index)
  if (type === 'class') return parseClassLine(line, index, state.inClass)
  if (type === 'er') return parseErLine(line, index, state.inEntity)
  if (type === 'state') return parseStateLine(line, index)
  return parseGenericLine(type, line, index)
}

function fieldValueOrEmpty(fields: Record<string, string>, key: string) {
  return (fields[key] ?? '').replace(/\r?\n|\r/g, ' ').trim()
}

function shapeSyntax(shape: string, label: string) {
  const safeLabel = escapeQuoted(label || '新しい要素')
  switch (shape) {
    case 'rounded': return `(${safeLabel})`
    case 'stadium': return `([${safeLabel}])`
    case 'subroutine': return `[[${safeLabel}]]`
    case 'cylinder': return `[(${safeLabel})]`
    case 'circle': return `((${safeLabel}))`
    case 'doublecircle': return `((("${safeLabel}")))`
    case 'diamond': return `{${safeLabel}}`
    case 'hexagon': return `{{${safeLabel}}}`
    case 'parallelogram': return `[/${safeLabel}/]`
    case 'trapezoid': return `[\\${safeLabel}\\/]`
    case 'asymmetric': return `>${safeLabel}]`
    case 'rect': return `[${safeLabel}]`
    default: return `@{ shape: ${shape}, label: "${safeLabel}" }`
  }
}

function flowEndpointSyntax(id: string, shape: string, label: string) {
  if (!label && shape === 'rect') return id || 'N'
  return `${id || 'N'}${shapeSyntax(shape || 'rect', label || id || '新しい要素')}`
}

function formatElement(type: MermaidDiagramType, item: MermaidVisualElement) {
  const fields = item.fields
  switch (item.kind) {
    case 'node':
      if (type === 'mindmap') return `${'  '.repeat(Math.max(0, Number(fieldValue(fields, 'level', '1'))))}${fieldValueOrEmpty(fields, 'label') || '新しい要素'}`
      if (type === 'treemap') return `${'  '.repeat(Math.max(0, Number(fieldValue(fields, 'level', '1'))))}"${escapeQuoted(fieldValueOrEmpty(fields, 'label') || '新しい要素')}": ${fieldValueOrEmpty(fields, 'value') || '1'}`
      return `  ${fieldValueOrEmpty(fields, 'id') || 'N'}${shapeSyntax(fieldValue(fields, 'shape', 'rect'), fieldValue(fields, 'label'))}`
    case 'edge':
      if (type === 'flowchart') {
        const from = flowEndpointSyntax(fieldValueOrEmpty(fields, 'from') || 'A', fieldValue(fields, 'fromShape', 'rect'), fieldValueOrEmpty(fields, 'fromLabel'))
        const to = flowEndpointSyntax(fieldValueOrEmpty(fields, 'to') || 'B', fieldValue(fields, 'toShape', 'rect'), fieldValueOrEmpty(fields, 'toLabel'))
        return `  ${from} ${fieldValue(fields, 'arrow', '-->')} ${fieldValue(fields, 'label') ? `|${fieldValueOrEmpty(fields, 'label')}| ` : ''}${to}`
      }
      return `  ${fieldValueOrEmpty(fields, 'from') || 'A'} ${fieldValue(fields, 'arrow', '-->')} ${fieldValue(fields, 'label') ? `|${fieldValueOrEmpty(fields, 'label')}| ` : ''}${fieldValueOrEmpty(fields, 'to') || 'B'}`
    case 'group':
      return `  subgraph ${fieldValueOrEmpty(fields, 'title') || 'グループ'}${fieldValue(fields, 'content') ? `\n${fieldValue(fields, 'content')}` : ''}\n  end`
    case 'direction': return `  direction ${fieldValue(fields, 'direction', 'TD')}`
    case 'participant': return `  participant ${fieldValueOrEmpty(fields, 'name') || 'A'}${fieldValue(fields, 'alias') ? ` as ${fieldValueOrEmpty(fields, 'alias')}` : ''}`
    case 'message': return `  ${fieldValueOrEmpty(fields, 'from') || 'A'}${fieldValue(fields, 'arrow', '->>')}${fieldValueOrEmpty(fields, 'to') || 'B'}: ${fieldValueOrEmpty(fields, 'message') || 'メッセージ'}`
    case 'note': return `  Note over ${fieldValueOrEmpty(fields, 'over') || 'A'}: ${fieldValueOrEmpty(fields, 'message') || '注釈'}`
    case 'control': return `  ${fieldValueOrEmpty(fields, 'command') || 'alt'}${fieldValue(fields, 'label') ? ` ${fieldValueOrEmpty(fields, 'label')}` : ''}`
    case 'class': return `  class ${fieldValueOrEmpty(fields, 'name') || 'NewClass'} {`
    case 'attribute': return `    ${fieldValue(fields, 'visibility', '+')}${fieldValueOrEmpty(fields, 'name') || 'value'}${fieldValue(fields, 'type') ? ` : ${fieldValueOrEmpty(fields, 'type')}` : ''}`
    case 'method': return `    ${fieldValue(fields, 'visibility', '+')}${fieldValueOrEmpty(fields, 'name') || 'method'}(${fieldValueOrEmpty(fields, 'parameters')})${fieldValue(fields, 'returnType') ? ` : ${fieldValueOrEmpty(fields, 'returnType')}` : ''}`
    case 'relation':
      if (type === 'er') return `  ${fieldValueOrEmpty(fields, 'from') || 'A'} ${fieldValue(fields, 'cardinalityFrom', '||')}--${fieldValue(fields, 'cardinalityTo', 'o{')} ${fieldValueOrEmpty(fields, 'to') || 'B'} : ${fieldValueOrEmpty(fields, 'label') || 'relates'}`
      if (type === 'c4') return `  Rel(${fieldValueOrEmpty(fields, 'from') || 'a'}, ${fieldValueOrEmpty(fields, 'to') || 'b'}, "${escapeQuoted(fieldValueOrEmpty(fields, 'label') || 'relates')}")`
      return `  ${fieldValueOrEmpty(fields, 'from') || 'A'} ${fieldValue(fields, 'arrow', '-->')} ${fieldValueOrEmpty(fields, 'to') || 'B'}${fieldValue(fields, 'label') ? ` : ${fieldValueOrEmpty(fields, 'label')}` : ''}`
    case 'transition': return `  ${fieldValueOrEmpty(fields, 'from') || 'A'} --> ${fieldValueOrEmpty(fields, 'to') || 'B'}${fieldValue(fields, 'label') ? ` : ${fieldValueOrEmpty(fields, 'label')}` : ''}`
    case 'start-end': return fieldValue(fields, 'direction') === 'end' ? `  ${fieldValueOrEmpty(fields, 'to') || 'A'} --> [*]` : `  [*] --> ${fieldValueOrEmpty(fields, 'to') || 'A'}`
    case 'state': return `  state "${escapeQuoted(fieldValueOrEmpty(fields, 'label') || fieldValueOrEmpty(fields, 'id') || '新しい状態')}" as ${fieldValueOrEmpty(fields, 'id') || 'NewState'}`
    case 'composite': return `  state ${fieldValueOrEmpty(fields, 'name') || 'Composite'} {${fieldValue(fields, 'content') ? `\n${fieldValue(fields, 'content')}\n  ` : ''}}`
    case 'entity': return `  ${fieldValueOrEmpty(fields, 'name') || 'ENTITY'} {`
    case 'field':
      if (type === 'er') return `    ${fieldValueOrEmpty(fields, 'type') || 'string'} ${fieldValueOrEmpty(fields, 'name') || 'value'}${fieldValue(fields, 'key') ? ` ${fieldValueOrEmpty(fields, 'key')}` : ''}`
      if (type === 'packet') return `  ${fieldValueOrEmpty(fields, 'range') || '0-7'}: "${escapeQuoted(fieldValueOrEmpty(fields, 'label') || 'Field')}"`
      return `  ${fieldValueOrEmpty(fields, 'content')}`
    case 'section': {
      const name = fieldValueOrEmpty(fields, 'name') || '新しい区分'
      return type === 'gantt' || type === 'journey' ? `  section ${name}` : `  ${name}`
    }
    case 'card': return `    [${fieldValueOrEmpty(fields, 'label') || '新しいカード'}]`
    case 'item':
      if (type === 'pie') return `  "${escapeQuoted(fieldValueOrEmpty(fields, 'label') || '項目')}" : ${fieldValueOrEmpty(fields, 'value') || '1'}`
      if (type === 'timeline') return `  ${fieldValueOrEmpty(fields, 'start') || fieldValueOrEmpty(fields, 'name') || '2026-01'} : ${fieldValueOrEmpty(fields, 'label') || '新しい出来事'}`
      if (type === 'gantt') return `  ${fieldValueOrEmpty(fields, 'name') || '新しい作業'} :${fieldValue(fields, 'status') ? `${fieldValueOrEmpty(fields, 'status')}, ` : ' '}${fieldValueOrEmpty(fields, 'start') || '2026-01-01'}, ${fieldValueOrEmpty(fields, 'duration') || '1d'}`
      if (type === 'journey') return `    ${fieldValueOrEmpty(fields, 'name') || '新しい行動'}: ${fieldValueOrEmpty(fields, 'value') || '3'}: ${fieldValueOrEmpty(fields, 'actor') || '利用者'}`
      return `  ${fieldValueOrEmpty(fields, 'content') || fieldValueOrEmpty(fields, 'label') || '新しい要素'}`
    case 'flow': return `  ${fieldValueOrEmpty(fields, 'from') || 'A'},${fieldValueOrEmpty(fields, 'to') || 'B'},${fieldValueOrEmpty(fields, 'value') || '1'}`
    case 'series': return `  ${fieldValueOrEmpty(fields, 'command') || 'bar'} [${fieldValueOrEmpty(fields, 'values') || '1, 2, 3'}]`
    case 'axis': return `  ${fieldValueOrEmpty(fields, 'axis') || 'x-axis'} ${fieldValueOrEmpty(fields, 'label') || '[1, 2, 3]'}`
    case 'annotation': return `  ${fieldValueOrEmpty(fields, 'command') || 'title'} ${fieldValueOrEmpty(fields, 'label') || fieldValueOrEmpty(fields, 'content') || '注釈'}`
    default: return fieldValue(fields, 'content', item.raw)
  }
}

type MermaidBlock = {
  id: string
  body: MermaidVisualElement[]
  end?: MermaidVisualElement
  name: string
}

function isBlockStart(type: MermaidDiagramType, item: MermaidVisualElement) {
  return (type === 'class' && item.kind === 'class') || (type === 'er' && item.kind === 'entity')
}

function isBlockMember(type: MermaidDiagramType, item: MermaidVisualElement) {
  return type === 'class'
    ? item.kind === 'attribute' || item.kind === 'method'
    : type === 'er' && item.kind === 'field'
}

function blockName(item: MermaidVisualElement) {
  return fieldValueOrEmpty(item.fields, 'name')
}

function generateBlockAwareElements(document: MermaidVisualDocument) {
  if (document.type !== 'class' && document.type !== 'er') {
    return document.elements.map((item) => item.dirty && item.editable ? formatElement(document.type, item) : item.raw)
  }

  const blocks = new Map<string, MermaidBlock>()
  for (const item of document.elements) {
    if (isBlockStart(document.type, item)) {
      const id = item.blockId ?? item.id
      const block = blocks.get(id) ?? {
        id,
        body: [],
        name: blockName(item),
      }
      block.name = blockName(item)
      blocks.set(id, block)
      continue
    }

    if (!item.blockId) continue
    const block = blocks.get(item.blockId)
    if (!block) continue
    if (item.blockRole === 'end') block.end = item
    else block.body.push(item)
  }

  const additionsByBlock = new Map<string, MermaidVisualElement[]>()
  for (const item of document.elements) {
    if (!isBlockMember(document.type, item) || item.blockId || !item.editable || !item.dirty) continue
    const owner = fieldValueOrEmpty(item.fields, 'owner')
    const candidates = [...blocks.values()]
    const target = owner
      ? candidates.find((block) => block.name === owner)
      : candidates[candidates.length - 1]
    if (!target) continue
    const additions = additionsByBlock.get(target.id) ?? []
    additions.push(item)
    additionsByBlock.set(target.id, additions)
  }

  const renderedBlocks = new Set<string>()
  const lines: string[] = []
  for (const item of document.elements) {
    if (isBlockStart(document.type, item)) {
      const block = blocks.get(item.blockId ?? item.id)
      if (!block || renderedBlocks.has(block.id)) continue

      lines.push(item.dirty && item.editable ? formatElement(document.type, item) : item.raw)
      for (const bodyItem of block.body) {
        lines.push(bodyItem.dirty && bodyItem.editable ? formatElement(document.type, bodyItem) : bodyItem.raw)
      }
      for (const addedItem of additionsByBlock.get(block.id) ?? []) {
        lines.push(formatElement(document.type, addedItem))
      }
      lines.push(block.end?.raw ?? '  }')
      renderedBlocks.add(block.id)
      continue
    }

    if (item.blockId) continue
    if (isBlockMember(document.type, item) && !item.blockId && item.editable && item.dirty) continue
    lines.push(item.dirty && item.editable ? formatElement(document.type, item) : item.raw)
  }

  return lines
}

export function generateMermaidSource(document: MermaidVisualDocument): string {
  const lines = [
    ...document.prefix,
    document.header,
    ...generateBlockAwareElements(document),
  ]
  const generated = lines.join('\n')
  return document.trailingNewline ? `${generated}\n` : generated
}

export function parseMermaidVisualSource(source: string): MermaidVisualDocument | null {
  const type = detectMermaidDiagramType(source)
  if (!type) return null
  const lines = source.replace(/^\uFEFF/, '').split(/\r\n|\n|\r/)
  const trailingNewline = /(?:\r\n|\n|\r)$/.test(source)
  if (trailingNewline && lines[lines.length - 1] === '') lines.pop()
  const headerIndex = lines.findIndex((line) => HEADER_PATTERNS.some(([candidate, pattern]) => candidate === type && pattern.test(line)))
  if (headerIndex < 0) return null
  const prefix = lines.slice(0, headerIndex)
  const header = lines[headerIndex]
  const state = { inClass: null as string | null, inEntity: null as string | null, seenNodes: new Set<string>() }
  const elements: MermaidVisualElement[] = []
  let unknownCount = 0
  let activeBlock: { id: string } | null = null
  for (const [index, line] of lines.slice(headerIndex + 1).entries()) {
    const parsed = parseLine(type, line, index, state)
    if (parsed.kind === 'class' || parsed.kind === 'entity') {
      activeBlock = { id: parsed.id }
      parsed.blockId = activeBlock.id
      parsed.blockRole = 'start'
    } else if (activeBlock) {
      parsed.blockId = activeBlock.id
      parsed.blockRole = parsed.kind === 'raw' && line.trim() === '}' ? 'end' : 'member'
    }
    if (parsed.kind === 'class') state.inClass = parsed.fields.name ?? null
    if (parsed.kind === 'entity') state.inEntity = parsed.fields.name ?? null
    if (parsed.kind === 'raw' && line.trim() === '}') {
      state.inClass = null
      state.inEntity = null
      activeBlock = null
    }
    if (!parsed.editable) unknownCount += 1
    elements.push(parsed)
  }
  return { type, prefix, header, elements, trailingNewline, unknownCount }
}

export function createMermaidElement(type: MermaidDiagramType, kind: string, index = 0): MermaidVisualElement {
  const definitionsForType = getMermaidElementKindDefinitions(type)
  const definition = definitionsForType.find((candidate) => candidate.kind === kind) ?? definitionsForType[0] ?? rawKind
  const fields = Object.fromEntries(definition.fields.map((field) => [field.key, field.options?.[0] ?? '']))
  if (kind === 'node' && type === 'flowchart') Object.assign(fields, { id: `N${index + 1}`, label: '新しい要素', shape: 'rect' })
  if (kind === 'edge') Object.assign(fields, {
    from: 'A',
    fromLabel: 'A',
    fromShape: 'rect',
    to: 'B',
    toLabel: 'B',
    toShape: 'rect',
    arrow: '-->',
    label: '',
  })
  if (kind === 'participant') Object.assign(fields, { name: `P${index + 1}`, alias: '参加者' })
  if (kind === 'message') Object.assign(fields, { from: 'A', to: 'B', arrow: '->>', message: 'メッセージ' })
  if (kind === 'class') Object.assign(fields, { name: `Class${index + 1}` })
  if (kind === 'entity') Object.assign(fields, { name: `ENTITY${index + 1}` })
  if (kind === 'state') Object.assign(fields, { id: `State${index + 1}`, label: '新しい状態' })
  if (kind === 'item') Object.assign(fields, { label: '新しい項目', value: '1', name: '新しい項目', start: '2026-01-01', duration: '1d' })
  if (kind === 'section') Object.assign(fields, { name: '新しい区分' })
  if (kind === 'raw') Object.assign(fields, { content: '' })
  return {
    id: `mermaid-element-new-${Date.now()}-${index}`,
    kind: definition.kind,
    raw: '',
    fields,
    editable: true,
    dirty: true,
  }
}

export function getMermaidElementLabel(type: MermaidDiagramType, kind: string) {
  return getMermaidElementKindDefinitions(type).find((definition) => definition.kind === kind)?.label ?? '要素'
}

export function getMermaidElementDefinition(type: MermaidDiagramType, kind: string) {
  return getMermaidElementKindDefinitions(type).find((definition) => definition.kind === kind) ?? rawKind
}

function hasUnsafeShapeMetadata(source: string) {
  if (!/@\s*\{/.test(source)) return false
  if (!/^\s*(?:flowchart|graph)\b/im.test(source)) return true

  const metadataPattern = /@\s*\{/g
  let match: RegExpExecArray | null
  while ((match = metadataPattern.exec(source)) !== null) {
    const closeIndex = source.indexOf('}', match.index + match[0].length)
    if (closeIndex < 0) return true
    const entries = source.slice(match.index + match[0].length, closeIndex)
      .split(',')
      .map((entry) => entry.trim())
      .filter(Boolean)
    if (entries.length === 0 || entries.length > 2) return true

    const keys = new Set<string>()
    for (const entry of entries) {
      const separator = entry.indexOf(':')
      if (separator <= 0) return true
      const key = entry.slice(0, separator).trim().toLowerCase()
      const value = entry.slice(separator + 1).trim()
      if (keys.has(key) || !['shape', 'label'].includes(key)) return true
      keys.add(key)

      if (key === 'shape') {
        if (!/^[a-z][a-z0-9-]*$/i.test(value) || !flowchartShapes.includes(value.toLowerCase() as typeof flowchartShapes[number])) return true
        continue
      }

      if (!/^"[^"\r\n]*"|'[^'\r\n]*'$/.test(value)) return true
      if (/(?:https?:|data:|file:|javascript:|\/\/)|<\/?[a-z][^>]*>|!\[|@\s*\{/i.test(value)) return true
    }

    if (!keys.has('shape')) return true
    metadataPattern.lastIndex = closeIndex + 1
  }

  return false
}

export function isMermaidEditorValueSafe(value: string) {
  return value.length <= 50_000
    && !hasUnsafeShapeMetadata(value)
    && !/%%\s*\{/.test(value)
    && !/^\s*---\s*(?:[\r\n]|$)/.test(value)
    && !/(?:^|[;\r\n])\s*(?:sequenceDiagram\s+)?(?:click|callback|init|properties|details|links?)\b/i.test(value)
    && !/\b(?:iconify|icon\s*[:(])/i.test(value)
    && !/\b(?:service|group)\s+[\w-]+\s*\([^)]*:/i.test(value)
    && !/(?:https?:|data:|file:|javascript:|\/\/)/i.test(value)
    && !/\burl\s*\(|@import|!\[/i.test(value)
    && !/<\/?[a-z][^>]*>/i.test(value)
    && !/(?:^|[;\r\n])\s*(?:style|classDef|linkStyle)\b[^\r\n]*(?:\\|\/\*)/i.test(value)
}

export function flowchartConnectionCount(document: MermaidVisualDocument, nodeId: string) {
  return document.elements.filter((item) => item.kind === 'edge' && (item.fields.from === nodeId || item.fields.to === nodeId)).length
}
