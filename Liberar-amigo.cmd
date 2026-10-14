@echo off
rem Atalho para o Liberar-amigo.ps1: o Windows bloqueia .ps1 digitado direto, entao este roda o script
rem com a politica liberada so para esta execucao, sem mexer na configuracao do computador.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0Liberar-amigo.ps1" %*
