<template>
<section data-settings-anchor="terminal" tabindex="-1">
              <h3>ターミナル</h3>
              <div class="settings-section" data-settings-anchor="terminal.placement" tabindex="-1">
                <h4>表示位置</h4>
                <div class="terminal-segments" role="group" aria-label="パネルの表示位置">
                  <button type="button" :aria-pressed="settingsStore.aiWorkspacePlacement === 'bottom'" @click="settingsStore.aiWorkspacePlacement = 'bottom'">下</button>
                  <button type="button" :aria-pressed="settingsStore.aiWorkspacePlacement === 'right'" @click="settingsStore.aiWorkspacePlacement = 'right'">右</button>
                </div>
              </div>
              <div class="settings-section">
                <h4>フォント</h4>
                <div class="setting-group" data-settings-anchor="terminal.font-size" tabindex="-1">
                  <label>文字サイズ</label>
                  <div class="terminal-segments" role="group" aria-label="ターミナルの文字サイズ">
                    <button v-for="size in TERMINAL_FONT_SIZES" :key="size" type="button" :aria-pressed="settingsStore.terminalFontSize === size" @click="settingsStore.terminalFontSize = size">{{ size }}</button>
                  </div>
                </div>
                <div class="setting-group" data-settings-anchor="terminal.font-family" tabindex="-1">
                  <label for="terminal-font">フォント指定</label>
                  <select id="terminal-font" v-model="settingsStore.terminalFont"><option v-for="font in TERMINAL_FONT_FAMILIES" :key="font" :value="font">{{ font === 'default' ? '既定' : font }}</option></select>
                  <p class="setting-help">未インストールのフォントは等幅フォントへ切り替わります。</p>
                </div>
              </div>
              <div class="settings-section" data-settings-anchor="terminal.theme" tabindex="-1">
                <h4>外観</h4>
                <div class="terminal-themes" role="group" aria-label="ターミナルの配色">
                  <button v-for="id in TERMINAL_THEME_IDS" :key="id" type="button" :aria-pressed="settingsStore.terminalTheme === id" @click="settingsStore.terminalTheme = id">
                    <span class="terminal-theme-preview" :style="{ background: TERMINAL_THEMES[id].theme.background, color: TERMINAL_THEMES[id].theme.foreground }" aria-hidden="true"><span :style="{ color: TERMINAL_THEMES[id].theme.blue }">━━━</span><br>━━━━<br><span :style="{ color: TERMINAL_THEMES[id].theme.green }">━━</span></span>
                    {{ TERMINAL_THEMES[id].label }}
                  </button>
                </div>
                <p class="setting-help">アプリのテーマとは独立して適用します。設定はこの端末に保存されます。</p>
              </div>
              <div class="settings-section">
                <h4>動作</h4>
                <div class="setting-group" data-settings-anchor="terminal.cursor" tabindex="-1">
                  <label for="terminal-cursor">カーソル</label>
                  <select id="terminal-cursor" v-model="settingsStore.terminalCursorStyle"><option v-for="style in TERMINAL_CURSOR_STYLES" :key="style" :value="style">{{ style === 'block' ? 'Block' : style === 'bar' ? 'Bar' : 'Underline' }}</option></select>
                  <label class="setting-checkbox terminal-blink"><input v-model="settingsStore.terminalCursorBlink" type="checkbox">カーソルを点滅する</label>
                </div>
                <div class="setting-group" data-settings-anchor="terminal.scrollback" tabindex="-1">
                  <label for="terminal-scrollback">保持する出力行数</label>
                  <select id="terminal-scrollback" v-model="settingsStore.terminalScrollback"><option v-for="lines in TERMINAL_SCROLLBACK_OPTIONS" :key="lines" :value="lines">{{ lines }}</option></select>
                </div>
              </div>
              <div class="settings-section">
                <h4>ショートカット案内</h4>
                <p class="setting-help">ターミナル内のキー入力はshellへ送信します。Ctrl+Cで実行中のコマンドを中断できます。</p>
                <button type="button" class="secondary-button" @click="settingsStore.openSettings('shortcuts')">アプリのショートカット設定を開く</button>
              </div>
            </section>
</template>

<script setup lang="ts">
import { useSettingsStore } from '../stores/useSettingsStore'
import { TERMINAL_FONT_SIZES, TERMINAL_THEME_IDS, TERMINAL_THEMES, TERMINAL_FONT_FAMILIES, TERMINAL_CURSOR_STYLES, TERMINAL_SCROLLBACK_OPTIONS } from '../utils/terminalAppearance'
const settingsStore = useSettingsStore()
</script>

<style scoped>
.terminal-segments{display:flex;flex-wrap:wrap;gap:0;width:fit-content;border:1px solid var(--border-strong);border-radius:4px;overflow:hidden}
.terminal-segments button{min-width:38px;padding:4px 12px;border-right:1px solid var(--border);background:var(--bg-input);color:var(--text-primary);font-size:12px}
.terminal-segments button:last-child{border-right:0}
.terminal-segments button[aria-pressed='true']{background:var(--brand-primary);color:#fff}
.terminal-segments button:focus-visible,.terminal-themes button:focus-visible{outline:2px solid var(--text-primary);outline-offset:-3px}
.terminal-themes{display:flex;flex-wrap:wrap;gap:8px}
.terminal-themes button{display:grid;gap:4px;min-width:78px;padding:4px;border:1px solid var(--border-strong);border-radius:4px;color:var(--text-primary);font-size:11px}
.terminal-themes button[aria-pressed='true']{border-color:var(--brand-primary);box-shadow:0 0 0 1px var(--brand-primary)}
.terminal-theme-preview{display:block;padding:5px 8px;border:1px solid var(--border-strong);border-radius:3px;text-align:left;line-height:1.2;font-family:monospace}
.terminal-blink{margin-top:10px}
[data-settings-anchor='terminal'] .settings-section{margin-bottom:20px}
[data-settings-anchor='terminal'] .setting-group{margin-bottom:14px}
[data-settings-anchor='terminal'] h4{margin-bottom:8px}
h3{margin:0 0 20px;padding-bottom:8px;border-bottom:1px solid var(--border);font-size:16px}
h4,label{color:var(--text-primary)}
label{display:block;margin-bottom:6px;font-size:13px}
select{width:200px;padding:5px 10px;border:1px solid var(--border-strong);border-radius:4px;background:var(--bg-sidebar);color:var(--text-primary)}
option{background:var(--bg-sidebar);color:var(--text-primary)}
.setting-help{margin:6px 0;color:var(--text-secondary);font-size:12px;line-height:1.5}
.setting-checkbox{display:flex;align-items:center;gap:8px}
input{accent-color:var(--brand-primary)}
.secondary-button{padding:5px 10px;border:1px solid var(--border-strong);border-radius:4px;background:var(--bg-input);color:var(--text-primary);font-size:12px}
button:hover{background:var(--bg-hover)}
select:focus-visible,.secondary-button:focus-visible{outline:2px solid var(--brand-primary)}
</style>
