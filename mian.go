package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	thisAppVersion = "1.1"                         // 当前版本号，记得打git tag的时候这里要一致
	proxyUrl       = "https://mirror.ghproxy.com/" // 存储反代服务器的URL

)

var (
	SIG_X64 = []byte{0x48, 0x89, 0xCE, 0x48, 0x8B, 0x11, 0x4C, 0x8B, 0x41, 0x08, 0x49, 0x29, 0xD0, 0x48, 0x8B, 0x49, 0x18, 0xE8}
	FIX_X64 = []byte{0x48, 0x89, 0xCE, 0x48, 0x8B, 0x11, 0x4C, 0x8B, 0x41, 0x08, 0x49, 0x29, 0xD0, 0x48, 0x8B, 0x49, 0x18, 0xB8, 0x01, 0x00, 0x00, 0x00}

	SIG_X86 = []byte{0x89, 0xCE, 0x8B, 0x01, 0x8B, 0x49, 0x04, 0x29, 0xC1, 0x51, 0x50, 0xFF, 0x76, 0x0C, 0xE8}
	FIX_X86 = []byte{0x89, 0xCE, 0x8B, 0x01, 0x8B, 0x49, 0x04, 0x29, 0xC1, 0x51, 0x50, 0xFF, 0x76, 0x0C, 0xB8, 0x01, 0x00, 0x00, 0x00}
)

type githubApiInfo struct {
	Body    string `json:"body"`
	TagName string `json:"tag_name"`
}

// go build -ldflags "-s -w" .
func main() {
	defer exitWithEnter()
	// 检测是否以管理员模式运行
	isAdminBool, err := isAdmin()
	if err != nil {
		log.Println(err)
		return
	}
	if !isAdminBool {
		log.Println("当前不是以管理员身份运行的，请关闭本程序并右击使用管理员身份运行，避免造成权限不足")
		return
	}
	qqExeFilePath, qqNTPath := getQQExePath()
	if qqExeFilePath == "" || qqNTPath == "" {
		log.Println("未找到QQ.exe绝对路径，请正确安装QQNT")
		return
	}
	if !checkForUpdate() {
		return
	}
	if isFileExists(filepath.Join(qqNTPath, "dbghelp.dll")) {
		log.Println("检测到dbghelp.dll，推测你已修补QQ，跳过修补")
	} else {
		// 开始修补
		if !patchPEFile(qqExeFilePath) {
			log.Println("修复失败")
			return
		}
	}
	downLoadAndInstallLiteLoader(qqNTPath)

}

// 判断文件是否存在
func isFileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

// 查看是否有新版本
func checkForUpdate() bool {
	resp, err := getHttpClient(3).Get("https://api.github.com/repos/anyanfei/go-liteLoadQQNT/releases/latest")
	if err != nil {
		log.Println("检查更新时发生错误，无法获取https://api.github.com/repos/anyanfei/go-liteLoadQQNT/releases/latest信息")
		return false
	}
	defer resp.Body.Close()
	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return false
	}
	var tempGithubApiInfo githubApiInfo
	err = json.Unmarshal(respByte, &tempGithubApiInfo)
	if err != nil {
		log.Println(err)
		return false
	}
	latestTagName := tempGithubApiInfo.TagName
	latestBody := tempGithubApiInfo.Body
	if latestTagName > thisAppVersion { // 发现新版本，下载新版本
		log.Println("发现新版本", latestTagName)
		log.Println("更新日志：", latestBody)
		newFileName := fmt.Sprintf("liteLoadQQNT-%s", latestTagName)
		log.Println("正在下载新版本，这可能需要一些时间")
		downloadBool := downloadFile("https://github.com/anyanfei/go-liteLoadQQNT/releases/download/"+latestTagName+"/liteLoadQQNT.exe", newFileName)
		if !downloadBool {
			return false
		}
		log.Println("版本号已更新，请重新运行执行新文件", newFileName+".exe")
		return false
	}
	log.Println("当前已经是最新版本，开始安装")
	return true
}

// 返回QQNT exe的绝对路径和当前QQNT的路径绝对
func getQQExePath() (string, string) {
	keyInfo, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\QQ`, registry.READ)
	if err != nil {
		log.Println(err)
		return "", ""
	}
	defer keyInfo.Close()
	uninstallStringValue, _, err := keyInfo.GetStringValue("UninstallString")
	if err != nil {
		log.Println(err)
		return "", ""
	}
	if uninstallStringValue == "" {
		log.Println("未找到QQNT的安装目录，请加QQ群：1048452984截图咨询")
		return "", ""
	}
	qqExePath := strings.Replace(uninstallStringValue, "Uninstall.exe", "QQ.exe", 1)
	return qqExePath, filepath.Dir(qqExePath)
}

// 是否可以连接代理的github，能则返回true，反之false
func canConnectToGithubWithProxy(downloadUrl string) bool {
	resp, err := getHttpClient(5).Get(proxyUrl + downloadUrl)
	if err != nil {
		log.Println(err)
		return false
	}
	return resp.StatusCode == http.StatusOK
}

// 下载文件，并重命名文件名字(fileName只需要传名字不用包含后缀)
func downloadFile(needDownloadUrl, fileName string) bool {
	if canConnectToGithubWithProxy(needDownloadUrl) {
		needDownloadUrl = proxyUrl + needDownloadUrl
	}
	res, err := getHttpClient(60).Get(needDownloadUrl)
	if err != nil {
		log.Println("无法访问当前url", needDownloadUrl, err)
		return false
	}
	defer res.Body.Close()
	reader := bufio.NewReaderSize(res.Body, 32*1024)
	newFilePath, err := os.Create(fileName + path.Ext(needDownloadUrl))
	if err != nil {
		log.Println(err)
		return false
	}
	writer := bufio.NewWriter(newFilePath)
	written, err := io.Copy(writer, reader)
	if err != nil {
		log.Println(err)
		return false
	}
	fmt.Printf("下载文件成功，文件长度为 :%d\r\n", written)
	return true
}

func scanAndReplace(buffer *[]byte, pattern []byte, replacement []byte) {
	index := bytes.Index(*buffer, pattern)
	for index != -1 {
		*buffer = append((*buffer)[:index], append(replacement, (*buffer)[index+len(pattern):]...)...)
		fmt.Printf("Found at 0x%X", index)
		index += len(replacement)
		index = bytes.Index((*buffer)[index:], pattern)
	}
}

// 修补exe文件
func patchPEFile(filePath string) bool {
	savePath := filePath + ".bak"
	err := os.Rename(filePath, savePath)
	if err != nil {
		log.Println("Error renaming file:", err)
		return false
	}
	log.Printf("已将原版备份在 : %s\r\n", savePath)

	peFileBytes, err := os.ReadFile(savePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return false
	}
	if 32<<(^uint(0)>>63) == 32 { // 判断当前是64位还是32位系统
		scanAndReplace(&peFileBytes, SIG_X86, FIX_X86)
	} else {
		scanAndReplace(&peFileBytes, SIG_X64, FIX_X64)
	}
	err = os.WriteFile(filePath, peFileBytes, 0644)
	if err != nil {
		log.Println("Error writing file:", err)
		return false
	}
	log.Println("修补成功！")
	return true
}

func downLoadAndInstallLiteLoader(qqNTPath string) bool {
	log.Println("正在拉取LiteLoaderQQNT最新版本的仓库...")
	//downloadBool := downloadFile(`https://github.com/LiteLoaderQQNT/LiteLoaderQQNT/archive/master.zip`, "LiteLoader")
	//if !downloadBool {
	//	return false
	//}
	thisDir, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println("开始解压到LiteLoader文件夹")
	unZip(filepath.Join(thisDir, "LiteLoader.zip"), "LiteLoader")
	log.Println("拉取完成，正在安装 LiteLoaderQQNT")
	// 移除目标路径及其内容
	LiteLoaderQQNTBakPath := filepath.Join(qqNTPath, `resources`, `app`, `LiteLoaderQQNT_bak`)
	if isFileExists(LiteLoaderQQNTBakPath) {
		err = os.RemoveAll(LiteLoaderQQNTBakPath)
		if err != nil {
			log.Println("请完全关闭QQ，重新执行")
			return false
		}
	}
	sourceDir := filepath.Join(qqNTPath, `resources`, `app`, `LiteLoaderQQNT-main`)
	destinationDir := filepath.Join(LiteLoaderQQNTBakPath)
	log.Println("从", filepath.Join(thisDir, `LiteLoader`, `LiteLoaderQQNT-main`), "移动到", sourceDir)
	if isFileExists(sourceDir) {
		err = os.Rename(sourceDir, destinationDir)
		if err != nil {
			log.Println("请完全关闭QQ，重新执行", err)
			return false
		}
		log.Println("已将旧版重命名为: ", destinationDir)
	} else {
		log.Println(sourceDir, "不存在，全新安装")
		err = moveFolder(filepath.Join(thisDir, `LiteLoader`, `LiteLoaderQQNT-main`), sourceDir)
		if err != nil {
			log.Println("移动文件夹出错", err)
			return false
		}
	}
	return true
}

// 移动文件夹
func moveFolder(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		} else {
			return moveOneFile(path, dstPath)
		}
	})
}

// 跨卷移动
func moveOneFile(oldPath, newPath string) error {
	from, err := syscall.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	return syscall.MoveFile(from, to)
}

// 解压一个zip包，传入zip包的绝对地址和需要输出的当前路径的文件
func unZip(zipPath string, outPutFileName string) bool {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Println(err)
		return false
	}
	defer archive.Close()
	for _, f := range archive.File {
		filePath := filepath.Join(outPutFileName, f.Name)
		fmt.Println("开始解压文件... ", filePath)
		if !strings.HasPrefix(filePath, filepath.Clean(outPutFileName)+string(os.PathSeparator)) {
			log.Println("无效的文件路径")
			return false
		}
		if f.FileInfo().IsDir() {
			fmt.Println("创建目录...")
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}
		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			log.Println(err)
			return false
		}
		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			log.Println(err)
			return false
		}
		fileInArchive, err := f.Open()
		if err != nil {
			log.Println(err)
			return false
		}
		if _, err = io.Copy(dstFile, fileInArchive); err != nil {
			log.Println(err)
			return false
		}
		dstFile.Close()
		fileInArchive.Close()
	}
	return true
}

func getHttpClient(timeOut int) *http.Client {
	return &http.Client{
		Timeout: time.Duration(timeOut) * time.Second,
	}
}

// 判断当前程序是否以管理员身份运行
func isAdmin() (bool, error) {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0,
		0,
		0,
		0,
		0,
		0,
		&sid,
	)
	if err != nil {
		return false, err
	}
	token := windows.Token(0)
	return token.IsMember(sid)
}

// 按回车键退出程序
func exitWithEnter() {
	log.Println("请按回车键退出当前程序 ...")
	os.Stdin.Read(make([]byte, 1))
}
