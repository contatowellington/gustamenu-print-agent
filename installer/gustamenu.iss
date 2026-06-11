#define MyAppName    "GustaMenu Assistente de Impressao"
#define MyAppVersion "1.6.0"
#define MyAppPublisher "GustaMenu"
#define MyAppExeName "GustaMenu.PrintAgent.exe"

[Setup]
AppId={{A3F1C9D2-7B4E-4F6A-9C28-1D5E8B2A6F40}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
SetupIconFile=..\gustamenu.ico
DefaultDirName={autopf}\GustaMenu\Assistente de Impressao
UsePreviousAppDir=no
DefaultGroupName=GustaMenu
DisableProgramGroupPage=yes
OutputDir=Output
OutputBaseFilename=GustaMenu-PrintAgent-Setup-v{#MyAppVersion}
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "desktopicon"; Description: "Criar atalho na area de trabalho"; GroupDescription: "Atalhos:"; Flags: unchecked

[Files]
; Executavel compilado pelo go build
Source: "..\GustaMenu.PrintAgent.exe"; DestDir: "{app}"; Flags: ignoreversion
; Icone necessario para a bandeja do sistema e o circulo flutuante
Source: "..\gustamenu.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\GustaMenu Assistente de Impressao"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\GustaMenu Assistente de Impressao"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Abrir GustaMenu Assistente de Impressao"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{userappdata}\GustaMenu\PrintAgent"
