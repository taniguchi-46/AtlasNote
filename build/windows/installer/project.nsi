Unicode true

####
## Atlas Note の Wails NSIS プロジェクト設定です。
## wails_tools.nsh は wails build --nsis で生成されるため、直接編集しません。
####
!include "wails_tools.nsh"

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

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
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
FunctionEnd

Section "インストール"
    !insertmacro wails.setShellContext
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    !insertmacro wails.writeUninstaller
SectionEnd

Var UninstallCleanupFailed

!macro DeleteAndTrack path
    ClearErrors
    Delete "${path}"
    IfErrors 0 +2
        StrCpy $UninstallCleanupFailed 1
!macroend

!macro RemoveDirectoryAndTrack path
    ClearErrors
    RMDir "${path}"
    IfErrors 0 +2
        StrCpy $UninstallCleanupFailed 1
!macroend

Section "uninstall"
    !insertmacro wails.setShellContext
    StrCpy $UninstallCleanupFailed 0

    !insertmacro DeleteAndTrack "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    !insertmacro DeleteAndTrack "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    # 実行中のアンインストーラーを削除対象の外から動かす。
    SetOutPath "$TEMP"
    !insertmacro DeleteAndTrack "$INSTDIR\${PRODUCT_EXECUTABLE}"
    !insertmacro DeleteAndTrack "$INSTDIR\uninstall.exe"
    !insertmacro RemoveDirectoryAndTrack "$INSTDIR"
    !insertmacro RemoveDirectoryAndTrack "$PROGRAMFILES64\${INFO_COMPANYNAME}"

    SetRegView 64
    ClearErrors
    DeleteRegKey HKLM "${UNINST_KEY}"
    IfErrors 0 +2
        StrCpy $UninstallCleanupFailed 1

    ${If} $UninstallCleanupFailed != 0
        SetErrorLevel 1
        MessageBox MB_ICONEXCLAMATION|MB_OK "一部のアプリファイルまたは空のフォルダを削除できませんでした。Atlas Noteを終了してから再度実行してください。"
    ${EndIf}
SectionEnd
