//go:build !windows

package main

func acquireSingleInstance() bool { return true }

func setStartWithWindows(_ bool) {}
