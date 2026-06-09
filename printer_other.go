//go:build !windows

package main

import "fmt"

func printJob(_ Config, job PrintJob) error {
	return fmt.Errorf("impressão não suportada nesta plataforma (job %d)", job.ID)
}

func installedPrinters() []string {
	return nil
}

func defaultPrinterName() (string, error) {
	return "", nil
}
