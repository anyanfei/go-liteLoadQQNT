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
	thisAppVersion = "1.2"                         // 当前版本号，记得打git tag的时候这里要一致
	proxyUrl       = "https://mirror.ghproxy.com/" // 存储反代服务器的URL

)

var (
	thisDir = ""
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
	thisDir, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return
	}
	defer os.RemoveAll(filepath.Join(thisDir, "LiteLoader.zip"))
	defer os.RemoveAll(filepath.Join(thisDir, "LiteLoader"))
	defer os.RemoveAll(filepath.Join(thisDir, "LLOneBot.zip"))
	defer exitWithEnter()
	ts := time.Now()
	// 检测是否以管理员模式运行
	if isAdminBool, _ := isAdmin(); !isAdminBool {
		log.Println("当前不是以管理员身份运行的，请关闭本程序并右击使用管理员身份运行，避免造成权限不足")
		return
	}
	// 获取QQ文件绝对路径和QQ文件夹绝对路径
	qqExeFilePath, qqNTPath := getQQExePath()
	if qqExeFilePath == "" || qqNTPath == "" {
		log.Println("未找到QQ.exe绝对路径，请正确安装QQNT")
		return
	}
	// 检测本go-liteLoadQQNT是否需要升级
	if !checkForUpdate() {
		return
	}
	// 预安装，检测之前是否已安装过LiteLoader
	if !prepareForInstallation(qqExeFilePath, qqNTPath) {
		return
	}
	// 查看dbghelp.dll是否存在
	if isFileExists(filepath.Join(qqNTPath, "dbghelp.dll")) {
		log.Println("检测到dbghelp.dll，推测你已修补QQ，跳过修补")
	} else {
		// 开始修补
		if !patchPEFile(qqExeFilePath) {
			log.Println("修复失败")
			return
		}
	}
	// 下载LiteLoaderQQNT，并安装
	if !downLoadAndInstallLiteLoader(qqNTPath) {
		return
	}
	// 将LiteLoaderQQNT_bak中的插件和插件保存的数据都复制到LiteLoaderQQNT-main里面
	if !copyOldFiles(qqNTPath) {
		return
	}
	// 把resources/app/app_launcher/index.js中的内容替换成LiteLoaderQQNT-main的绝对路径，最下面一行不变
	if !patchIndexJS(qqNTPath) {
		return
	}
	// 下载并安装LLOneBot https://github.com/LLOneBot/LLOneBot/releases/download/v3.13.7/LLOneBot.zip 检测最新版
	if !downLoadAndInstallLLOneBot(qqNTPath) {
		return
	}
	log.Println("全部完成，总用时：", time.Since(ts), "欢迎来QQ群里交流：1048452984")
}

// 判断文件是否存在，也可以判断目录是否存在
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
		log.Println("未找到QQNT的安装目录，请加QQ群：1048452984 截图咨询")
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

func scanAndReplace(buffer, pattern, replacement []byte) {
	index := 0
	for index < len(buffer) {
		foundIndex := bytes.Index(buffer[index:], pattern)
		if foundIndex == -1 {
			break
		}
		index += foundIndex
		copy(buffer[index:index+len(replacement)], replacement)
		fmt.Printf("Found at 0x%08X\n", index)
		index += len(replacement)
	}
}

// 修补exe文件
func patchPEFile(filePath string) bool {
	savePath := filePath + ".bak"
	err := copyFile(filePath, savePath)
	if err != nil {
		log.Println("Error copy file:", err)
		return false
	}
	log.Printf("已将原版备份在 : %s\r\n", savePath)

	peFileBytes, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return false
	}
	if 32<<(^uint(0)>>63) == 32 { // 判断当前是32位还是64位系统
		log.Println("32位系统")
		scanAndReplace(peFileBytes, SIG_X86, FIX_X86)
	} else {
		log.Println("64位系统")
		scanAndReplace(peFileBytes, SIG_X64, FIX_X64)
	}
	err = os.WriteFile(filePath, peFileBytes, 0644)
	if err != nil {
		log.Println("Error writing file:", err)
		return false
	}
	log.Println("修补成功！")
	return true
}

// 预安装，检测之前是否已安装过LiteLoader
func prepareForInstallation(qqExePath, qqNTPath string) bool {
	packageFilePath := filepath.Join(qqNTPath, "resources", "app", "package.json")
	replacementLine := `"main": "./app_launcher/index.js"`
	targetLine := `"main": "./LiteLoader"`

	// 读package.json，是有一行是targetLine的内容，如果是，则替换为replacementLine的内容
	content, err := os.ReadFile(packageFilePath)
	if err != nil {
		fmt.Println("读取 package.json 文件失败：", err)
		return false
	}
	if strings.Contains(string(content), targetLine) {
		fmt.Println("检测到安装过旧版，执行复原 package.json")
		newContent := strings.ReplaceAll(string(content), targetLine, replacementLine)
		err = os.WriteFile(packageFilePath, []byte(newContent), 0644)
		if err != nil {
			log.Println("写入 package.json 文件失败：", err)
			return false
		}
		log.Printf("成功替换目标行： %s -> %s", targetLine, replacementLine)
		log.Println("请根据需求自行删除 LiteloaderQQNT 0.x 版本本体以及 LITELOADERQQNT_PROFILE 环境变量以及对应目录")
	} else {
		log.Println("未安装过旧版，全新安装")
	}

	// 看是否需要删除备份的QQ.exe.bak
	bakFilePath := qqExePath + ".bak"
	if isFileExists(bakFilePath) {
		err = os.Remove(bakFilePath)
		if err != nil {
			log.Println("删除备份文件失败：", err)
			return false
		}
		log.Printf("已删除备份文件： %s", bakFilePath)
	} else {
		log.Println("备份文件不存在，无需删除。")
	}
	return true
}

// 下载LiteLoaderQQNT，并安装
func downLoadAndInstallLiteLoader(qqNTPath string) bool {
	log.Println("正在拉取LiteLoaderQQNT最新版本的仓库...")
	if !downloadFile(`https://github.com/LiteLoaderQQNT/LiteLoaderQQNT/archive/master.zip`, "LiteLoader") {
		return false
	}
	log.Println("开始解压到LiteLoader文件夹")
	unZip(filepath.Join(thisDir, "LiteLoader.zip"), "LiteLoader")
	// 删除一下下载的压缩包文件
	log.Println("解压完成，正在安装 LiteLoaderQQNT")
	// 移除目标路径及其内容
	LiteLoaderQQNTBakPath := filepath.Join(qqNTPath, `resources`, `app`, `LiteLoaderQQNT_bak`)
	if isFileExists(LiteLoaderQQNTBakPath) {
		err := os.RemoveAll(LiteLoaderQQNTBakPath)
		if err != nil {
			log.Println("请完全关闭QQ，重新执行")
			return false
		}
	}
	sourceDir := filepath.Join(qqNTPath, `resources`, `app`, `LiteLoaderQQNT-main`)
	log.Println("从", filepath.Join(thisDir, `LiteLoader`, `LiteLoaderQQNT-main`), "移动到", sourceDir)
	if isFileExists(sourceDir) {
		err := os.Rename(sourceDir, LiteLoaderQQNTBakPath)
		if err != nil {
			log.Println("请完全关闭QQ，重新执行", err)
			return false
		}
		log.Println("已将旧版重命名为: ", LiteLoaderQQNTBakPath)
		os.RemoveAll(sourceDir) // 删掉全部的LiteLoaderQQNT-main
	}
	log.Println(sourceDir, "不存在，全新安装")
	err := moveOrCopyFolder(filepath.Join(thisDir, `LiteLoader`, `LiteLoaderQQNT-main`), sourceDir, true)
	if err != nil {
		log.Println("移动文件夹出错", err)
		return false
	}
	return true
}

// 下载并安装LLOneBot插件
func downLoadAndInstallLLOneBot(qqNTPath string) bool {
	resp, err := getHttpClient(3).Get("https://api.github.com/repos/LLOneBot/LLOneBot/releases/latest")
	if err != nil {
		log.Println("检查更新时发生错误，无法获取https://api.github.com/repos/LLOneBot/LLOneBot/releases/latest信息")
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
	log.Println("正在拉取LLOneBot最新版本的仓库...")
	downloadBool := downloadFile(`https://github.com/LLOneBot/LLOneBot/releases/download/`+latestTagName+`/LLOneBot.zip`, "LLOneBot")
	if !downloadBool {
		return false
	}
	log.Println("开始解压到LLOneBot文件夹")
	unZip(filepath.Join(thisDir, "LLOneBot.zip"), "LLOneBot")
	log.Println("解压完成，正在安装 LLOneBot")
	srcPath := filepath.Join(thisDir, `LLOneBot`)
	dstPath := filepath.Join(qqNTPath, `resources`, `app`, `LiteLoaderQQNT-main`, `plugins`, `LLOneBot`)
	log.Println("从", srcPath, "移动到", dstPath)
	log.Println("全新安装LLOneBot")
	err = moveOrCopyFolder(srcPath, dstPath, true)
	if err != nil {
		log.Println("移动文件夹出错", err)
		return false
	}
	log.Println("LLOneBot安装完成")
	return true
}

// 移动文件夹
func moveOrCopyFolder(src, dst string, isMove bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		relPath, err := filepath.Rel(src, path) // 获取文件的相对路径
		if err != nil {
			log.Println("获取文件相对路径出错", err)
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() { // 判断当前信息是否是路径，是则mkdir -p一样的创建所有
			return os.MkdirAll(dstPath, info.Mode())
		} else {
			if isMove {
				err = moveOneFile(path, dstPath) // 如果不是目录，则把文件跨卷移动到目标目录
				if err != nil {
					log.Println("移动文件时出错", err)
					return err
				}
			} else {
				err = copyFile(path, dstPath) // 如果不是目录，则把文件复制过去
				if err != nil {
					log.Println("复制文件时出错", err)
					return err
				}
			}
		}
		return nil
	})
}

// 跨卷移动 oldPath源目录 newPath目标目录(仅适用于Windows系统)
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

// 将LiteLoaderQQNT_bak中的插件和插件保存的数据都复制到LiteLoaderQQNT-main里面
func copyOldFiles(filePath string) bool {
	oldPluginsPath := filepath.Join(filePath, "resources", "app", "LiteLoaderQQNT_bak", "plugins")
	newLiteLoaderPath := filepath.Join(filePath, "resources", "app", "LiteLoaderQQNT-main")
	// 复制 LiteLoader_bak 中的插件到新的 LiteLoader 目录
	if isFileExists(oldPluginsPath) {
		err := moveOrCopyFolder(oldPluginsPath, filepath.Join(newLiteLoaderPath, "plugins"), false)
		if err != nil {
			log.Println(err)
			return false
		}
		log.Println("已将 LiteLoader_bak 中旧插件 Plugins 复制到新的 LiteLoader 目录")
		oldConfigPath := filepath.Join(filePath, "resources", "app", "LiteLoaderQQNT_bak")
		// 复制 LiteLoader_bak 中的 config.json 文件到新的 LiteLoader 目录
		err = copyFile(filepath.Join(oldConfigPath, "config.json"), filepath.Join(newLiteLoaderPath, "config.json"))
		if err != nil {
			log.Println("复制 config.json 文件失败：", err)
		} else {
			log.Println("已将 LiteLoader_bak 中旧 config.json 复制到新的 LiteLoader 目录")
		}
	}

	// 复制 LiteLoader_bak 中的数据文件到新的 LiteLoader 目录
	oldDataPath := filepath.Join(filePath, "resources", "app", "LiteLoaderQQNT_bak", "data")
	if isFileExists(oldDataPath) {
		err := moveOrCopyFolder(oldDataPath, filepath.Join(newLiteLoaderPath, "data"), false)
		if err != nil {
			log.Println(err)
			return false
		}
		log.Println("已将 LiteLoader_bak 中旧数据文件 data 复制到新的 LiteLoader 目录")
	}
	return true
}

// 把resources/app/app_launcher/index.js中的内容替换成LiteLoaderQQNT-main的绝对路径，最下面一行不变
func patchIndexJS(filePath string) bool {
	appLauncherPath := filepath.Join(filePath, "resources", "app", "app_launcher")
	err := os.Chdir(appLauncherPath)
	if err != nil {
		log.Println("Error changing directory:", err)
		return false
	}
	log.Println("开始修补 index.js…")
	indexPath := filepath.Join(appLauncherPath, "index.js")
	// 备份原文件
	log.Println("已将旧版文件备份为 index.js.bak")
	bakIndexPath := indexPath + ".bak"
	err = copyFile(indexPath, bakIndexPath)
	if err != nil {
		log.Println("Error backing up file:", err)
		return false
	}
	content := fmt.Sprintf("require('%s');\n", strings.Replace(filepath.Join(filePath, "resources", "app", "LiteLoaderQQNT-main"), string(filepath.Separator), "/", -1))
	content += "require('./launcher.node').load('external_index', module);"
	err = os.WriteFile(indexPath, []byte(content), 0644)
	if err != nil {
		log.Println("Error writing to file:", err)
		return false
	}
	return true
}

// 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
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
		log.Println("检测是否为admin发生错误", err)
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
