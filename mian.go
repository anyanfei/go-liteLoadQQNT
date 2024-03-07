package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/sys/windows"
)

const (
	thisAppVersion = "1.0" // 当前版本号

	proxyUrl = "https://mirror.ghproxy.com/" // 存储反代服务器的URL
)

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
	if !checkForUpdate() {
		log.Println("检测失败")
		return
	}

}

// 查看是否有新版本
func checkForUpdate() bool {
	resp, err := getHttpClient(3).Get("https://api.github.com/repos/anyanfei/go-liteLoaderQQNT/releases/latest")
	if err != nil {
		log.Println("检查更新时发生错误，无法获取https://api.github.com/repos/anyanfei/go-liteLoaderQQNT/releases/latest信息")
		return false
	}
	defer resp.Body.Close()
	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return false
	}
	fmt.Println(string(respByte))
	return true
}

//def check_for_updates():
//try:
//# 获取最新版本号
//response = requests.get("https://api.github.com/repos/Mzdyl/LiteLoaderQQNT_Install/releases/latest", timeout=3)
//latest_release = response.json()
//tag_name = latest_release['tag_name']
//body = latest_release['body']
//if tag_name > current_version:
//print(f"发现新版本 {tag_name}！")
//print(f"更新日志：\n ")
//console = Console()
//markdown = Markdown(body)
//console.print(markdown)
//download_url = (
//f"https://github.com/Mzdyl/LiteLoaderQQNT_Install/releases/download/{tag_name}/install_windows.exe")
//# urllib.request.urlretrieve(download_url, f"install_windows-{tag_name}.exe")
//download_file(download_url, f"install_windows-{tag_name}.exe", PROXY_URL)
//print("版本号已更新。")
//print("请重新运行脚本。")
//sys.exit(0)
//else:
//print("当前已是最新版本，开始安装。")
//except Exception as e:
//print(f"检查更新阶段发生错误: {e}")

// 是否可以连接代理的github，能则返回true，反之false
func canConnectToGithubWithProxy() bool {
	resp, err := getHttpClient(5).Get(proxyUrl + "https://github.com/Mzdyl/LiteLoaderQQNT_Install/releases/download/1.9/install_windows.exe")
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println(resp.StatusCode)
	return resp.StatusCode == http.StatusOK
}

func downloadFile(needDownloadUrl, fileName string) bool {
	if canConnectToGithubWithProxy() {
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
