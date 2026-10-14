@echo off
rem Shortcut for golive-delegate.ps1: Windows refuses a .ps1 typed directly, so this runs the script
rem with the policy relaxed for this run only, without touching the computer's configuration.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0golive-delegate.ps1" %*
set code=%errorlevel%
rem Opened by double click, the window would close before anyone read the output.
echo %cmdcmdline% | find /i "%~nx0" >nul
if not errorlevel 1 (
    echo.
    pause
)
exit /b %code%
