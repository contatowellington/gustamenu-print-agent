<#
  Gera o instalador do GustaMenu Assistente de Impressao usando apenas
  ferramentas Microsoft (makecab + IExpress), embutindo um runtime Python
  enxuto. Resultado: installer\Output\GustaMenu-PrintAgent-Setup-vX.Y.Z.exe
#>
param(
  [string]$Version = "1.0.0",
  [string]$PythonHome = "$env:LOCALAPPDATA\Programs\Python\Python315"
)

$ErrorActionPreference = "Stop"
$Root   = Split-Path -Parent $MyInvocation.MyCommand.Path
$Build  = Join-Path $Root "build"
$Stage  = Join-Path $Build "stage"
$OutDir = Join-Path $Root "installer\Output"
$SetupName = "GustaMenu-PrintAgent-Setup-v$Version.exe"

function Step($m) { Write-Host "==> $m" -ForegroundColor Cyan }

if (-not (Test-Path "$PythonHome\pythonw.exe")) { throw "Python nao encontrado em $PythonHome" }

# ---------------------------------------------------------------- limpa
Step "Limpando build/"
if (Test-Path $Build) { Remove-Item $Build -Recurse -Force }
New-Item -ItemType Directory -Force -Path $Stage,$OutDir | Out-Null
$RtDst  = Join-Path $Stage "runtime"
$AppDst = Join-Path $Stage "app"
New-Item -ItemType Directory -Force -Path $RtDst,$AppDst | Out-Null

# ------------------------------------------------------- runtime enxuto
Step "Copiando runtime Python enxuto"
Copy-Item "$PythonHome\*.exe","$PythonHome\*.dll" $RtDst -ErrorAction SilentlyContinue
Get-ChildItem "$PythonHome" -Filter "*._pth" | Copy-Item -Destination $RtDst -ErrorAction SilentlyContinue
Copy-Item "$PythonHome\DLLs" $RtDst -Recurse
Copy-Item "$PythonHome\tcl"  $RtDst -Recurse

# Lib sem testes, caches e pacotes desnecessarios
$libExclude = @("test","tests","__pycache__","site-packages","idlelib",
                "ensurepip","lib2to3","turtledemo","pydoc_data","venv")
$null = robocopy "$PythonHome\Lib" (Join-Path $RtDst "Lib") /E /NFL /NDL /NJH /NJS /NP `
                 /XD $libExclude /XF *.pyc
if ($LASTEXITCODE -ge 8) { throw "robocopy Lib falhou ($LASTEXITCODE)" }

# --------------------------------------------------------------- app
Step "Copiando o app"
$null = robocopy "$Root\gustamenu_print" (Join-Path $AppDst "gustamenu_print") /E /NFL /NDL /NJH /NJS /NP /XD __pycache__
Copy-Item "$Root\main.py" $AppDst

# --------------------------------------------------- testa o runtime
Step "Testando o runtime embutido"
& "$RtDst\python.exe" -c "import tkinter, ctypes, ssl, json, urllib.request, winsound, winreg; print('runtime-ok')"
if ($LASTEXITCODE -ne 0) { throw "runtime embutido nao passou no teste de import" }

$rtSize = (Get-ChildItem $Stage -Recurse -File | Measure-Object Length -Sum).Sum
Step ("Stage pronto: {0:N1} MB" -f ($rtSize/1MB))

# --------------------------------------------------- monta o payload.cab
Step "Gerando payload.cab (makecab, preservando subpastas)"
$ddf = Join-Path $Build "payload.ddf"
$lines = @(
  '.OPTION EXPLICIT'
  '.Set CabinetNameTemplate=payload.cab'
  ".Set DiskDirectory1=$Build"
  '.Set Cabinet=on'
  '.Set Compress=on'
  '.Set CompressionType=LZX'
  '.Set CompressionMemory=21'
  '.Set CabinetFileCountThreshold=0'
  '.Set FolderFileCountThreshold=0'
  '.Set MaxDiskFileCount=0'
  '.Set MaxCabinetSize=0'
  '.Set MaxDiskSize=0'
)
Get-ChildItem $Stage -Recurse -File | ForEach-Object {
  $rel = $_.FullName.Substring($Stage.Length + 1)
  $lines += ('"{0}" "{1}"' -f $_.FullName, $rel)
}
Set-Content -Path $ddf -Value $lines -Encoding ASCII
& makecab.exe /F $ddf | Out-Null
if (-not (Test-Path "$Build\payload.cab")) { throw "makecab nao gerou payload.cab" }
$cabSize = (Get-Item "$Build\payload.cab").Length
Step ("payload.cab: {0:N1} MB" -f ($cabSize/1MB))

# --------------------------------------------------- IExpress -> setup.exe
Step "Empacotando setup.exe (IExpress)"
Copy-Item "$Root\installer\install.cmd" $Build
$target = Join-Path $OutDir $SetupName
$sed = Join-Path $Build "setup.sed"
$sedLines = @(
  '[Version]'
  'Class=IEXPRESS'
  'SEDVersion=3'
  '[Options]'
  'PackagePurpose=InstallApp'
  'ShowInstallProgramWindow=0'
  'HideExtractAnimation=1'
  'UseLongFileName=1'
  'InsideCompressed=0'
  'CAB_FixedSize=0'
  'CAB_ResvCodeSigning=0'
  'RebootMode=N'
  'InstallPrompt=%InstallPrompt%'
  'DisplayLicense=%DisplayLicense%'
  'FinishMessage=%FinishMessage%'
  'TargetName=%TargetName%'
  'FriendlyName=%FriendlyName%'
  'AppLaunched=%AppLaunched%'
  'PostInstallCmd=%PostInstallCmd%'
  'AdminQuietInstCmd=%AdminQuietInstCmd%'
  'UserQuietInstCmd=%UserQuietInstCmd%'
  '[Strings]'
  'InstallPrompt='
  'DisplayLicense='
  'FinishMessage=GustaMenu Assistente de Impressao instalado com sucesso.'
  "TargetName=$target"
  'FriendlyName=GustaMenu Assistente de Impressao'
  'AppLaunched=cmd /c install.cmd'
  'PostInstallCmd=<None>'
  'AdminQuietInstCmd='
  'UserQuietInstCmd='
  'FILE0="payload.cab"'
  'FILE1="install.cmd"'
  '[SourceFiles]'
  "SourceFiles0=$Build"
  '[SourceFiles0]'
  '%FILE0%='
  '%FILE1%='
)
Set-Content -Path $sed -Value $sedLines -Encoding ASCII

if (Test-Path $target) { Remove-Item $target -Force }
$ix = Start-Process -FilePath "$env:WINDIR\System32\iexpress.exe" `
        -ArgumentList "/N","/Q","`"$sed`"" -Wait -PassThru

Write-Host ""
if (Test-Path $target) {
  $setupSize = (Get-Item $target).Length
  Write-Host ("OK! Instalador: {0} ({1:N1} MB)" -f $target, ($setupSize/1MB)) -ForegroundColor Green
} else {
  Write-Warning ("IExpress retornou {0} e nao gerou o setup." -f $ix.ExitCode)
  Write-Host  "O payload ja esta pronto e validado:"
  Write-Host  "  CAB ........: $Build\payload.cab"
  Write-Host  "  install.cmd : $Build\install.cmd"
  Write-Host  "  SED ........: $sed"
  Write-Host  "Para gerar o .exe numa sessao interativa, rode:" -ForegroundColor Yellow
  Write-Host  "  iexpress /N `"$sed`"" -ForegroundColor Yellow
  exit 2
}
