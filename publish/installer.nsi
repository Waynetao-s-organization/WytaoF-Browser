; WytaoF Browser NSIS Installer Script
; Usage: makensis /DVERSION=1.2.0 /DSTAGINGDIR=C:\path\to\staging installer.nsi

Unicode True

!ifndef VERSION
  !define VERSION "1.2.0"
!endif
!ifndef STAGINGDIR
  !define STAGINGDIR "..\publish\staging"
!endif
!ifndef OUTFILE
  !define OUTFILE "..\publish\output\WytaoFBrowser-Setup-${VERSION}.exe"
!endif
!ifndef REG_ROOT
  !define REG_ROOT HKLM
!endif
!ifndef UNINSTALL_KEY
  !define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\WytaoFBrowser"
!endif
!ifndef SHORTCUT_DIR
  !define SHORTCUT_DIR "WytaoF Browser"
!endif
!ifndef EXECUTION_LEVEL
  !define EXECUTION_LEVEL admin
!endif

!define PRODUCT_NAME    "WytaoF Browser"
!define PRODUCT_EXE     "wytaof-browser.exe"
!define INSTALL_DIR     "$PROGRAMFILES64\WytaoF Browser"
!define POWERSHELL_EXE  "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"

!include "MUI2.nsh"
!include "LogicLib.nsh"

!ifdef BESTCOMPRESSION
  SetCompressor /SOLID lzma
!else
  SetCompressor lzma
!endif

!macro WriteCloseProcessScript HANDLE
  FileWrite ${HANDLE} "param([string]$$InstallDir, [string]$$ExcludePath, [string]$$Mode = 'close')$\r$\n"
  FileWrite ${HANDLE} "$$ErrorActionPreference = 'SilentlyContinue'$\r$\n"
  FileWrite ${HANDLE} "if ([string]::IsNullOrWhiteSpace($$InstallDir) -or -not (Test-Path -LiteralPath $$InstallDir)) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$root = [System.IO.Path]::GetFullPath($$InstallDir).TrimEnd('\') + '\'$\r$\n"
  FileWrite ${HANDLE} "$$exclude = ''$\r$\n"
  FileWrite ${HANDLE} "if (-not [string]::IsNullOrWhiteSpace($$ExcludePath)) { $$exclude = [System.IO.Path]::GetFullPath($$ExcludePath) }$\r$\n"
  FileWrite ${HANDLE} "function Get-WytaoFBrowserProcesses {$\r$\n"
  FileWrite ${HANDLE} "  @(Get-CimInstance Win32_Process | Where-Object {$\r$\n"
  FileWrite ${HANDLE} "    $$_.ExecutablePath -and $$_.ExecutablePath.StartsWith($$root, [System.StringComparison]::OrdinalIgnoreCase) -and ($$exclude -eq '' -or -not $$_.ExecutablePath.Equals($$exclude, [System.StringComparison]::OrdinalIgnoreCase))$\r$\n"
  FileWrite ${HANDLE} "  })$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "function Get-WytaoFBrowserInstances {$\r$\n"
  FileWrite ${HANDLE} "  @(Get-WytaoFBrowserProcesses | Where-Object {$\r$\n"
  FileWrite ${HANDLE} "    $$_.ExecutablePath.StartsWith(($$root + 'chrome\'), [System.StringComparison]::OrdinalIgnoreCase) -and $$_.CommandLine -notmatch '--type='$\r$\n"
  FileWrite ${HANDLE} "  })$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "if ($$Mode -eq 'count') {$\r$\n"
  FileWrite ${HANDLE} "  $$count = (Get-WytaoFBrowserInstances).Count$\r$\n"
  FileWrite ${HANDLE} "  if ($$count -gt 200) { $$count = 200 }$\r$\n"
  FileWrite ${HANDLE} "  exit $$count$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "$$graceDeadline = (Get-Date).AddSeconds(25)$\r$\n"
  FileWrite ${HANDLE} "while ((Get-Date) -lt $$graceDeadline) {$\r$\n"
  FileWrite ${HANDLE} "  $$instances = Get-WytaoFBrowserInstances$\r$\n"
  FileWrite ${HANDLE} "  if (-not $$instances -or $$instances.Count -eq 0) { break }$\r$\n"
  FileWrite ${HANDLE} "  foreach ($$item in $$instances) { try { [void](Get-Process -Id $$item.ProcessId -ErrorAction Stop).CloseMainWindow() } catch {} }$\r$\n"
  FileWrite ${HANDLE} "  Start-Sleep -Milliseconds 700$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "$$deadline = (Get-Date).AddSeconds(10)$\r$\n"
  FileWrite ${HANDLE} "do {$\r$\n"
  FileWrite ${HANDLE} "  $$procs = Get-WytaoFBrowserProcesses$\r$\n"
  FileWrite ${HANDLE} "  if (-not $$procs -or $$procs.Count -eq 0) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "  foreach ($$p in $$procs) { try { Stop-Process -Id $$p.ProcessId -Force -ErrorAction Stop } catch {} }$\r$\n"
  FileWrite ${HANDLE} "  Start-Sleep -Milliseconds 400$\r$\n"
  FileWrite ${HANDLE} "} while ((Get-Date) -lt $$deadline)$\r$\n"
  FileWrite ${HANDLE} "$$left = Get-WytaoFBrowserProcesses$\r$\n"
  FileWrite ${HANDLE} "if ($$left -and $$left.Count -gt 0) {$\r$\n"
  FileWrite ${HANDLE} "  $$names = ($$left | ForEach-Object { $$_.Name + '#' + $$_.ProcessId }) -join ', '$\r$\n"
  FileWrite ${HANDLE} "  Write-Host ('still running: ' + $$names)$\r$\n"
  FileWrite ${HANDLE} "  exit 1$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "exit 0$\r$\n"
!macroend

Function ConfirmRunningInstances
  IfFileExists "$INSTDIR\${PRODUCT_EXE}" 0 done
  IfFileExists "${POWERSHELL_EXE}" 0 done

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  nsExec::Exec '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "" -Mode count'
  Pop $2
  Delete $0

  IntCmp $2 0 done done 0
  MessageBox MB_OKCANCEL|MB_ICONINFORMATION "检测到有 $2 个浏览器实例正在运行。$\r$\n$\r$\n继续安装会先把它们正常关闭（和点窗口右上角的关闭一样），登录状态和 Cookie 会照常保存。网页里还没提交的内容请先处理好。$\r$\n$\r$\n点击“确定”继续安装，点击“取消”终止本次安装。" /SD IDOK IDOK done
  Abort "安装已取消：还有浏览器实例在运行。"

done:
FunctionEnd

Function CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "正在关闭旧版本和正在运行的浏览器实例: $INSTDIR"
  nsExec::Exec '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath ""'
  Pop $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到旧版本仍有进程占用安装目录。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“重试”将再次尝试关闭残留进程，点击“取消”将终止本次安装。" /SD IDCANCEL IDRETRY retry_powershell IDCANCEL install_abort

install_abort:
  Abort "安装已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  DetailPrint "PowerShell 不可用，回退到 taskkill 清理主进程和代理进程..."
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM ${PRODUCT_EXE}' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM xray.exe' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM sing-box.exe' $2
  Sleep 1500

done:
FunctionEnd

Function un.CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "正在关闭安装目录中的残留进程: $INSTDIR"
  nsExec::Exec '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "$INSTDIR\Uninstall.exe"'
  Pop $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到安装目录中仍有旧进程占用文件。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“重试”将再次尝试关闭残留进程，点击“取消”将终止本次卸载。" IDRETRY retry_powershell IDCANCEL uninstall_abort

uninstall_abort:
  Abort "卸载已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  DetailPrint "PowerShell 不可用，回退到 taskkill 清理主进程和代理进程..."
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM ${PRODUCT_EXE}' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM xray.exe' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM sing-box.exe' $2
  Sleep 1500

done:
FunctionEnd

Function WarnInstallDir
  ReadRegStr $3 ${REG_ROOT} "${UNINSTALL_KEY}" "InstallLocation"
  ${If} "$3" != ""
  ${AndIf} "$3" != "$INSTDIR"
  ${AndIf} ${FileExists} "$3\${PRODUCT_EXE}"
    MessageBox MB_ICONEXCLAMATION|MB_YESNO|MB_DEFBUTTON2 "这台电脑上已经装过 WytaoF Browser，位置是：$\r$\n$3$\r$\n$\r$\n现在选的是另一个目录。装到新目录相当于全新安装，原来的实例、登录状态和配置不会带过去。$\r$\n要升级并保留数据，请点“否”，把安装目录改回上面那个位置。$\r$\n$\r$\n仍然要装到新目录吗？" IDYES check_protected_dir
    Abort
  ${EndIf}

check_protected_dir:
  StrCpy $0 "$INSTDIR"

  StrCpy $1 "$0" 2
  ${If} "$1" == "C:"
    Goto warn_dir
  ${EndIf}
  ${If} "$1" == "c:"
    Goto warn_dir
  ${EndIf}

  StrLen $1 "$PROGRAMFILES64"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES64"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  StrLen $1 "$PROGRAMFILES"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  StrLen $1 "$PROGRAMFILES32"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES32"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  Return

warn_dir:
  MessageBox MB_ICONEXCLAMATION|MB_OKCANCEL|MB_DEFBUTTON2 "不建议安装到 C 盘、Program Files 或其他受权限保护的目录。$\r$\n$\r$\nWytaoF Browser 会在安装目录写入 data、浏览器实例和运行时文件；普通权限运行时可能写入失败，导致再次启动闪退或浏览器实例启动失败。$\r$\n$\r$\n建议改为非 C 盘可写目录，例如 D:\software\WytaoF Browser 或 E:\software\WytaoF Browser。$\r$\n$\r$\n点击“确定”继续安装到当前目录，点击“取消”返回修改安装路径。" IDOK continue_install IDCANCEL cancel_install

continue_install:
  Return

cancel_install:
  Abort
FunctionEnd

Name "${PRODUCT_NAME} ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "${INSTALL_DIR}"
InstallDirRegKey ${REG_ROOT} "${UNINSTALL_KEY}" "InstallLocation"
RequestExecutionLevel ${EXECUTION_LEVEL}

!define MUI_ICON "..\build\windows\icon.ico"
!define MUI_UNICON "..\build\windows\icon.ico"

!define MUI_WELCOMEPAGE_TEXT "安装向导会把 WytaoF Browser ${VERSION} 装到这台电脑上。$\r$\n$\r$\n之前装过旧版本的，直接装到原来的目录就是升级：实例、登录状态、Cookie、代理和各项设置都原样保留，不用先卸载。$\r$\n$\r$\n点击“下一步”继续。"
!insertmacro MUI_PAGE_WELCOME
!define MUI_DIRECTORYPAGE_TEXT_TOP "请选择 WytaoF Browser 的安装目录。建议安装到非 C 盘的可写目录，例如 D:\software\WytaoF Browser 或 E:\software\WytaoF Browser；不要安装到 C 盘、Program Files 或其他受权限保护的目录。"
!define MUI_DIRECTORYPAGE_TEXT_DESTINATION "安装目录（建议非 C 盘）"
!define MUI_PAGE_CUSTOMFUNCTION_LEAVE WarnInstallDir
!insertmacro MUI_PAGE_DIRECTORY
!undef MUI_PAGE_CUSTOMFUNCTION_LEAVE
!undef MUI_DIRECTORYPAGE_TEXT_TOP
!undef MUI_DIRECTORYPAGE_TEXT_DESTINATION
!define MUI_COMPONENTSPAGE_SMALLDESC
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "启动 WytaoF Browser"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"

Section "WytaoF Browser (required)" SecMain
  SectionIn RO
  Call ConfirmRunningInstances
  Call CloseInstalledProcesses
  SetOutPath "$INSTDIR"
  File "${STAGINGDIR}\${PRODUCT_EXE}"
!if /FileExists "${STAGINGDIR}\config.yaml"
  IfFileExists "$INSTDIR\config.yaml" +2 0
    File "${STAGINGDIR}\config.yaml"
!else
  !echo "Warning: ${STAGINGDIR}\\config.yaml not found, installer will use runtime defaults."
!endif
!if /FileExists "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR\chrome"
  File /r "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR"
!endif
  CreateDirectory "$INSTDIR\data"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "DisplayName"     "${PRODUCT_NAME}"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "Publisher"       "蔚蓝海岸"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "DisplayIcon"     "$INSTDIR\${PRODUCT_EXE}"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "NoModify"        "1"
  WriteRegStr ${REG_ROOT} "${UNINSTALL_KEY}" "NoRepair"        "1"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateDirectory "$SMPROGRAMS\${SHORTCUT_DIR}"
  CreateShortcut "$SMPROGRAMS\${SHORTCUT_DIR}\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}"
  CreateShortcut "$SMPROGRAMS\${SHORTCUT_DIR}\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Proxy Runtime (xray / sing-box)" SecRuntime
  SectionIn RO
  SetOutPath "$INSTDIR\bin"
  File "${STAGINGDIR}\bin\xray.exe"
  File "${STAGINGDIR}\bin\sing-box.exe"
SectionEnd

Section /o "Desktop Shortcut" SecDesktop
  CreateShortcut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain}    "WytaoF Browser main program and default config (required)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRuntime} "xray and sing-box proxy tools (vless/vmess/hysteria2)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Create a shortcut on the desktop"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  Call un.CloseInstalledProcesses

  Delete /REBOOTOK "$INSTDIR\${PRODUCT_EXE}"
  Delete /REBOOTOK "$INSTDIR\config.yaml"
  Delete /REBOOTOK "$INSTDIR\proxies.yaml"
  Delete /REBOOTOK "$INSTDIR\Uninstall.exe"
  RMDir /r /REBOOTOK "$INSTDIR\bin"
  RMDir /r /REBOOTOK "$INSTDIR\chrome"
  Delete /REBOOTOK "$SMPROGRAMS\${SHORTCUT_DIR}\${PRODUCT_NAME}.lnk"
  Delete /REBOOTOK "$SMPROGRAMS\${SHORTCUT_DIR}\Uninstall.lnk"
  RMDir /REBOOTOK "$SMPROGRAMS\${SHORTCUT_DIR}"
  Delete /REBOOTOK "$DESKTOP\${PRODUCT_NAME}.lnk"
  DeleteRegKey ${REG_ROOT} "${UNINSTALL_KEY}"
  MessageBox MB_ICONQUESTION|MB_YESNO|MB_DEFBUTTON2 "是否彻底清理所有用户数据？$\r$\n$\r$\n选择“是”将删除 data 目录（含数据库/实例数据）以及安装目录残留文件。$\r$\n此操作不可恢复。" /SD IDNO IDYES un_remove_all_data IDNO un_keep_user_data

un_remove_all_data:
  RMDir /r /REBOOTOK "$INSTDIR\data"
  RMDir /r /REBOOTOK "$INSTDIR\logs"
  RMDir /r /REBOOTOK "$INSTDIR"
  Goto un_done

un_keep_user_data:
  RMDir /REBOOTOK "$INSTDIR"
  Goto un_done

un_done:
  IfFileExists "$INSTDIR\." 0 +2
    MessageBox MB_ICONEXCLAMATION|MB_OK "检测到部分文件仍被占用，已标记为重启后自动删除。"
SectionEnd
