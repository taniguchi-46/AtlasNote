Unicode true

####
## Atlas Note の Wails NSIS プロジェクト設定です。
## wails_tools.nsh は wails build --nsis で生成されるため、直接編集しません。
####
!include "wails_tools.nsh"
!include "uninstall.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} インストーラー"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI.nsh"
!include "nsDialogs.nsh"
!include "WinMessages.nsh"
!include "installer-options.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Atlas Noteを起動"
!define MUI_FINISHPAGE_RUN_NOTCHECKED
!define MUI_FINISHPAGE_RUN_FUNCTION AtlasNoteLaunchFromFinish

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
Page custom AtlasNoteOptionsPageCreate AtlasNoteOptionsPageLeave
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "Japanese"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
ShowInstDetails show

Function .onInit
    !insertmacro wails.checkArchitecture
    Call AtlasNoteInitializeOptions
FunctionEnd

Section "インストール"
    !insertmacro wails.setShellContext
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files

    !insertmacro AtlasNoteCreateOptionalShortcuts "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    !insertmacro wails.writeUninstaller
SectionEnd

Function un.AtlasNoteUnassociate
    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols
FunctionEnd

Section "uninstall"
    !insertmacro wails.setShellContext
    !insertmacro AtlasNoteUninstall "$INSTDIR\${PRODUCT_EXECUTABLE}" "$INSTDIR\uninstall.exe" "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$DESKTOP\${INFO_PRODUCTNAME}.lnk" HKLM 0x80000002 "${UNINST_KEY}" "$INSTDIR" "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" "$PROGRAMFILES64\${INFO_COMPANYNAME}" un.AtlasNoteUnassociate
SectionEnd
