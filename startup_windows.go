//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW  = kernel32.NewProc("CreateMutexW")

	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

const (
	hkeyCurrentUser  = uintptr(0x80000001)
	keySetValue      = uintptr(0x0002)
	regSZ            = uintptr(1)
	errorAlreadyExists = syscall.Errno(183)
)

var singleInstanceHandle uintptr

// acquireSingleInstance cria um mutex global para garantir uma única instância.
// Retorna false se o app já estiver em execução.
func acquireSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString("Global\\GustaMenuPrintAgent")
	h, _, lastErr := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true // falha ao criar mutex — permite executar
	}
	if lastErr == errorAlreadyExists {
		syscall.CloseHandle(syscall.Handle(h))
		return false
	}
	singleInstanceHandle = h // mantém vivo pelo tempo de vida do processo
	return true
}

// setStartWithWindows adiciona ou remove a entrada de inicialização automática no registro.
func setStartWithWindows(enable bool) {
	subkey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Run`)
	var hKey uintptr

	r, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(subkey)),
		0,
		keySetValue,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return
	}
	defer procRegCloseKey.Call(hKey)

	valueName, _ := syscall.UTF16PtrFromString("GustaMenuPrintAgent")

	if !enable {
		procRegDeleteValueW.Call(hKey, uintptr(unsafe.Pointer(valueName)))
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}

	// Valor: "C:\...\GustaMenu.PrintAgent.exe"
	val, _ := syscall.UTF16FromString(`"` + exePath + `"`)
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valueName)),
		0,
		regSZ,
		uintptr(unsafe.Pointer(&val[0])),
		uintptr(len(val)*2),
	)
}
