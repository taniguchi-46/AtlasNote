Unicode true
RequestExecutionLevel user
!include "LogicLib.nsh"
!include "nsDialogs.nsh"
!include "WinMessages.nsh"
!include "..\uninstall.nsh"
Name "Atlas Note cleanup dispatch fixture"
OutFile "${FIXTURE_OUTPUT}"
SilentInstall silent
Section
    ReadEnvStr $1 "ATLAS_NOTE_CLEANUP_FIXTURE"
    StrCmp $1 "" fail
    ReadEnvStr $AtlasNoteUninstallDeleteDisplaySettings "ATLAS_NOTE_CLEANUP_FIXTURE_DISPLAY"
    ReadEnvStr $AtlasNoteUninstallDeleteCredentials "ATLAS_NOTE_CLEANUP_FIXTURE_CREDENTIALS"
    !insertmacro AtlasNoteExecuteCleanup "$1\maintenance.exe"
    SetErrorLevel $AtlasNoteUninstallCleanupFailure
    Quit
    fail:
    SetErrorLevel 2
SectionEnd
