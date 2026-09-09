Unicode true
RequestExecutionLevel user
!include "LogicLib.nsh"
!include "FileFunc.nsh"
Name "Atlas Note maintenance fixture"
OutFile "${FIXTURE_OUTPUT}"
SilentInstall silent
Section
    ReadEnvStr $0 "ATLAS_NOTE_CLEANUP_FIXTURE"
    StrCmp $0 "" fail
    ${GetParameters} $1
    FileOpen $2 "$0\arguments.txt" w
    FileWrite $2 "$1"
    FileClose $2
    ReadEnvStr $3 "ATLAS_NOTE_CLEANUP_FIXTURE_EXIT"
    SetErrorLevel $3
    Quit
    fail:
    SetErrorLevel 2
SectionEnd
