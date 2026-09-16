<template>
  <section class="help-settings" data-settings-anchor="help" tabindex="-1">
    <h3>ヘルプ</h3>
    <p class="help-intro">
      基本操作は「Atlas Note 利用ガイド」にまとめています。ここでは、よくある問題の切り分け、問い合わせ、診断ログの保存を案内します。
    </p>

    <section
      v-for="section in helpSections"
      :key="section.id"
      class="help-section"
      :data-settings-anchor="section.id"
      tabindex="-1"
    >
      <h4>{{ section.title }}</h4>
      <p v-for="paragraph in section.paragraphs" :key="paragraph">{{ paragraph }}</p>
      <ul v-if="section.items" class="help-list">
        <li v-for="item in section.items" :key="item">{{ item }}</li>
      </ul>
    </section>

    <section class="help-section help-contact" data-settings-anchor="help.contact" tabindex="-1">
      <h4>問い合わせ</h4>
      <p>問い合わせ窓口は未設定・未公開です。</p>
      <p>
        問題が起きた場合は、設定の「保存場所」にある診断情報をコピーし、指定された窓口へ共有してください。APIキー、本文、プロンプトなどの秘密情報は共有しないでください。
      </p>
    </section>

    <section class="help-section help-diagnostics" data-settings-anchor="help.diagnostics" tabindex="-1">
      <h4>診断ログ</h4>
      <p>操作の失敗を調査するための安全なメタデータです。本文、タイトル、APIキー、プロンプト、ツール履歴、ファイルパスは含めません。</p>
      <div class="help-diagnostics-actions">
        <button type="button" :disabled="diagnosticsLoading" @click="loadDiagnostics">
          {{ diagnosticsLoading ? '取得中…' : '診断ログを更新' }}
        </button>
        <button type="button" :disabled="!diagnosticReport || diagnosticsLoading" @click="copyDiagnostics">
          コピー
        </button>
        <button type="button" :disabled="diagnosticsLoading" @click="saveDiagnosticsFile">
          ファイルに保存
        </button>
      </div>
      <p v-if="diagnosticsMessage" class="help-diagnostics-message" role="status">{{ diagnosticsMessage }}</p>
      <pre v-if="diagnosticReport" class="help-diagnostics-report" aria-label="診断ログ">{{ diagnosticReport }}</pre>
      <p v-else class="help-diagnostics-empty">記録された診断ログはありません。</p>
    </section>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
import { getDiagnostics, saveDiagnostics } from '../api/diagnostics'

type HelpSection = {
  id: string
  title: string
  paragraphs: readonly string[]
  items?: readonly string[]
}

const helpSections: readonly HelpSection[] = [
  {
    id: 'help.faq',
    title: 'よくある質問',
    paragraphs: [
      'ノートが見つからない場合は、選択中のノートブック、検索条件、ゴミ箱を確認してください。保存場所を切り替えた直後は、切り替え先の保存空間が表示されているかも確認します。',
      '自動保存をOFFにしても入力中の下書きは保持されます。保存するときは Ctrl+S、または設定の自動保存をONにします。保存失敗・競合中は、状態表示の案内を解消してから再試行してください。',
      'AIの完了した質問・応答はこの端末の履歴へ自動保存されます。保存済み履歴と同じ会話を続ける場合は履歴一覧から開き、新しい会話は「新しいチャット」を使います。',
    ],
  },
  {
    id: 'help.troubleshooting',
    title: '一般的な問題の切り分け',
    paragraphs: [
      '起動・読み込みに失敗した場合は、保存場所設定と空き容量を確認し、ノートやデータフォルダーを手動で削除・移動しないでください。必要ならバックアップからの復元を検討します。',
      '同期が進まない場合は、未保存の下書き、保存競合、同期先の接続状態を順に確認します。未保存の変更がある間は自動同期が保留されるため、先にローカル保存を完了してください。',
      '画像をドラッグして添付できない場合は、PNGまたはJPEGであること、1枚10 MiB以下、ノート全体64 MiB以下であることを確認してください。その他のファイル形式は現在の添付対象外です。',
      '問題が再現する場合は、下の診断ログを更新してから「ファイルに保存」または「コピー」を使います。本文、APIキー、プロンプト、ファイルパスは診断ログに含めない設計です。',
    ],
  },
  {
    id: 'help.diagnostics-guide',
    title: '診断ログを保存する前に',
    paragraphs: [
      'まず「診断ログを更新」を押し、問題が起きた操作の直後に取得します。「コピー」は問い合わせ本文へ貼り付ける場合、「ファイルに保存」は添付や保管が必要な場合に使います。',
      '保存ダイアログをキャンセルした場合は失敗ではありません。保存に失敗した場合は保存場所の空き容量・権限を確認し、アプリを終了せずに再試行してください。',
    ],
  },
]

const diagnosticReport = ref('')
const diagnosticsLoading = ref(false)
const diagnosticsMessage = ref('')

async function loadDiagnostics() {
  if (diagnosticsLoading.value) return
  diagnosticsLoading.value = true
  diagnosticsMessage.value = ''
  try {
    const result = await getDiagnostics()
    diagnosticReport.value = result.report ?? ''
    if (!diagnosticReport.value) diagnosticsMessage.value = '診断ログはありません。'
  } catch {
    diagnosticReport.value = ''
    diagnosticsMessage.value = '診断ログを取得できませんでした。'
  } finally {
    diagnosticsLoading.value = false
  }
}

async function copyDiagnostics() {
  if (!diagnosticReport.value) await loadDiagnostics()
  if (!diagnosticReport.value) return
  try {
    const copied = await ClipboardSetText(diagnosticReport.value)
    diagnosticsMessage.value = copied
      ? '診断ログをクリップボードにコピーしました。'
      : '診断ログのコピーに失敗しました。'
  } catch {
    diagnosticsMessage.value = '診断ログのコピーに失敗しました。'
  }
}

async function saveDiagnosticsFile() {
  diagnosticsMessage.value = ''
  diagnosticsLoading.value = true
  try {
    const result = await saveDiagnostics()
    diagnosticsMessage.value = result.saved
      ? `診断ログを保存しました: ${result.savedName ?? 'ファイル'}`
      : result.cancelled
        ? '診断ログの保存をキャンセルしました。'
        : result.error ?? '診断ログを保存できませんでした。'
  } catch {
    diagnosticsMessage.value = '診断ログを保存できませんでした。'
  } finally {
    diagnosticsLoading.value = false
  }
}

onMounted(() => {
  void loadDiagnostics()
})
</script>

<style scoped>
.help-settings {
  color: var(--text-primary);
  padding-bottom: 24px;
}

.help-settings h3 {
  margin: 0 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
  font-size: 1.1rem;
}

.help-intro,
.help-section p {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.help-intro {
  margin: 0 0 20px;
}

.help-section {
  margin-bottom: 22px;
  scroll-margin-top: 16px;
}

.help-section h4 {
  margin: 0 0 8px;
  color: var(--text-primary);
  font-size: 14px;
}

.help-section p {
  margin: 6px 0;
}

.help-list {
  display: grid;
  gap: 5px;
  margin: 8px 0 0;
  padding-left: 20px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.help-contact {
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

.help-diagnostics-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.help-diagnostics-actions button {
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
}

.help-diagnostics-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.help-diagnostics-message,
.help-diagnostics-empty {
  color: var(--text-secondary);
  font-size: 12px;
}

.help-diagnostics-report {
  max-height: 260px;
  overflow: auto;
  margin: 0;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-secondary);
  font-size: 11px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
