"""Impressão térmica RAW (ESC/POS) via Windows Print Spooler.

Usa apenas ctypes chamando winspool.drv — sem pywin32. Reproduz o
comportamento do agente Go: ESC @ (init) + texto Latin1 + avanço + GS V 0 (corte).
"""

from __future__ import annotations

import ctypes
from ctypes import wintypes
from typing import List

from .api import PrintJob
from .config import Config

# --- comandos ESC/POS ----------------------------------------------------
ESC_INIT = b"\x1b\x40"        # ESC @  — inicializa a impressora
ESC_CUT = b"\x1d\x56\x00"     # GS V 0 — corta o papel
FEED = b"\x0a\x0a\x0a"        # alimenta o papel

# --- bindings winspool.drv ----------------------------------------------
_winspool = ctypes.WinDLL("winspool.drv", use_last_error=True)


class _DOC_INFO_1(ctypes.Structure):
    _fields_ = [
        ("pDocName", wintypes.LPWSTR),
        ("pOutputFile", wintypes.LPWSTR),
        ("pDatatype", wintypes.LPWSTR),
    ]


class _PRINTER_INFO_4(ctypes.Structure):
    _fields_ = [
        ("pPrinterName", wintypes.LPWSTR),
        ("pServerName", wintypes.LPWSTR),
        ("Attributes", wintypes.DWORD),
    ]


_OpenPrinter = _winspool.OpenPrinterW
_OpenPrinter.argtypes = [wintypes.LPWSTR, ctypes.POINTER(wintypes.HANDLE), wintypes.LPVOID]
_OpenPrinter.restype = wintypes.BOOL

_StartDocPrinter = _winspool.StartDocPrinterW
_StartDocPrinter.argtypes = [wintypes.HANDLE, wintypes.DWORD, ctypes.POINTER(_DOC_INFO_1)]
_StartDocPrinter.restype = wintypes.DWORD

_StartPagePrinter = _winspool.StartPagePrinter
_StartPagePrinter.argtypes = [wintypes.HANDLE]
_StartPagePrinter.restype = wintypes.BOOL

_WritePrinter = _winspool.WritePrinter
_WritePrinter.argtypes = [wintypes.HANDLE, wintypes.LPVOID, wintypes.DWORD, ctypes.POINTER(wintypes.DWORD)]
_WritePrinter.restype = wintypes.BOOL

_EndPagePrinter = _winspool.EndPagePrinter
_EndPagePrinter.argtypes = [wintypes.HANDLE]
_EndPagePrinter.restype = wintypes.BOOL

_EndDocPrinter = _winspool.EndDocPrinter
_EndDocPrinter.argtypes = [wintypes.HANDLE]
_EndDocPrinter.restype = wintypes.BOOL

_ClosePrinter = _winspool.ClosePrinter
_ClosePrinter.argtypes = [wintypes.HANDLE]
_ClosePrinter.restype = wintypes.BOOL

_GetDefaultPrinter = _winspool.GetDefaultPrinterW
_GetDefaultPrinter.argtypes = [wintypes.LPWSTR, ctypes.POINTER(wintypes.DWORD)]
_GetDefaultPrinter.restype = wintypes.BOOL

_EnumPrinters = _winspool.EnumPrintersW
_EnumPrinters.argtypes = [
    wintypes.DWORD, wintypes.LPWSTR, wintypes.DWORD, wintypes.LPBYTE,
    wintypes.DWORD, ctypes.POINTER(wintypes.DWORD), ctypes.POINTER(wintypes.DWORD),
]
_EnumPrinters.restype = wintypes.BOOL

_PRINTER_ENUM_LOCAL = 0x00000002
_PRINTER_ENUM_CONNECTIONS = 0x00000004


def _winerror(func: str) -> RuntimeError:
    err = ctypes.get_last_error()
    return RuntimeError(f"{func} falhou (erro {err})")


# --- texto ---------------------------------------------------------------
def _normalize_line_endings(text: str) -> str:
    return text.replace("\r\n", "\n").replace("\r", "\n").replace("\n", "\r\n")


def _to_iso8859_1(text: str) -> bytes:
    """Converte para Latin1; caracteres fora de 0x00–0xFF viram '?'."""
    return text.encode("iso-8859-1", errors="replace")


def build_print_data(text: str) -> bytes:
    body = _to_iso8859_1(_normalize_line_endings(text))
    return ESC_INIT + body + FEED + ESC_CUT


# --- impressoras ---------------------------------------------------------
def default_printer_name() -> str:
    size = wintypes.DWORD(0)
    _GetDefaultPrinter(None, ctypes.byref(size))
    if size.value == 0:
        return ""
    buf = ctypes.create_unicode_buffer(size.value)
    if not _GetDefaultPrinter(buf, ctypes.byref(size)):
        return ""
    return buf.value


def installed_printers() -> List[str]:
    flags = _PRINTER_ENUM_LOCAL | _PRINTER_ENUM_CONNECTIONS
    level = 4
    needed = wintypes.DWORD(0)
    returned = wintypes.DWORD(0)

    _EnumPrinters(flags, None, level, None, 0, ctypes.byref(needed), ctypes.byref(returned))
    if needed.value == 0:
        return []

    buf = ctypes.create_string_buffer(needed.value)
    ok = _EnumPrinters(
        flags, None, level, ctypes.cast(buf, wintypes.LPBYTE),
        needed.value, ctypes.byref(needed), ctypes.byref(returned),
    )
    if not ok or returned.value == 0:
        return []

    array = ctypes.cast(buf, ctypes.POINTER(_PRINTER_INFO_4))
    names = [array[i].pPrinterName for i in range(returned.value) if array[i].pPrinterName]
    names.sort()
    return names


def raw_print(printer_name: str, data: bytes) -> None:
    if not data:
        raise RuntimeError("buffer de impressão vazio")

    handle = wintypes.HANDLE()
    if not _OpenPrinter(printer_name, ctypes.byref(handle), None):
        raise _winerror(f"OpenPrinter({printer_name!r})")

    try:
        doc = _DOC_INFO_1("GustaMenu Cupom", None, "RAW")
        if _StartDocPrinter(handle, 1, ctypes.byref(doc)) == 0:
            raise _winerror("StartDocPrinter")
        try:
            if not _StartPagePrinter(handle):
                raise _winerror("StartPagePrinter")
            written = wintypes.DWORD(0)
            buf = ctypes.create_string_buffer(data, len(data))
            if not _WritePrinter(handle, buf, len(data), ctypes.byref(written)):
                raise _winerror("WritePrinter")
            _EndPagePrinter(handle)
        finally:
            _EndDocPrinter(handle)
    finally:
        _ClosePrinter(handle)


def print_job(cfg: Config, job: PrintJob) -> None:
    printer_name = cfg.printer_name
    if not printer_name:
        printer_name = default_printer_name()
        if not printer_name:
            raise RuntimeError(
                "nenhuma impressora disponível — configure em Configurar"
            )
    raw_print(printer_name, build_print_data(job.receipt_text))
