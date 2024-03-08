package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFile(t *testing.T) {
	hasFileBool := downloadFile("https://github.com/Mzdyl/LiteLoaderQQNT_Install/releases/download/1.9/install_windows.exe", "windows_install")
	log.Println(hasFileBool)
}

func TestGetQQPathInfo(t *testing.T) {
	log.Println(getQQExePath())
}

func TestIsFileExists(t *testing.T) {
	_, qqNTPath := getQQExePath()
	fmt.Println(isFileExists(filepath.Join(qqNTPath, "dbghelp.dll")))
	fmt.Println(isFileExists(qqNTPath))
}

func TestGetFileMd5(t *testing.T) {
	//5c4457f8ed767669d163c7376dbb97c9
	log.Println(getFileMD5(filepath.Join(`d:\Program Files\Tencent\QQNT`, "QQ.exe")))
}

func getFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func TestDownLoadAndInstallLiteLoader(t *testing.T) {
	downLoadAndInstallLiteLoader(`d:\Program Files\Tencent\QQNT`)
}

func TestPrepareForInstallation(t *testing.T) {
	prepareForInstallation(`d:\Program Files\Tencent\QQNT\QQ.exe`, `d:\Program Files\Tencent\QQNT`)
}

func TestCopyOldFiles(t *testing.T) {
	copyOldFiles(`d:\Program Files\Tencent\QQNT`)
}

func TestPatchIndexJS(t *testing.T) {
	patchIndexJS(`d:\Program Files\Tencent\QQNT`)
}

func TestDownLoadAndInstallLLOneBot(t *testing.T) {
	downLoadAndInstallLLOneBot(`d:\Program Files\Tencent\QQNT`)
}
