@echo off
:menu
cls
echo ========================================
echo       DongWai 账号管理傻瓜工具
echo ========================================
echo  1. 查看所有账号列表
echo  2. 添加新账号 (管理员/编辑)
echo  3. 修改用户密码
echo  4. 删除指定账号
echo  5. 退出
echo ========================================
set /p choice=请选择操作 (1-5): 

if "%choice%"=="1" goto list
if "%choice%"=="2" goto add
if "%choice%"=="3" goto pwd
if "%choice%"=="4" goto del
if "%choice%"=="5" exit
goto menu

:list
cls
echo [当前账号列表]
.\user-cli.exe list
pause
goto menu

:add
cls
set /p uname=请输入用户名: 
set /p upass=请输入密码: 
set /p urole=请输入角色 (admin/editor, 直接回车默认为admin): 
if "%urole%"=="" set urole=admin
.\user-cli.exe add -u %uname% -p %upass% -r %urole%
pause
goto menu

:pwd
cls
set /p uname=请输入要修改的用户名: 
set /p upass=请输入新密码: 
.\user-cli.exe pwd -u %uname% -p %upass%
pause
goto menu

:del
cls
set /p uname=请输入要删除的用户名: 
echo 警告：删除操作不可恢复！
set /p confirm=确定要删除 %uname% 吗？(y/n): 
if /i "%confirm%"=="y" .\user-cli.exe del -u %uname%
pause
goto menu