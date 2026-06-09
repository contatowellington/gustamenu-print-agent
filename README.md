# GustaMenu — Assistente de Impressão (Python)

Agente desktop para Windows que monitora a fila de pedidos do GustaMenu,
**imprime o cupom automaticamente** na impressora térmica e **toca um
alarme sonoro** quando chega pedido novo. Roda na bandeja do sistema.

## Recursos

- 🟠 **Widget flutuante** — círculo sempre-no-topo, arrastável, com contador de pedidos e status. Duplo-clique abre as configurações; botão direito abre o menu.
- 🖨️ **Impressão automática** — busca pedidos no intervalo configurado e imprime sozinho (ESC/POS).
- 🔔 **Alarme sonoro + visual** ao chegar pedido novo (o círculo pisca em vermelho "PEDIDO!" e toca o som; ligável/desligável, repetição configurável).
- 🧾 Impressão térmica RAW via spooler do Windows (58/80 mm), texto em Latin1.
- ⚙️ Tela de configuração (código do assistente, impressora, intervalo, alarme).
- 🚀 Inicia com o Windows (registro).
- 🔒 Instância única (mutex).

## Estrutura

```
print-agent-python/
├── main.py                     # entry point
├── gustamenu_print/
│   ├── config.py               # settings.json em %APPDATA%\GustaMenu\PrintAgent
│   ├── api.py                  # fetch_jobs / report_job (urllib)
│   ├── printer.py              # impressão RAW ESC/POS (ctypes → winspool.drv)
│   ├── alarm.py                # alarme sonoro (winsound)
│   ├── worker.py               # loop de polling + impressão automática
│   ├── widget.py               # widget flutuante (círculo na tela)
│   ├── settings_window.py      # tela de config (tkinter)
│   ├── startup.py              # single-instance (ctypes) + autostart (winreg)
│   └── app.py                  # orquestração + alarme som/visual
├── requirements.txt            # (vazio — usa só a stdlib)
├── build_installer.ps1         # gera o setup.exe (makecab + IExpress)
└── installer/install.cmd       # rotina de instalação executada pelo setup
```

## Desenvolvimento

Sem dependências externas — roda direto no Python instalado (inclusive
3.15 alpha), pois usa apenas a biblioteca padrão.

```bat
py main.py
```

## Gerar instalador

Usa apenas **ferramentas Microsoft** (`makecab` + `IExpress`, já no Windows).
O runtime Python é embutido enxuto, então o lojista **não precisa ter Python**.

```powershell
powershell -ExecutionPolicy Bypass -File build_installer.ps1 -Version 1.0.0
```

Saída: `installer\Output\GustaMenu-PrintAgent-Setup-v1.0.0.exe` (~12 MB).
Instala em `%LOCALAPPDATA%\GustaMenu\PrintAgent` (sem admin), cria atalhos
e configura o início automático com o Windows.

## Configuração

Gravada em `%APPDATA%\GustaMenu\PrintAgent\settings.json`:

| Campo | Descrição |
|-------|-----------|
| `api_endpoint` | URL da fila de impressão |
| `device_token` | código do assistente (contém o ID da loja) |
| `printer_name` | impressora (vazio = padrão do Windows) |
| `poll_seconds` | intervalo de checagem (3–60) |
| `start_with_windows` | iniciar junto com o Windows |
| `alarm_enabled` | alarme sonoro ligado |
| `alarm_repeat` | repetições do alarme (1–10) |
