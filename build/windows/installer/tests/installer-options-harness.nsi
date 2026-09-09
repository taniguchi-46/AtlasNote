Unicode true
RequestExecutionLevel user

!define INFO_PRODUCTNAME "Atlas Note"
!define PRODUCT_EXECUTABLE "AtlasNote.exe"

!include "nsDialogs.nsh"
!include "WinMessages.nsh"
!define ATLAS_NOTE_OPTIONS_HARNESS
!include "..\installer-options.nsh"
!include "MUI.nsh"

!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Atlas Noteを起動"
!define MUI_FINISHPAGE_RUN_NOTCHECKED
!define MUI_FINISHPAGE_RUN_FUNCTION AtlasNoteLaunchFromFinish

!insertmacro MUI_PAGE_WELCOME
Page custom AtlasNoteOptionsPageCreate AtlasNoteOptionsPageLeave
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_LANGUAGE "Japanese"

Name "Atlas Note options harness"
OutFile "..\installer-options-harness.exe"
InstallDir "$TEMP\AtlasNote-options-harness"
ShowInstDetails nevershow
SilentInstall normal

Var AtlasNoteOptionsHarnessRoot
Var AtlasNoteOptionsHarnessInstallDir
Var AtlasNoteOptionsHarnessStartShortcut
Var AtlasNoteOptionsHarnessDesktopShortcut

Function .onInit
    Call AtlasNoteInitializeOptions
    ReadEnvStr $AtlasNoteOptionsHarnessRoot "ATLAS_NOTE_OPTIONS_FIXTURE"
    ReadEnvStr $AtlasNoteOptionsHarnessInstallDir "ATLAS_NOTE_OPTIONS_INSTALL_DIR"
    ReadEnvStr $AtlasNoteOptionsHarnessStartShortcut "ATLAS_NOTE_OPTIONS_START_SHORTCUT"
    ReadEnvStr $AtlasNoteOptionsHarnessDesktopShortcut "ATLAS_NOTE_OPTIONS_DESKTOP_SHORTCUT"
    ${If} $AtlasNoteOptionsHarnessRoot == ""
        SetErrorLevel 2
        Abort
    ${EndIf}
    ${If} $AtlasNoteOptionsHarnessInstallDir == ""
        StrCpy $AtlasNoteOptionsHarnessInstallDir "$AtlasNoteOptionsHarnessRoot\install"
    ${EndIf}
    ${If} $AtlasNoteOptionsHarnessStartShortcut == ""
        StrCpy $AtlasNoteOptionsHarnessStartShortcut "$AtlasNoteOptionsHarnessRoot\shortcuts\start.lnk"
    ${EndIf}
    ${If} $AtlasNoteOptionsHarnessDesktopShortcut == ""
        StrCpy $AtlasNoteOptionsHarnessDesktopShortcut "$AtlasNoteOptionsHarnessRoot\shortcuts\desktop.lnk"
    ${EndIf}
    StrCpy $INSTDIR $AtlasNoteOptionsHarnessInstallDir

    ReadEnvStr $0 "ATLAS_NOTE_OPTIONS_DESKTOP"
    StrCmp $0 "0" atlasnote_options_harness_desktop_off atlasnote_options_harness_desktop_done
    atlasnote_options_harness_desktop_off:
        StrCpy $AtlasNoteDesktopShortcutEnabled 0
    atlasnote_options_harness_desktop_done:

    ReadEnvStr $0 "ATLAS_NOTE_OPTIONS_START"
    StrCmp $0 "0" atlasnote_options_harness_start_off atlasnote_options_harness_start_done
    atlasnote_options_harness_start_off:
        StrCpy $AtlasNoteStartMenuShortcutEnabled 0
    atlasnote_options_harness_start_done:
FunctionEnd

Section "fixture"
    CreateDirectory $INSTDIR
    FileOpen $0 "$INSTDIR\${PRODUCT_EXECUTABLE}" w
    FileWrite $0 "Atlas Note options fixture"
    FileClose $0
    !insertmacro AtlasNoteCreateOptionalShortcuts "$AtlasNoteOptionsHarnessStartShortcut" "$AtlasNoteOptionsHarnessDesktopShortcut"
    WriteINIStr "$AtlasNoteOptionsHarnessRoot\options.ini" "options" "desktop" "$AtlasNoteDesktopShortcutEnabled"
    WriteINIStr "$AtlasNoteOptionsHarnessRoot\options.ini" "options" "start" "$AtlasNoteStartMenuShortcutEnabled"

    ReadEnvStr $0 "ATLAS_NOTE_OPTIONS_FINISH_LAUNCH"
    StrCmp $0 "1" atlasnote_options_harness_finish_launch atlasnote_options_harness_finish_done
    atlasnote_options_harness_finish_launch:
        Call AtlasNoteLaunchFromFinish
    atlasnote_options_harness_finish_done:
SectionEnd
