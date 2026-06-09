@echo off
REM ===================================================================
REM  GustaMenu Assistente de Impressao - rotina de instalacao
REM  Executada pelo setup self-extracting (IExpress) apos a extracao.
REM  Nao exige admin: instala em %LOCALAPPDATA% e autostart em HKCU.
REM ===================================================================
setlocal enableextensions

set "APPROOT=%LOCALAPPDATA%\GustaMenu\PrintAgent"
set "TARGET=%APPROOT%\app_files"

echo Instalando GustaMenu Assistente de Impressao...

REM Remove instalacao anterior (mantem settings.json, que fica em %APPROOT%)
if exist "%TARGET%" rmdir /s /q "%TARGET%"
mkdir "%TARGET%" 2>nul

REM Extrai o payload (runtime Python + app), preservando as subpastas
extrac32 /Y /E "%~dp0payload.cab" /L "%TARGET%" >nul
if errorlevel 1 (
  echo ERRO: falha ao extrair os arquivos.
  exit /b 1
)

set "PYW=%TARGET%\runtime\pythonw.exe"
set "MAIN=%TARGET%\app\main.py"

if not exist "%PYW%" (
  echo ERRO: runtime nao encontrado em "%PYW%".
  exit /b 1
)

REM Atalho no Menu Iniciar
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$s=(New-Object -ComObject WScript.Shell).CreateShortcut([Environment]::GetFolderPath('Programs')+'\GustaMenu Assistente de Impressao.lnk');$s.TargetPath='%PYW%';$s.Arguments='\"%MAIN%\"';$s.WorkingDirectory='%TARGET%\app';$s.IconLocation='%PYW%';$s.Save()" >nul 2>&1

REM Atalho na Area de Trabalho
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$s=(New-Object -ComObject WScript.Shell).CreateShortcut([Environment]::GetFolderPath('Desktop')+'\GustaMenu Assistente de Impressao.lnk');$s.TargetPath='%PYW%';$s.Arguments='\"%MAIN%\"';$s.WorkingDirectory='%TARGET%\app';$s.IconLocation='%PYW%';$s.Save()" >nul 2>&1

REM Iniciar com o Windows
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v GustaMenuPrintAgent /t REG_SZ /d "\"%PYW%\" \"%MAIN%\"" /f >nul

REM Abre o assistente agora
start "" "%PYW%" "%MAIN%"

echo Instalacao concluida.
endlocal
exit /b 0
