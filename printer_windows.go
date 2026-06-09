//go:build windows

package main

import (
	"bytes"
	"fmt"
	"sort"
	"syscall"
	"unsafe"
)

var (
	winspoolDll = syscall.NewLazyDLL("winspool.drv")

	procOpenPrinterW       = winspoolDll.NewProc("OpenPrinterW")
	procStartDocPrinterW   = winspoolDll.NewProc("StartDocPrinterW")
	procStartPagePrinter   = winspoolDll.NewProc("StartPagePrinter")
	procWritePrinter       = winspoolDll.NewProc("WritePrinter")
	procEndPagePrinter     = winspoolDll.NewProc("EndPagePrinter")
	procEndDocPrinter      = winspoolDll.NewProc("EndDocPrinter")
	procClosePrinter       = winspoolDll.NewProc("ClosePrinter")
	procGetDefaultPrinterW = winspoolDll.NewProc("GetDefaultPrinterW")
	procEnumPrintersW      = winspoolDll.NewProc("EnumPrintersW")
)

// docInfo1W mapeia DOC_INFO_1W do Windows.
type docInfo1W struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}

// printerInfo4 mapeia PRINTER_INFO_4W do Windows (64-bit: 24 bytes).
type printerInfo4 struct {
	pPrinterName *uint16
	pServerName  *uint16
	attributes   uint32
	_            [4]byte // padding para alinhamento de 8 bytes
}

// Comandos ESC/POS para impressoras térmicas.
var (
	escInit = []byte{0x1B, 0x40}             // ESC @ — inicializa impressora
	escCut  = []byte{0x1D, 0x56, 0x00}       // GS V 0 — corta o papel
)

// printJob imprime um job na impressora configurada (ou na padrão do sistema).
func printJob(cfg Config, job PrintJob) error {
	printerName := cfg.Printer
	if printerName == "" {
		var err error
		printerName, err = defaultPrinterName()
		if err != nil {
			return fmt.Errorf("impressora padrão: %w", err)
		}
		if printerName == "" {
			return fmt.Errorf("nenhuma impressora disponível — configure em Configurar")
		}
	}

	data := buildPrintData(job.ReceiptText)
	return rawPrint(printerName, data)
}

// buildPrintData monta o buffer ESC/POS com o texto do cupom em codificação Latin1.
func buildPrintData(text string) []byte {
	var buf bytes.Buffer
	buf.Write(escInit)

	// Normaliza quebras de linha para \r\n (padrão ESC/POS)
	text = normalizeLineEndings(text)

	// Converte UTF-8 para Latin1 (ISO-8859-1), compatível com a maioria das
	// impressoras térmicas. Caracteres fora do intervalo 0x00-0xFF são substituídos por '?'.
	buf.Write(toISO88591(text))

	buf.Write([]byte{0x0A, 0x0A, 0x0A}) // alimenta o papel
	buf.Write(escCut)
	return buf.Bytes()
}

// normalizeLineEndings substitui \r\n e \r por \r\n.
func normalizeLineEndings(s string) string {
	s = strings_ReplaceAll(s, "\r\n", "\n")
	s = strings_ReplaceAll(s, "\r", "\n")
	s = strings_ReplaceAll(s, "\n", "\r\n")
	return s
}

// strings_ReplaceAll substitui todas as ocorrências de old por new em s.
func strings_ReplaceAll(s, old, new string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result = append(result, new...)
			i += len(old)
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

// toISO88591 converte uma string UTF-8 para bytes ISO-8859-1.
func toISO88591(s string) []byte {
	buf := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 0x100 {
			buf = append(buf, byte(r))
		} else {
			buf = append(buf, '?')
		}
	}
	return buf
}

// defaultPrinterName retorna o nome da impressora padrão do Windows.
func defaultPrinterName() (string, error) {
	var size uint32
	procGetDefaultPrinterW.Call(0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return "", nil
	}

	buf := make([]uint16, size)
	r, _, err := procGetDefaultPrinterW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 {
		return "", fmt.Errorf("GetDefaultPrinterW: %w", err)
	}

	return syscall.UTF16ToString(buf), nil
}

// installedPrinters retorna a lista de impressoras instaladas no Windows, ordenada.
func installedPrinters() []string {
	const flags = 0x2 | 0x4 // PRINTER_ENUM_LOCAL | PRINTER_ENUM_CONNECTIONS
	const level = 4

	var needed, returned uint32

	// Primeira chamada para obter o tamanho do buffer necessário
	procEnumPrintersW.Call(
		flags, 0, level, 0, 0,
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if needed == 0 {
		return nil
	}

	buf := make([]byte, needed)
	r, _, _ := procEnumPrintersW.Call(
		flags, 0, level,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if r == 0 || returned == 0 {
		return nil
	}

	infoSize := uint32(unsafe.Sizeof(printerInfo4{}))
	printers := make([]string, 0, returned)
	for i := uint32(0); i < returned; i++ {
		info := (*printerInfo4)(unsafe.Pointer(&buf[i*infoSize]))
		if info.pPrinterName == nil {
			continue
		}
		name := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(info.pPrinterName))[:])
		if name != "" {
			printers = append(printers, name)
		}
	}

	sort.Strings(printers)
	return printers
}

// rawPrint envia dados RAW para a impressora via Windows Print Spooler.
func rawPrint(printerName string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("buffer de impressão vazio")
	}

	pName, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return fmt.Errorf("nome de impressora inválido: %w", err)
	}

	var hPrinter uintptr
	r, _, err := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(pName)),
		uintptr(unsafe.Pointer(&hPrinter)),
		0,
	)
	if r == 0 {
		return fmt.Errorf("OpenPrinter(%q): %w", printerName, err)
	}
	defer procClosePrinter.Call(hPrinter)

	docName, _ := syscall.UTF16PtrFromString("BoraPede Cupom")
	rawType, _ := syscall.UTF16PtrFromString("RAW")
	di := docInfo1W{
		pDocName:  docName,
		pDatatype: rawType,
	}

	r, _, err = procStartDocPrinterW.Call(
		hPrinter, 1,
		uintptr(unsafe.Pointer(&di)),
	)
	if r == 0 {
		return fmt.Errorf("StartDocPrinter: %w", err)
	}
	defer procEndDocPrinter.Call(hPrinter)

	r, _, err = procStartPagePrinter.Call(hPrinter)
	if r == 0 {
		return fmt.Errorf("StartPagePrinter: %w", err)
	}
	defer procEndPagePrinter.Call(hPrinter)

	var written uint32
	r, _, err = procWritePrinter.Call(
		hPrinter,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r == 0 {
		return fmt.Errorf("WritePrinter: %w", err)
	}

	return nil
}
