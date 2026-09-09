Unicode true
RequestExecutionLevel user

!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "nsDialogs.nsh"
!include "WinMessages.nsh"
!include "..\uninstall.nsh"

Name "Atlas Note uninstall harness"
OutFile "..\uninstall-harness.exe"
InstallDir "$TEMP\AtlasNote-uninstall-harness"
ShowInstDetails nevershow
SilentInstall silent
SilentUninstall silent

UninstPage custom un.AtlasNoteUninstallOptionsPageCreate un.AtlasNoteUninstallOptionsPageLeave

Var AtlasNoteHarnessRoot
Var AtlasNoteHarnessInstallDir
Var AtlasNoteHarnessStartShortcut
Var AtlasNoteHarnessDesktopShortcut
Var AtlasNoteHarnessRegistryKey
Var AtlasNoteHarnessDefaultInstallDir
Var AtlasNoteHarnessCompanyParent

!macro AtlasNoteHarnessReadEnvironment
    ReadEnvStr $AtlasNoteHarnessRoot "ATLAS_NOTE_UNINSTALL_FIXTURE"
    ReadEnvStr $AtlasNoteHarnessRegistryKey "ATLAS_NOTE_UNINSTALL_REGISTRY_KEY"
    ReadEnvStr $AtlasNoteHarnessInstallDir "ATLAS_NOTE_UNINSTALL_INSTALL_DIR"
    ReadEnvStr $AtlasNoteHarnessStartShortcut "ATLAS_NOTE_UNINSTALL_START_SHORTCUT"
    ReadEnvStr $AtlasNoteHarnessDesktopShortcut "ATLAS_NOTE_UNINSTALL_DESKTOP_SHORTCUT"
    ReadEnvStr $AtlasNoteHarnessDefaultInstallDir "ATLAS_NOTE_UNINSTALL_DEFAULT_INSTALL_DIR"
    ReadEnvStr $AtlasNoteHarnessCompanyParent "ATLAS_NOTE_UNINSTALL_COMPANY_PARENT"
    ${If} $AtlasNoteHarnessRoot == ""
        SetErrorLevel 2
        Abort
    ${EndIf}
    ${If} $AtlasNoteHarnessRegistryKey == ""
        SetErrorLevel 2
        Abort
    ${EndIf}
    ${If} $AtlasNoteHarnessInstallDir == ""
        StrCpy $AtlasNoteHarnessInstallDir "$AtlasNoteHarnessRoot\install"
    ${EndIf}
    ${If} $AtlasNoteHarnessStartShortcut == ""
        StrCpy $AtlasNoteHarnessStartShortcut "$AtlasNoteHarnessRoot\shortcuts\start.lnk"
    ${EndIf}
    ${If} $AtlasNoteHarnessDesktopShortcut == ""
        StrCpy $AtlasNoteHarnessDesktopShortcut "$AtlasNoteHarnessRoot\shortcuts\desktop.lnk"
    ${EndIf}
    ${If} $AtlasNoteHarnessDefaultInstallDir == ""
        StrCpy $AtlasNoteHarnessDefaultInstallDir "$AtlasNoteHarnessRoot\install"
    ${EndIf}
    ${If} $AtlasNoteHarnessCompanyParent == ""
        StrCpy $AtlasNoteHarnessCompanyParent "$AtlasNoteHarnessRoot\company-parent"
    ${EndIf}
!macroend

Function un.AtlasNoteHarnessUnassociate
    ClearErrors
FunctionEnd

Function .onInit
    !insertmacro AtlasNoteHarnessReadEnvironment
FunctionEnd

Function un.onInit
    !insertmacro AtlasNoteHarnessReadEnvironment
FunctionEnd

Section "fixture"
    CreateDirectory "$AtlasNoteHarnessInstallDir"
    WriteUninstaller "$AtlasNoteHarnessInstallDir\uninstall.exe"
SectionEnd

Section "uninstall"
    !insertmacro AtlasNoteUninstall "$AtlasNoteHarnessInstallDir\AtlasNote.exe" "$AtlasNoteHarnessInstallDir\uninstall.exe" "$AtlasNoteHarnessStartShortcut" "$AtlasNoteHarnessDesktopShortcut" HKCU 0x80000001 "$AtlasNoteHarnessRegistryKey" "$AtlasNoteHarnessInstallDir" "$AtlasNoteHarnessDefaultInstallDir" "$AtlasNoteHarnessCompanyParent" un.AtlasNoteHarnessUnassociate
SectionEnd
