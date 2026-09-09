; Optional installation choices shared by the production installer and the
; isolated harness. The harness supplies fixture paths to the macro instead
; of allowing these functions to touch the real Desktop or Start Menu.

Var AtlasNoteDesktopShortcutCheckbox
Var AtlasNoteStartMenuShortcutCheckbox
Var AtlasNoteDesktopShortcutEnabled
Var AtlasNoteStartMenuShortcutEnabled

Function AtlasNoteInitializeOptions
    ; Both optional shortcuts are enabled by default, including silent installs.
    StrCpy $AtlasNoteDesktopShortcutEnabled 1
    StrCpy $AtlasNoteStartMenuShortcutEnabled 1
FunctionEnd

Function AtlasNoteOptionsPageCreate
    nsDialogs::Create 1018
    Pop $0
    ${If} $0 == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 24u "インストールするショートカットを選択してください。どちらも初期状態では有効です。"
    Pop $0

    ${NSD_CreateCheckbox} 0 34u 100% 14u "デスクトップショートカットを作成する"
    Pop $AtlasNoteDesktopShortcutCheckbox
    ${If} $AtlasNoteDesktopShortcutEnabled == 1
        ${NSD_SetState} $AtlasNoteDesktopShortcutCheckbox ${BST_CHECKED}
    ${EndIf}

    ${NSD_CreateCheckbox} 0 56u 100% 14u "スタートメニューショートカットを作成する"
    Pop $AtlasNoteStartMenuShortcutCheckbox
    ${If} $AtlasNoteStartMenuShortcutEnabled == 1
        ${NSD_SetState} $AtlasNoteStartMenuShortcutCheckbox ${BST_CHECKED}
    ${EndIf}

    nsDialogs::Show
FunctionEnd

Function AtlasNoteOptionsPageLeave
    ${NSD_GetState} $AtlasNoteDesktopShortcutCheckbox $AtlasNoteDesktopShortcutEnabled
    ${NSD_GetState} $AtlasNoteStartMenuShortcutCheckbox $AtlasNoteStartMenuShortcutEnabled
FunctionEnd

!macro AtlasNoteCreateOptionalShortcuts startMenuShortcutPath desktopShortcutPath
    ${If} $AtlasNoteStartMenuShortcutEnabled == 1
        CreateShortcut "${startMenuShortcutPath}" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${EndIf}
    ${If} $AtlasNoteDesktopShortcutEnabled == 1
        CreateShortCut "${desktopShortcutPath}" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${EndIf}
!macroend

Function AtlasNoteLaunchFromFinish
!ifdef ATLAS_NOTE_OPTIONS_HARNESS
    ; The harness records the callback without launching a fixture process.
    ReadEnvStr $0 "ATLAS_NOTE_OPTIONS_LAUNCH_MARKER"
    ${If} $0 != ""
        ClearErrors
        FileOpen $1 $0 w
        IfErrors 0 atlasnote_options_harness_launch_written
        Goto atlasnote_options_harness_launch_done
        atlasnote_options_harness_launch_written:
            FileWrite $1 "launch"
            FileClose $1
        atlasnote_options_harness_launch_done:
    ${EndIf}
!else
    ; The elevated installer delegates to the user's existing Explorer shell
    ; so Atlas Note starts with the normal desktop user's token.
    Exec '"$WINDIR\explorer.exe" "$INSTDIR\${PRODUCT_EXECUTABLE}"'
!endif
FunctionEnd
