; Instalador do GustaMenu Assistente de Impressao (Inno Setup).
; Empacota o runtime Python enxuto + o app a partir de ..\build\stage
; (gerado por build_installer.ps1). Instala em %LOCALAPPDATA% sem admin.

#define MyAppName    "GustaMenu Assistente de Impressao"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "GustaMenu"

[Setup]
AppId={{C2A6F7D1-9E4B-4F3A-8C21-7A4E0B9D5C10}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\GustaMenu\PrintAgent\app_files
UsePreviousAppDir=no
DisableProgramGroupPage=yes
DisableDirPage=yes
PrivilegesRequired=lowest
OutputDir=Output
OutputBaseFilename=GustaMenu-PrintAgent-Setup-v{#MyAppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayName={#MyAppName}

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "desktopicon"; Description: "Criar atalho na area de trabalho"; Flags: unchecked
Name: "startup"; Description: "Iniciar com o Windows"; Flags: checkedonce

[Files]
Source: "..\build\stage\runtime\*"; DestDir: "{app}\runtime"; Flags: recursesubdirs createallsubdirs ignoreversion
Source: "..\build\stage\app\*";     DestDir: "{app}\app";     Flags: recursesubdirs createallsubdirs ignoreversion

[Icons]
Name: "{autoprograms}\GustaMenu Assistente de Impressao"; Filename: "{app}\runtime\pythonw.exe"; Parameters: """{app}\app\main.py"""; WorkingDir: "{app}\app"
Name: "{autodesktop}\GustaMenu Assistente de Impressao"; Filename: "{app}\runtime\pythonw.exe"; Parameters: """{app}\app\main.py"""; WorkingDir: "{app}\app"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "GustaMenuPrintAgent"; ValueData: """{app}\runtime\pythonw.exe"" ""{app}\app\main.py"""; Flags: uninsdeletevalue; Tasks: startup

[Run]
Filename: "{app}\runtime\pythonw.exe"; Parameters: """{app}\app\main.py"""; Description: "Abrir o GustaMenu Assistente de Impressao"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{app}"
