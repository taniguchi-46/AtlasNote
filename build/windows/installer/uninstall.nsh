; Shared uninstall flow used by the production installer and the regression
; harness. The caller supplies every path so the harness cannot fall back to
; the real Desktop, Program Files, or HKLM locations.

Var AtlasNoteUninstallFailure
Var AtlasNoteUninstallRecoveryFailed
Var AtlasNoteUninstallBackupReady
Var AtlasNoteUninstallRegistryExists
Var AtlasNoteUninstallRegistryProbeFailed
Var AtlasNoteUninstallRegistryPublisher
Var AtlasNoteUninstallRegistryDisplayName
Var AtlasNoteUninstallRegistryDisplayVersion
Var AtlasNoteUninstallRegistryDisplayIcon
Var AtlasNoteUninstallRegistryUninstallString
Var AtlasNoteUninstallRegistryQuietUninstallString
Var AtlasNoteUninstallRegistryEstimatedSize
Var AtlasNoteUninstallRegistryPublisherPresent
Var AtlasNoteUninstallRegistryDisplayNamePresent
Var AtlasNoteUninstallRegistryDisplayVersionPresent
Var AtlasNoteUninstallRegistryDisplayIconPresent
Var AtlasNoteUninstallRegistryUninstallStringPresent
Var AtlasNoteUninstallRegistryQuietUninstallStringPresent
Var AtlasNoteUninstallRegistryEstimatedSizePresent

!macro AtlasNoteUninstall productPath uninstallPath startShortcutPath desktopShortcutPath registryRoot registryRootHandle registryKey installDirPath defaultInstallPath companyParentPath unassociateFunction
    StrCpy $AtlasNoteUninstallFailure 0
    StrCpy $AtlasNoteUninstallRecoveryFailed 0
    StrCpy $AtlasNoteUninstallBackupReady 0
    StrCpy $AtlasNoteUninstallRegistryExists 0
    StrCpy $AtlasNoteUninstallRegistryProbeFailed 0

    ; The application binary is the first destructive operation. A locked
    ; binary stops the uninstall before any recovery-sensitive item changes.
    IfFileExists "${productPath}" atlasnote_uninstall_main_present atlasnote_uninstall_main_deleted
    atlasnote_uninstall_main_present:
        ClearErrors
        Delete "${productPath}"
        IfErrors atlasnote_uninstall_failure atlasnote_uninstall_main_deleted
    atlasnote_uninstall_main_deleted:

    ClearErrors
    Delete "${startShortcutPath}"
    IfErrors atlasnote_uninstall_failure atlasnote_uninstall_start_shortcut_deleted
    atlasnote_uninstall_start_shortcut_deleted:
    ClearErrors
    Delete "${desktopShortcutPath}"
    IfErrors atlasnote_uninstall_failure atlasnote_uninstall_desktop_shortcut_deleted
    atlasnote_uninstall_desktop_shortcut_deleted:

    ; Association/protocol cleanup is supplied by the caller. The harness
    ; supplies a no-op function, while production calls the generated Wails
    ; wrappers.
    ClearErrors
    Call ${unassociateFunction}
    IfErrors atlasnote_uninstall_failure atlasnote_unassociate_done
    atlasnote_unassociate_done:

    ; Move the installer's working directory out of INSTDIR before handling
    ; the running uninstaller. The only backup name is the original filename
    ; in NSIS's private plugin directory; no product-directory alias is made.
    SetOutPath "$TEMP"
    InitPluginsDir
    ClearErrors
    CopyFiles /SILENT "${uninstallPath}" "$PLUGINSDIR"
    IfErrors atlasnote_uninstall_failure atlasnote_uninstaller_backed_up
    atlasnote_uninstaller_backed_up:
        StrCpy $AtlasNoteUninstallBackupReady 1

    ClearErrors
    Delete "${uninstallPath}"
    IfErrors atlasnote_uninstall_failure atlasnote_uninstaller_deleted
    atlasnote_uninstaller_deleted:

    ; Take a 64-bit-view registry snapshot before attempting deletion. The
    ; native probe distinguishes ERROR_FILE_NOT_FOUND from access denial and
    ; does not use an empty ReadRegStr as an existence test.
    SetRegView 64
    System::Call 'advapi32::RegOpenKeyEx(i ${registryRootHandle}, t "${registryKey}", i 0, i 0x20119, *i .r0) i.r1'
    ${If} $1 == 0
        StrCpy $AtlasNoteUninstallRegistryExists 1
        System::Call 'advapi32::RegCloseKey(i r0)'
    ${ElseIf} $1 == 2
        StrCpy $AtlasNoteUninstallRegistryExists 0
    ${Else}
        StrCpy $AtlasNoteUninstallRegistryExists 1
        StrCpy $AtlasNoteUninstallRegistryProbeFailed 1
    ${EndIf}

    ${If} $AtlasNoteUninstallRegistryExists == 1
        ; Preserve the values written by wails.writeUninstaller. Missing
        ; optional values are recorded as absent; empty values are preserved.
        ClearErrors
        ReadRegStr $AtlasNoteUninstallRegistryPublisher ${registryRoot} "${registryKey}" "Publisher"
        IfErrors 0 atlasnote_uninstall_publisher_read
        Goto atlasnote_uninstall_publisher_done
        atlasnote_uninstall_publisher_read:
            StrCpy $AtlasNoteUninstallRegistryPublisherPresent 1
        atlasnote_uninstall_publisher_done:

        ClearErrors
        ReadRegStr $AtlasNoteUninstallRegistryDisplayName ${registryRoot} "${registryKey}" "DisplayName"
        IfErrors 0 atlasnote_uninstall_display_name_read
        Goto atlasnote_uninstall_display_name_done
        atlasnote_uninstall_display_name_read:
            StrCpy $AtlasNoteUninstallRegistryDisplayNamePresent 1
        atlasnote_uninstall_display_name_done:

        ClearErrors
        ReadRegStr $AtlasNoteUninstallRegistryDisplayVersion ${registryRoot} "${registryKey}" "DisplayVersion"
        IfErrors 0 atlasnote_uninstall_display_version_read
        Goto atlasnote_uninstall_display_version_done
        atlasnote_uninstall_display_version_read:
            StrCpy $AtlasNoteUninstallRegistryDisplayVersionPresent 1
        atlasnote_uninstall_display_version_done:

        ClearErrors
        ReadRegStr $AtlasNoteUninstallRegistryDisplayIcon ${registryRoot} "${registryKey}" "DisplayIcon"
        IfErrors 0 atlasnote_uninstall_display_icon_read
        Goto atlasnote_uninstall_display_icon_done
        atlasnote_uninstall_display_icon_read:
            StrCpy $AtlasNoteUninstallRegistryDisplayIconPresent 1
        atlasnote_uninstall_display_icon_done:

        ClearErrors
        ReadRegStr $AtlasNoteUninstallRegistryUninstallString ${registryRoot} "${registryKey}" "UninstallString"
        IfErrors 0 atlasnote_uninstall_uninstall_string_read
        Goto atlasnote_uninstall_uninstall_string_done
        atlasnote_uninstall_uninstall_string_read:
            StrCpy $AtlasNoteUninstallRegistryUninstallStringPresent 1
        atlasnote_uninstall_uninstall_string_done:

        ClearErrors
        ReadRegStr $AtlasNoteUninstallRegistryQuietUninstallString ${registryRoot} "${registryKey}" "QuietUninstallString"
        IfErrors 0 atlasnote_uninstall_quiet_uninstall_string_read
        Goto atlasnote_uninstall_quiet_uninstall_string_done
        atlasnote_uninstall_quiet_uninstall_string_read:
            StrCpy $AtlasNoteUninstallRegistryQuietUninstallStringPresent 1
        atlasnote_uninstall_quiet_uninstall_string_done:

        ClearErrors
        ReadRegDWORD $AtlasNoteUninstallRegistryEstimatedSize ${registryRoot} "${registryKey}" "EstimatedSize"
        IfErrors 0 atlasnote_uninstall_estimated_size_read
        Goto atlasnote_uninstall_estimated_size_done
        atlasnote_uninstall_estimated_size_read:
            StrCpy $AtlasNoteUninstallRegistryEstimatedSizePresent 1
        atlasnote_uninstall_estimated_size_done:
    ${EndIf}

    ${If} $AtlasNoteUninstallRegistryProbeFailed == 1
        Goto atlasnote_uninstall_failure
    ${EndIf}
    ${If} $AtlasNoteUninstallRegistryExists == 1
        ClearErrors
        DeleteRegKey ${registryRoot} "${registryKey}"
        IfErrors atlasnote_uninstall_failure atlasnote_uninstall_registry_deleted

        ; DeleteRegKey's error flag is checked above, then the same 64-bit
        ; view is queried again so a partial/no-op delete is never reported as
        ; success.
        atlasnote_uninstall_registry_deleted:
        System::Call 'advapi32::RegOpenKeyEx(i ${registryRootHandle}, t "${registryKey}", i 0, i 0x20119, *i .r0) i.r1'
        ${If} $1 == 2
            Goto atlasnote_uninstall_registry_confirmed
        ${Else}
            Goto atlasnote_uninstall_failure
        ${EndIf}
    ${EndIf}
    atlasnote_uninstall_registry_confirmed:

    ; Product files and registration are complete. Folder cleanup is best
    ; effort only: user files and child folders must remain and must not turn
    ; an otherwise successful uninstall into a permanent retry loop.
    ClearErrors
    RMDir "${installDirPath}"
    StrCmp "${installDirPath}" "${defaultInstallPath}" atlasnote_uninstall_default_path atlasnote_uninstall_finished
    atlasnote_uninstall_default_path:
        ClearErrors
        RMDir "${companyParentPath}"
    atlasnote_uninstall_finished:
        SetErrorLevel 0
        Goto atlasnote_uninstall_done

    atlasnote_uninstall_failure:
        StrCpy $AtlasNoteUninstallFailure 1

        ; If the original uninstaller is still present (for example, because
        ; it was locked), leave it alone. Restore it under its original name
        ; only after a successful private backup and only when it is absent.
        IfFileExists "${uninstallPath}" atlasnote_uninstall_restore_done atlasnote_uninstall_restore_required
        atlasnote_uninstall_restore_required:
            ${If} $AtlasNoteUninstallBackupReady != 1
                StrCpy $AtlasNoteUninstallRecoveryFailed 1
                Goto atlasnote_uninstall_restore_registry
            ${EndIf}
            CreateDirectory "${installDirPath}"
            ClearErrors
            CopyFiles /SILENT "$PLUGINSDIR\uninstall.exe" "${installDirPath}"
            IfErrors atlasnote_uninstall_restore_failed atlasnote_uninstall_restore_copied
            atlasnote_uninstall_restore_copied:
                IfFileExists "${uninstallPath}" atlasnote_uninstall_restore_done atlasnote_uninstall_restore_failed
            atlasnote_uninstall_restore_failed:
                StrCpy $AtlasNoteUninstallRecoveryFailed 1
        atlasnote_uninstall_restore_done:

        atlasnote_uninstall_restore_registry:
        ; A failed delete normally leaves the key intact. If it disappeared,
        ; restore the saved values so the user has a working retry entry.
        ${If} $AtlasNoteUninstallRegistryExists == 1
            SetRegView 64
            System::Call 'advapi32::RegOpenKeyEx(i ${registryRootHandle}, t "${registryKey}", i 0, i 0x20119, *i .r0) i.r1'
            ${If} $1 == 2
                ClearErrors
                ${If} $AtlasNoteUninstallRegistryPublisherPresent == 1
                    WriteRegStr ${registryRoot} "${registryKey}" "Publisher" "$AtlasNoteUninstallRegistryPublisher"
                    IfErrors atlasnote_uninstall_registry_restore_failed
                ${EndIf}
                ${If} $AtlasNoteUninstallRegistryDisplayNamePresent == 1
                    WriteRegStr ${registryRoot} "${registryKey}" "DisplayName" "$AtlasNoteUninstallRegistryDisplayName"
                    IfErrors atlasnote_uninstall_registry_restore_failed
                ${EndIf}
                ${If} $AtlasNoteUninstallRegistryDisplayVersionPresent == 1
                    WriteRegStr ${registryRoot} "${registryKey}" "DisplayVersion" "$AtlasNoteUninstallRegistryDisplayVersion"
                    IfErrors atlasnote_uninstall_registry_restore_failed
                ${EndIf}
                ${If} $AtlasNoteUninstallRegistryDisplayIconPresent == 1
                    WriteRegStr ${registryRoot} "${registryKey}" "DisplayIcon" "$AtlasNoteUninstallRegistryDisplayIcon"
                    IfErrors atlasnote_uninstall_registry_restore_failed
                ${EndIf}
                ${If} $AtlasNoteUninstallRegistryUninstallStringPresent == 1
                    WriteRegStr ${registryRoot} "${registryKey}" "UninstallString" "$AtlasNoteUninstallRegistryUninstallString"
                    IfErrors atlasnote_uninstall_registry_restore_failed
                ${EndIf}
                ${If} $AtlasNoteUninstallRegistryQuietUninstallStringPresent == 1
                    WriteRegStr ${registryRoot} "${registryKey}" "QuietUninstallString" "$AtlasNoteUninstallRegistryQuietUninstallString"
                    IfErrors atlasnote_uninstall_registry_restore_failed
                ${EndIf}
                ${If} $AtlasNoteUninstallRegistryEstimatedSizePresent == 1
                    WriteRegDWORD ${registryRoot} "${registryKey}" "EstimatedSize" "$AtlasNoteUninstallRegistryEstimatedSize"
                    IfErrors atlasnote_uninstall_registry_restore_failed
                ${EndIf}
                Goto atlasnote_uninstall_registry_restore_done
            ${ElseIf} $1 != 0
                ; The key still exists or cannot be inspected. Do not claim
                ; that registration recovery succeeded.
                StrCpy $AtlasNoteUninstallRecoveryFailed 1
            ${EndIf}
        ${EndIf}
        Goto atlasnote_uninstall_registry_restore_done

        atlasnote_uninstall_registry_restore_failed:
            StrCpy $AtlasNoteUninstallRecoveryFailed 1
        atlasnote_uninstall_registry_restore_done:
        SetErrorLevel 1
        IfSilent atlasnote_uninstall_failure_silent atlasnote_uninstall_failure_interactive
        atlasnote_uninstall_failure_interactive:
            ${If} $AtlasNoteUninstallRecoveryFailed == 1
                MessageBox MB_ICONEXCLAMATION|MB_OK "アンインストールに失敗しました。再実行に必要なファイルまたは登録情報を復元できませんでした。再インストールしてから、もう一度アンインストールしてください。"
            ${Else}
                MessageBox MB_ICONEXCLAMATION|MB_OK "一部のアプリファイルまたは登録を削除できませんでした。Atlas Noteを終了してから再度実行してください。"
            ${EndIf}
        atlasnote_uninstall_failure_silent:
            Quit
    atlasnote_uninstall_done:
!macroend
