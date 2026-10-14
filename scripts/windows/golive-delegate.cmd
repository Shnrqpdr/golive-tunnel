@echo off
rem Shortcut for golive-delegate.ps1: Windows refuses a .ps1 typed directly, so this runs the script
rem with the policy relaxed for this run only, without touching the computer's configuration.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0golive-delegate.ps1" %*
