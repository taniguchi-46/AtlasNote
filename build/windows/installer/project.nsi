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
Var UninstallerBackupPath
Var UninstallerBackupReady

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
    StrCpy $UninstallerBackupReady 0

    # 本体を最初に削除する。ロック中なら、再実行に必要な登録と
    # uninstall.exeを一切触らず、その場で中断する。
    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" uninstallMainPresent uninstallMainDeleted
    uninstallMainPresent:
        ClearErrors
        Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
        IfErrors 0 uninstallMainDeleted
        Goto uninstallFailure
    uninstallMainDeleted:
    !insertmacro DeleteAndTrack "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    !insertmacro DeleteAndTrack "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    ClearErrors
    !insertmacro wails.unassociateFiles
    IfErrors 0 +2
        StrCpy $UninstallCleanupFailed 1
    ClearErrors
    !insertmacro wails.unassociateCustomProtocols
    IfErrors 0 +2
        StrCpy $UninstallCleanupFailed 1

    ${If} $UninstallCleanupFailed != 0
        Goto uninstallFailure
    ${EndIf}

    # インストーラー自身の作業ディレクトリを製品フォルダの外へ移す。
    SetOutPath "$TEMP"

    # 後段のフォルダ削除や登録削除に失敗しても、再実行手段を復元できる
    # よう、実行中のuninstall.exeをNSIS専用一時領域へ退避する。
    InitPluginsDir
    StrCpy $UninstallerBackupPath "$PLUGINSDIR\AtlasNote-uninstall-backup.exe"
    ClearErrors
    CopyFiles /SILENT "$INSTDIR\uninstall.exe" "$PLUGINSDIR"
    IfErrors 0 uninstallBackupCopied
    Goto uninstallFailure
    uninstallBackupCopied:
        ClearErrors
        Rename "$PLUGINSDIR\uninstall.exe" "$UninstallerBackupPath"
        IfErrors 0 uninstallBackupReady
        Goto uninstallFailure
    uninstallBackupReady:
        StrCpy $UninstallerBackupReady 1

    ClearErrors
    Delete "$INSTDIR\uninstall.exe"
    IfErrors 0 uninstallBinaryDeleted
    Goto uninstallFailure
    uninstallBinaryDeleted:

    ClearErrors
    RMDir "$INSTDIR"
    IfErrors 0 uninstallInstallDirDeleted
    Goto uninstallFailure
    uninstallInstallDirDeleted:
    ClearErrors
    RMDir "$PROGRAMFILES64\${INFO_COMPANYNAME}"
    IfErrors 0 uninstallProductDirDeleted
    Goto uninstallFailure
    uninstallProductDirDeleted:

    SetRegView 64
    ClearErrors
    DeleteRegKey HKLM "${UNINST_KEY}"
    IfErrors 0 +2
        StrCpy $UninstallCleanupFailed 1

    ${If} $UninstallCleanupFailed != 0
        Goto uninstallFailure
    ${EndIf}
    Delete "$UninstallerBackupPath"
    Goto uninstallFinished

    uninstallFailure:
        # 本体削除後の失敗では、退避したアンインストーラーだけを戻す。
        # アプリ本体やユーザーデータは復元・削除しない。
        ${If} $UninstallerBackupReady == 1
            CreateDirectory "$INSTDIR"
            ClearErrors
            CopyFiles /SILENT "$UninstallerBackupPath" "$INSTDIR"
            IfErrors 0 uninstallBackupRestoreCopied
            Goto uninstallFailureDone
        uninstallBackupRestoreCopied:
            ClearErrors
            Rename "$INSTDIR\AtlasNote-uninstall-backup.exe" "$INSTDIR\uninstall.exe"
            IfErrors 0 uninstallFailureDone
        ${EndIf}
    uninstallFailureDone:
        SetErrorLevel 1
        IfSilent uninstallFailureSilent uninstallFailureInteractive
    uninstallFailureInteractive:
        MessageBox MB_ICONEXCLAMATION|MB_OK "一部のアプリファイルまたは登録を削除できませんでした。Atlas Noteを終了してから再度実行してください。"
    uninstallFailureSilent:
        Quit

    uninstallFinished:
SectionEnd
