@echo off
REM ============================================================
REM  Build do GustaMenu Assistente de Impressao (Go) p/ Windows x64
REM ============================================================
REM  Pre-requisitos:
REM    - Go 1.21+ instalado e no PATH
REM    - Inno Setup 6 instalado (para gerar o instalador)
REM ============================================================

set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

echo [1/4] Baixando dependencias...
go mod tidy
if %ERRORLEVEL% NEQ 0 ( echo ERRO: go mod tidy falhou. & exit /b 1 )

echo [2/4] Embutindo manifesto + icone (rsrc.syso)...
REM Manifesto Common-Controls v6 e OBRIGATORIO (senao a janela falha com
REM "TTM_ADDTOOL failed"). O rsrc.syso ja vai versionado; este passo apenas
REM regenera se a ferramenta rsrc estiver instalada (go install github.com/akavel/rsrc@latest).
where rsrc >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    rsrc -manifest app.manifest -ico gustamenu.ico -o rsrc.syso
    echo rsrc.syso regenerado.
) else (
    echo rsrc nao encontrado - usando rsrc.syso versionado.
)

echo [3/4] Compilando...
go build -ldflags="-s -w -H windowsgui" -o GustaMenu.PrintAgent.exe .
if %ERRORLEVEL% NEQ 0 ( echo ERRO: build falhou. & exit /b 1 )

echo Build OK: GustaMenu.PrintAgent.exe

echo.
echo [4/4] Gerando instalador (requer Inno Setup)...
set INNO="C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
if exist %INNO% (
    %INNO% installer\gustamenu.iss
    if %ERRORLEVEL% EQU 0 ( echo Instalador gerado em installer\Output\ ) else ( echo AVISO: Inno Setup retornou erro. )
) else (
    echo AVISO: Inno Setup nao encontrado. Instale em https://jrsoftware.org/isinfo.php
    echo        O executavel GustaMenu.PrintAgent.exe foi gerado e pode ser distribuido diretamente.
)
