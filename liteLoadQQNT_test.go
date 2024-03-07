package main

import (
	"log"
	"testing"
)

func TestDownloadFile(t *testing.T) {
	hasFileBool := downloadFile("https://github.com/Mzdyl/LiteLoaderQQNT_Install/releases/download/1.9/install_windows.exe", "windows_install")
	log.Println(hasFileBool)
}
