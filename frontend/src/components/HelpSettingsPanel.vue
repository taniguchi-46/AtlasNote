<template>
  <section class="help-settings" data-settings-anchor="help" tabindex="-1">
    <h3>ヘルプ</h3>
    <p class="help-intro">
      Atlas Noteの基本操作と、データを安全に扱うための案内です。画面に表示される設定名から設定検索も利用できます。
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
  </section>
</template>

<script setup lang="ts">
type HelpSection = {
  id: string
  title: string
  paragraphs: readonly string[]
  items?: readonly string[]
}

const helpSections: readonly HelpSection[] = [
  {
    id: 'help.notes',
    title: 'ノートとノートブック',
    paragraphs: [
      '左側のノートブックを選ぶと、その中のノートを一覧できます。ノートブックは階層化でき、ノートは作成時または移動操作で所属を変更できます。',
      '本文はMarkdownを正本として保存し、通常の編集は自動保存の対象です。保存失敗や競合が表示された場合は、最新版の再読込または下書きのコピー保存を確認してから操作してください。',
    ],
  },
  {
    id: 'help.recovery',
    title: '保存場所・復旧・診断情報',
    paragraphs: [
      '保存場所は「保存場所」設定で確認できます。保存場所の移動や切り替えは、画面の確認結果と案内を読んでから実行してください。',
      '障害の調査では、保存場所設定の「診断情報をコピー」を使えます。診断情報は安全な状態確認用であり、問い合わせ時に必要な範囲だけ共有してください。',
    ],
  },
  {
    id: 'help.backups',
    title: 'バックアップ・復元・ゴミ箱',
    paragraphs: [
      'バックアップは保存空間の復旧用コピーです。復元前には安全用バックアップが作成されるため、復元先と内容を確認してから実行してください。',
      'ゴミ箱への移動はノートを通常一覧から外す操作で、バックアップの削除や完全削除とは異なります。ゴミ箱を空にする操作は元に戻せないため、必要なノートを復元してから実行してください。',
    ],
  },
  {
    id: 'help.markdown',
    title: 'Markdown編集',
    paragraphs: [
      'Markdownモードでは、見出し・箇条書き・番号付きリスト・タスクなどをそのまま編集できます。リスト項目の末尾でEnterを押すと、次の項目を自動的に続けます。空の項目でEnterを押すとリストを終了します。',
      'コードフェンスと水平線の中ではリスト継続を行いません。IME入力中や範囲選択中のEnterは通常の入力として扱います。',
    ],
  },
  {
    id: 'help.ai',
    title: 'AI機能',
    paragraphs: [
      'AI設定のスイッチをOFFにすると、AIワークスペース、リサイズ境界、エディターのAIアイコンを非表示にしてAI操作を停止します。AI設定・履歴・認証情報は削除しません。',
      'AIへの送信や本文への適用は既存の確認・保存・競合チェックを経由します。送信内容と保存範囲を確認してから実行してください。',
    ],
  },
  {
    id: 'help.shortcuts',
    title: 'ショートカット',
    paragraphs: [
      'ショートカットは各操作につき2つまで登録できます。同じキーを複数の操作へ登録することはできません。Esc、Delete、Backspaceで枠ごとの割り当てを解除できます。',
      '設定を初期値に戻すと、その操作の2枠をまとめて初期状態へ戻します。',
    ],
  },
  {
    id: 'help.uninstall',
    title: 'アンインストール',
    paragraphs: [
      'アプリのアンインストールはWindowsの「インストールされているアプリ」から行います。アンインストールしても、ノートや保存空間などのユーザーデータは削除しません。必要なデータは先にバックアップしてください。',
    ],
  },
]
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
</style>
