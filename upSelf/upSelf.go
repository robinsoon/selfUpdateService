//负责更新自己 2020-04-09 Robin
//update self
//由 selfUpdateClient.exe下载更新包到临时文件夹,调用upSelf.exe后,退出
//upSelf.exe完成文件覆盖,提示已更新至最新版本。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gen2brain/dlgs" //简易对话框库gui备用提示
	"github.com/widuu/goini"
)

var iniconfig string = "selfUpdate.ini"       //配置文件
var webhost string = "http://localhost:8386/" //更新服务器地址 来自ini配置
var appName string = "selfUpdateClient.exe"

func main() {

	//读取配置文件
	conf := goini.SetConfig(iniconfig)
	webhost = conf.GetValue("UpdateTrans", "webhost")
	if webhost != "no value" && webhost != "" {

	} else {
		return
	}
	var exepath, tempdown, bakpath string
	tempdown = "./downpack/" + appName
	exepath = "./" + appName
	bakpath = "./" + appName + ".bak"
	if "" == GetFileName(tempdown) {
		//不存在更新文件
		return
	}

	if "" == GetFileName(exepath) {
		//不存在主程序??
		dlgs.Error("备份警告", "无法执行备份, 文件不存在："+exepath+"\n备份名: "+bakpath+"\n更新可能存在风险！")
		//return
	} else {
		//先备份一下,避免异常导致更新彻底失灵
		Copy(exepath, bakpath) //实际需要替换的文件
	}
	time.Sleep(500000000) //等待0.5秒,显示内容
	//终止进程,不管有没有运行
	//cmd2 := exec.Command("cmd.exe", "/C", "taskkill", "/IM", appName)
	cmd2 := exec.Command("cmd.exe", "/C", "start taskkill /F /IM", appName) //
	//隐藏黑屏执行
	cmd2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	errkill := cmd2.Run() //cmd2.Start()
	if errkill != nil {
		fmt.Println(errkill, appName)
	}
	//fmt.Println("已关闭进程 cmd.exe ", appName)
	time.Sleep(500000000)                 //等待0.5秒,显示内容
	_, copyerr := Copy(tempdown, exepath) //实际需要替换的文件
	//_, copyerr := Copy(tempdown, "./upload/"+copyfile.Ufile.Name) //测试路径
	if copyerr != nil {

		dlgs.Error("文件覆盖失败", "检查到程序文件替换错误\n文件名："+tempdown+" -> "+exepath+"\n"+copyerr.Error())

	} else {
		fmt.Println("覆盖文件:", exepath)
	}

}

//对话框窗口 12种预置交互窗口
func SysmsgBox(title string, content string) string {
	//1.标准消息框  无返回 同类型还有 ：Warning , Error , Info
	_, err := dlgs.Info(title, content)
	if err != nil {
		panic(err)
	}
	return ""
}

func GetFileName(filename string) string {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		//fmt.Println(filename, err)
		return ""
	} else {
		return fileInfo.Name()
	}

}

func Copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
