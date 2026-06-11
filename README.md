# GustaMenu — Assistente de Impressão (Go)

Aplicação **oficial** de desktop (Windows) que supre o GustaMenu: busca os
pedidos na fila de impressão, imprime o cupom na impressora térmica e avisa o
lojista com **alarme sonoro + visual** quando chega pedido novo.

Escrita em **Go** — gera um único `.exe`, sem runtime para instalar.

## Recursos

- **Círculo flutuante (GARNET)** sempre-no-topo, arrastável, mostrando o
  contador de pedidos e o status. Pisca em vermelho (**"PEDIDO! NOVO"**) quando
  chega pedido.
- **Ícone na bandeja**, ao lado do relógio, com menu (Configurar, Imprimir
  teste, Silenciar alarme, Abrir log, Sair).
- **Alarme intermitente** ao chegar pedido novo. Toca em loop até ser
  silenciado (clique no círculo ou menu **Silenciar**) ou até o limite de
  segundos configurado (padrão **60s**).
- **Impressão automática** em background com reporte de status à API.
- **Inicia com o Windows** (opcional).
- Instância única (não abre duas vezes).

## Configuração

Pelo círculo (duplo-clique / menu **Configurar…**) ou pelo menu da bandeja:

- Código do assistente (token do dispositivo)
- Endpoint da API (padrão `https://gustamenu.com.br/api/print_jobs.php`)
- Impressora térmica
- Intervalo de consulta (3–60 s)
- Alarme sonoro ligado/desligado e duração (5–600 s)
- Iniciar com o Windows

As preferências ficam em `%APPDATA%\GustaMenu\PrintAgent\settings.json`.

## Compilar

Pré-requisitos: Go 1.21+ e (opcional) Inno Setup 6 para o instalador.

```bat
build.bat
```

Gera `GustaMenu.PrintAgent.exe` e, se o Inno Setup estiver instalado, o
instalador em `installer\Output\`.

Manualmente:

```bat
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w -H windowsgui" -o GustaMenu.PrintAgent.exe .
```

## Instalador

O script `installer\gustamenu.iss` (Inno Setup) empacota o `.exe` + ícone,
cria atalhos e configura a desinstalação. Para gerar:

```bat
iscc installer\gustamenu.iss
```

## Versão

1.6.0
