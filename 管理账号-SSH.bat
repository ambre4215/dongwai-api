@echo off
setlocal EnableExtensions

chcp 65001 >nul

if /i not "%~1"=="--stay" (
  start "DongWai 账号管理" cmd /k ""%~f0" --stay"
  exit /b 0
)

set "DW_SSH_USERHOST=root@117.72.15.108"
set "DW_SSH_PORT=22"
set "DW_REMOTE_DIR=/opt/1panel/docker/compose/dongwai-api"
set "DW_REMOTE_BIN=./user-cli"
set "DW_SSH_KEY="

call :detectKey

call :requireCmd ssh
if errorlevel 1 goto end
call :requireCmd scp
if errorlevel 1 goto end

if "%DW_SSH_USERHOST%"=="" call :promptConn

:menu
cls
echo ========================================
echo       DongWai 账号管理（SSH 远程）
echo ========================================
echo  连接: %DW_SSH_USERHOST%   端口: %DW_SSH_PORT%
echo  目录: %DW_REMOTE_DIR%
if "%DW_SSH_KEY%"=="" (
  echo  密钥: 未配置（将提示密码或直接失败）
) else (
  echo  密钥: %DW_SSH_KEY%
)
echo ----------------------------------------
echo  1. 查看账号列表
echo  2. 新增账号
echo  3. 重置密码
echo  4. 删除账号
echo  5. 检查远端工具与环境
echo  6. 上传本地 user-cli 到服务器
echo  7. 修改连接信息
echo  8. 退出
echo ========================================
set /p choice=请选择操作 (1-8): 

if "%choice%"=="1" goto list
if "%choice%"=="2" goto add
if "%choice%"=="3" goto pwd
if "%choice%"=="4" goto del
if "%choice%"=="5" goto check
if "%choice%"=="6" goto upload
if "%choice%"=="7" goto conn
if "%choice%"=="8" goto end
goto menu

:list
cls
echo [账号列表]
call :sshExec "%DW_REMOTE_DIR%" "%DW_REMOTE_BIN% list"
call :pauseBack
goto menu

:add
cls
set "uname="
set "upass="
set "urole=admin"

set /p uname=请输入用户名: 
if "%uname%"=="" (
  echo 用户名不能为空
  call :pauseBack
  goto menu
)

set /p upass=请输入密码(建议仅英文/数字/常见符号，避免单引号 ' ): 
if "%upass%"=="" (
  echo 密码不能为空
  call :pauseBack
  goto menu
)

set /p urole=请输入角色(admin/teacher/student，直接回车默认 admin): 
if "%urole%"=="" set "urole=admin"

echo 正在创建账号...
call :sshExec "%DW_REMOTE_DIR%" "%DW_REMOTE_BIN% add -u '%uname%' -p '%upass%' -r '%urole%'"
call :pauseBack
goto menu

:pwd
cls
set "uname="
set "upass="

set /p uname=请输入要重置密码的用户名: 
if "%uname%"=="" (
  echo 用户名不能为空
  call :pauseBack
  goto menu
)

set /p upass=请输入新密码(建议仅英文/数字/常见符号，避免单引号 ' ): 
if "%upass%"=="" (
  echo 新密码不能为空
  call :pauseBack
  goto menu
)

echo 正在重置密码...
call :sshExec "%DW_REMOTE_DIR%" "%DW_REMOTE_BIN% pwd -u '%uname%' -p '%upass%'"
call :pauseBack
goto menu

:del
cls
set "uname="

set /p uname=请输入要删除的用户名: 
if "%uname%"=="" (
  echo 用户名不能为空
  call :pauseBack
  goto menu
)

set /p confirm=确认删除 %uname% ? (y/n): 
if /i not "%confirm%"=="y" (
  echo 已取消
  call :pauseBack
  goto menu
)

echo 正在删除账号...
call :sshExec "%DW_REMOTE_DIR%" "%DW_REMOTE_BIN% del -u '%uname%'"
call :pauseBack
goto menu

:check
cls
echo [检查远端工具与环境]
if exist "%DW_SSH_KEY%" (
  echo SSH 密钥: %DW_SSH_KEY%
) else (
  echo SSH 密钥: 未找到 %DW_SSH_KEY% ^(将改为密码登录提示^)
)
call :sshExec "%DW_REMOTE_DIR%" "pwd; ls -la; echo ---; test -f .env && echo .env-OK || echo .env-MISSING; echo ---; test -x %DW_REMOTE_BIN% && echo user-cli-OK || echo user-cli-MISSING"
call :pauseBack
goto menu

:upload
cls
if not exist "user-cli" (
  echo 未找到本地文件: %CD%\user-cli
  echo 请先在本机编译 Linux 版本:
  echo   set CGO_ENABLED=0 ^&^& set GOOS=linux ^&^& set GOARCH=amd64 ^&^& go build -trimpath -ldflags "-s -w" -o user-cli .\cmd\user-cli
  call :pauseBack
  goto menu
)

echo 将本地 user-cli 上传到服务器...
call :scpPut "user-cli" "%DW_REMOTE_DIR%/user-cli"
if errorlevel 1 (
  echo 上传失败，请检查网络、账号、权限、SSH 端口
  call :pauseBack
  goto menu
)

echo 设置远端可执行权限...
call :sshExec "%DW_REMOTE_DIR%" "chmod +x user-cli; echo OK"
call :pauseBack
goto menu

:conn
call :promptConn
goto menu

:promptConn
cls
echo [配置 SSH 连接]
echo 请输入 SSH 目标，格式: user@host
set /p DW_SSH_USERHOST=SSH 目标(user@host): 
if "%DW_SSH_USERHOST%"=="" goto promptConn

set /p DW_SSH_PORT=SSH 端口(默认 22): 
if "%DW_SSH_PORT%"=="" set "DW_SSH_PORT=22"

set /p DW_REMOTE_DIR=远端目录(默认 /opt/1panel/docker/compose/dongwai-api): 
if "%DW_REMOTE_DIR%"=="" set "DW_REMOTE_DIR=/opt/1panel/docker/compose/dongwai-api"

exit /b 0

:sshExec
set "_dir=%~1"
set "_cmd=%~2"
call :sshRun "%_dir%" "%_cmd%"
if errorlevel 1 (
  echo.
  echo 远程执行失败，请检查:
  echo  1) SSH 是否可连
  echo  2) 远端目录是否存在: %_dir%
  echo  3) user-cli 是否存在且可执行
)
exit /b 0

:sshRun
set "_dir=%~1"
set "_cmd=%~2"
if not "%DW_SSH_KEY%"=="" if exist "%DW_SSH_KEY%" (
  ssh -i "%DW_SSH_KEY%" -p %DW_SSH_PORT% %DW_SSH_USERHOST% "cd %_dir% && %_cmd%"
  exit /b %errorlevel%
)
ssh -p %DW_SSH_PORT% %DW_SSH_USERHOST% "cd %_dir% && %_cmd%"
exit /b %errorlevel%

:scpPut
set "_local=%~1"
set "_remote=%~2"
if not "%DW_SSH_KEY%"=="" if exist "%DW_SSH_KEY%" (
  scp -i "%DW_SSH_KEY%" -P %DW_SSH_PORT% "%_local%" "%DW_SSH_USERHOST%:%_remote%"
  exit /b %errorlevel%
)
scp -P %DW_SSH_PORT% "%_local%" "%DW_SSH_USERHOST%:%_remote%"
exit /b %errorlevel%

:detectKey
set "_scriptDir=%~dp0"
set "_candidate="

if exist "%_scriptDir%ssh_key" set "DW_SSH_KEY=%_scriptDir%ssh_key" & exit /b 0
if exist "%_scriptDir%ssh_key.pem" set "DW_SSH_KEY=%_scriptDir%ssh_key.pem" & exit /b 0
if exist "%_scriptDir%ssh_key.key" set "DW_SSH_KEY=%_scriptDir%ssh_key.key" & exit /b 0
if exist "%_scriptDir%dongwai.pem" set "DW_SSH_KEY=%_scriptDir%dongwai.pem" & exit /b 0
if exist "%_scriptDir%id_ed25519" set "DW_SSH_KEY=%_scriptDir%id_ed25519" & exit /b 0
if exist "%_scriptDir%id_rsa" set "DW_SSH_KEY=%_scriptDir%id_rsa" & exit /b 0
if exist "%USERPROFILE%\.ssh\dongwai_id_ed25519" set "DW_SSH_KEY=%USERPROFILE%\.ssh\dongwai_id_ed25519" & exit /b 0
if exist "%USERPROFILE%\.ssh\id_ed25519" set "DW_SSH_KEY=%USERPROFILE%\.ssh\id_ed25519" & exit /b 0
if exist "%USERPROFILE%\.ssh\id_rsa" set "DW_SSH_KEY=%USERPROFILE%\.ssh\id_rsa" & exit /b 0

exit /b 0

:requireCmd
where %~1 >nul 2>nul
if errorlevel 1 (
  echo.
  echo 未检测到命令: %~1
  echo 请在 Windows 可选功能里启用 OpenSSH 客户端，或安装 Git for Windows。
  echo.
  call :pauseBack
  exit /b 1
)
exit /b 0

:pauseBack
echo.
pause
exit /b 0

:end
endlocal
exit /b 0
