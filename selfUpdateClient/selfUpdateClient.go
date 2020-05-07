// selfUpdateClient 自更新程序下载/上传文件
// 2020.03.21 by Robin
// 通过http传输文件
// upload目录下的所有文件上传发送到服务器
// 如连接不成功请检查Server端防火墙是否阻止
// downpack 目录存放从服务器download下载的更新文件
// 根据更新包路径备份并替换特定文件
// 2020.04.09 by Robin
// 1.唯一实例
// 2.传递参数
// 3.结束进程
// 4.拷贝文件,自动备份 + upSelf.exe  808KB
// 5.调用并退出
// 6.窗口更新说明webview + upDescWebPage.exe   1.35MB
// 7.兼容性
// 8.当 runexe 不需要时自动结束
// 9.拷贝完成删除 downpack 文件
package main

import (
	//"bytes"
	"encoding/json" //http有关
	"fmt"
	"io"
	"io/ioutil"

	//"log"
	//"mime/multipart"
	"net/http"
	"net/url"
	"os"

	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widuu/goini"
	//"github.com/goini"

	"github.com/getlantern/osversion" //操作系统版本

	//GUI包
	"github.com/andlabs/ui"               //不支持xp的gui库, Vista SP2以上
	_ "github.com/andlabs/ui/winmanifest" //需求组件清单 comctl32.dll 6.0
	"github.com/gen2brain/dlgs"           //简易对话框库gui备用提示
)

var iniconfig string = "selfUpdate.ini"       //配置文件
var webhost string = "http://localhost:8386/" //更新服务器地址 来自ini配置
var debug bool                                //调试模式
var autoRun bool                              //自动运行状态(命令参数调用)
var exeParam string                           //应用传入参数
var exepath string                            //exe
var killtask string                           //杀进程名
var updateFolder []string                     //更新文件夹名 最大10个
var updatePath []string                       //更新文件路径--指向本机
var updateVer []string                        //更新文件版本
var urlGet string                             //请求地址
var sysOSVer string                           //版本号
var sysOSName string                          //OS 名称
var chuiMsg chan string                       //界面消息
var winWidth int = 600
var winHeight int = 360
var openwebpage string //打开新版本说明程序
type FileInfor struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Date     time.Time `json:"date"`
	Type     string    `json:"type"`
	FilePath string    `json:"filepath"`
	Version  string    `json:"version"`
}

type FileDir struct {
	DirName   string      `json:"dirname"`
	FileCount int         `json:"count"`
	Date      time.Time   `json:"date"`
	Memo      string      `json:"memo"`
	DirPath   string      `json:"dirpath"`
	List      []FileInfor `json:"filelist"`
}

//更新任务列表 FileInfor 基础上增加对比
type UpdateTask struct {
	Taskid  int       `json:"taskid"`  //任务号
	Stat    string    `json:"stat"`    //状态 待更新
	UMark   string    `json:"umark"`   //说明 已修改,已过期
	Ufile   FileInfor `json:"ufile"`   //服务器端文件描述
	Locfile FileInfor `json:"locfile"` //本地文件描述
}

func main() {
	//按照命令参数连接
	comargstr := os.Args
	//启动参数
	if len(comargstr) >= 2 {
		for p, parm := range comargstr {
			if p >= 1 {
				exeParam = exeParam + " " + parm
			}
		}
		fmt.Println("Commandline:", exeParam)
		autoRun = true
	} else {

	}
	//设置运行目录
	Chdir()

	//读取配置文件
	conf := goini.SetConfig(iniconfig)
	webhost = conf.GetValue("UpdateTrans", "webhost")
	exepath = conf.GetValue("UpdateTrans", "runexe")
	killtask = GetFileName(exepath) //取文件名
	openwebpage = conf.GetValue("UpdateTrans", "Introduction")
	swidth := conf.GetValue("UpdateTrans", "windowWidth")
	sheight := conf.GetValue("UpdateTrans", "windowHeight")
	if swidth != "no value" && swidth != "" {
		winWidth, _ = strconv.Atoi(swidth)
		if winWidth <= 0 {
			winWidth = 600
		}
	}
	if sheight != "no value" && sheight != "" {
		winHeight, _ = strconv.Atoi(sheight)
		if winHeight <= 0 {
			winHeight = 360
		}
	}
	if exepath == "no value" || exepath == "" {
		exepath = ""
		killtask = ""
		autoRun = true
	}
	if webhost != "no value" && webhost != "" {

		fmt.Println("loading ... ", iniconfig, " Server@> ", webhost)
		//读取托管文件夹
		var iniFolder, iniPath, iniVer, foldername, pathname, fver string
		for i := 1; i <= 10; i++ {
			iniFolder = "UPDATE" + strconv.Itoa(i)
			iniPath = "LOCAL_PATH" + strconv.Itoa(i)
			iniVer = "LOCAL_VER" + strconv.Itoa(i)

			foldername = conf.GetValue("EXEPATH", iniFolder)
			pathname = conf.GetValue("EXEPATH", iniPath)
			fver = conf.GetValue("EXEPATH", iniVer)
			if foldername == "no value" || foldername == "" {
				continue
			}
			if pathname == "no value" || pathname == "" {
				continue
			}
			updateFolder = append(updateFolder, foldername) //向其中添加元素
			updatePath = append(updatePath, pathname)       //向其中添加元素
			updateVer = append(updateVer, fver)             //向其中添加元素 允许为空
			//"Add update list : ",
			fmt.Println(i, "[ ", foldername, " ] ", pathname, fver)
		}
		if "1" == conf.GetValue("UpdateTrans", "DEBUG") {
			debug = true
			//fmt.Println("开启 DEBUG 模式")
		} else {
			debug = false
		}
	} else {
		fmt.Println("no ini ", iniconfig, " 配置文件读取失败！请检查配置...")
		SysmsgBox("初始化错误",
			iniconfig+" 配置文件读取失败！请检查配置...")
		return
	}

	defer func() { // 必须要先声明defer，否则不能捕获到panic异常
		if err := recover(); err != nil {
			fmt.Println(err) // 这里的err其实就是panic传入的内容
			var errmsg string
			errmsg = fmt.Sprintf(" %v", err)
			if errmsg == " assignment to entry in nil map" {
				SysmsgBox("读取配置文件错误", "无法读取配置ini文件\n可能编码问题，导致配置节无法识别\n"+errmsg+"\n")
			} else {
				SysmsgBox("运行错误",
					"原因："+errmsg)
			}

		}
	}()

	//系统版本 XP系统不支持
	sysOSName, sysOSVer = sysver()
	if sysOSVer != "" {
		//处理兼容问题 要求comctl32.dll v6.0
		if sysOSVer[:2] == "5." {
			//XP 2003
			SysmsgBox("运行提示", sysOSName+"\n版本："+sysOSVer+"\n您的操作系统版本过时，\n窗口要求组件 comctl32.dll v6.0 以上,\n请注册新版本或更新操作系统。")

		} else if sysOSVer[:2] == "6." {
			//Vista 7 8 Server2008
			//SysmsgBox("运行提示", sysOSName+"\n版本："+sysOSVer+"\n您的操作系统版本较低，\n为保证使用体验，建议更新操作系统。")

		} else if sysOSVer[:3] == "10." {
			//10  Server2016
		}
	}

	//打开列表界面
	chuiMsg = make(chan string)
	go openWindow()

	//请求：filelist?target=source
	//开始执行请求
	//var resp0 *http.Response
	var err0 error
	urlGet = webhost + "filelist?target=source"
	//urlGet = webhost + "filelist?target=selfupdate"
	fmt.Println("Get请求地址 ", urlGet)
	var filejson FileDir //更新source
	var selfjson FileDir //自更新exe
	err0 = Getjson(urlGet, &filejson)

	if err0 != nil {
		fmt.Println("网络连接错误", urlGet)
		uiMultiline1.Append("\n网络连接错误, 请求无响应 " + urlGet)
		uiMultiline1.Append("\n更新服务维护中, 您可能无法获取最新版本，请咨询技术人员 ")

		SysmsgBox("网络连接错误",
			"原因：请求无响应 "+urlGet+"\n更新服务已暂停, 即将启动托管应用程序\n"+exepath)
		RunEXE(exepath, exeParam)

		os.Exit(0)
	}

	urlGet = webhost + "filelist?target=selfupdate"
	err0 = Getjson(urlGet, &selfjson)

	if err0 != nil {
		fmt.Println("网络请求中断", urlGet)
		SysmsgBox("网络连接错误",
			"原因：请求无响应 "+urlGet)
		os.Exit(0)
	}

	fmt.Println("已获取文件清单", filejson.DirName, filejson.DirPath, filejson.FileCount, "个文件")
	fmt.Println("已获取自更新", selfjson.DirName, selfjson.DirPath, selfjson.FileCount, "个文件")

	//SysmsgBox("已获取文件清单", strconv.Itoa(filejson.FileCount)+"个文件")
	if uiMultiline1 == nil {
		//窗口还未创建好,等待就绪再输出文本
		time.Sleep(200000000) //等待0.2秒,显示内容,避免通信过快导致空指针问题
	}

	//fmt.Println("日志消息对象", uiMultiline1.Text())
	uiMultiline1.Append("\n请求地址 " + urlGet)
	uiMultiline1.Append("\n已获取更新列表 " + strconv.Itoa(filejson.FileCount) + " 个文件:")
	uiPrograss1.SetValue(50)
	//fmt.Println("已显示日志信息")
	var newlist []string
	var newitem string
	for fs, ifile := range filejson.List {
		fvsize := float64(ifile.Size) / 1024
		newitem = fmt.Sprintf("%d. %s\t  %.2fKB", fs+1, ifile.Name, fvsize)
		newlist = append(newlist, newitem)
		uiMultiline1.Append("\n" + newitem + " 版本 " + ifile.Version)
	}

	// _, _, err := dlgs.List("下载更新文件", "新版本清单:"+strconv.Itoa(filejson.FileCount)+"个文件", newlist)
	// if err != nil {
	// 	fmt.Println("文件清单", err)
	// }

	// //检查文件状态
	// var searchlocal string
	// var scp int
	// for f, ifile := range filejson.List {
	// 	fmt.Println("文件", f+1, ifile.Name, ifile.Size, "Byte", ifile.Date, ifile.FilePath, ifile.Version)
	// 	//文件是否存在
	// 	searchlocal, scp = SearchFile(updateFolder, ifile.FilePath)
	// 	fmt.Println("查找文件", ifile.FilePath, " 在本地目录 ", searchlocal)
	// 	searchlocal = updatePath[scp] + "\\" + ifile.Name

	// 	if "" == GetFileName(searchlocal) {
	// 		fmt.Println("        ", scp, "不存在文件", searchlocal)
	// 	} else {
	// 		localsize := GetFileSize(searchlocal)
	// 		localtime := GetFileModTime(searchlocal)
	// 		fmt.Println("        ", scp, "已找到文件", searchlocal, localsize, "Byte", localtime, updateVer[scp])
	// 		fmt.Println("        ", f+1, "服务器文件", ifile.Name, ifile.Size, "Byte", ifile.Date, ifile.Version)
	// 		//比较差异,记录待更新文件清单
	// 		if ifile.Date.After(localtime) {
	// 			fmt.Println("        ", searchlocal, "文件已过期需要下载更新")
	// 		}
	// 		if localsize != ifile.Size {
	// 			fmt.Println("        ", searchlocal, "文件有修改需要下载更新")
	// 		}

	// 	}
	// }

	var urldown, tempdown string
	var totalsize int64 = 0
	//更新任务清单
	tasklist := CreateUpdateTask(filejson)
	if len(tasklist) == 0 {
		uiMultiline1.Append("\n检查更新：已是最新版 ！")
	} else {
		//有更新时,显示说明
		// if openwebpage != "" {
		// 	RunEXE(openwebpage, "")
		// 	uiMultiline1.Append("\n打开更新说明页面")
		// }
	}

	for _, iself := range selfjson.List {
		//fmt.Println("自更新", iself.Name, iself.Size, "Byte", iself.Date, iself.FilePath, iself.Version)

		flsize := float64(iself.Size) / 1024
		newitem = fmt.Sprintf(" %s\t  %.2fKB", iself.Name, flsize)
		newlist = append(newlist, newitem)
		uiMultiline1.Append("\n检查自更新程序 " + newitem + " 版本 " + iself.Version)
	}
	//自更新程序是否需要替换
	selftasklist := CreateSelfTask(selfjson)
	if len(selftasklist) > 0 {
		uiMultiline1.Append("\n有新的更新模块 , 立即重启服务！")
		for selftsk, selftask := range selftasklist {
			urldown = webhost + "download?file=" + selftask.Ufile.FilePath
			tempdown = "./downpack/" + selftask.Ufile.Name

			DownloadFile(urldown, tempdown)

			uiMultiline1.Append("\n已下载更新模块:" + strconv.Itoa(selftsk) + " " + selftask.Ufile.Name)

		}
		//下载后退出调用 upSelf.exe
		RunEXE("upSelf.exe", "")
		//os.Exit(0) //退出
	} else {
		uiMultiline1.Append("\n自更新程序已就绪")
	}
	uiPrograss1.SetValue(70)
	itms := time.Now()
	//fmt.Println("Start Transing", itms.Format("2006-01-02 03:04:05"))
	for tsk, task := range tasklist {
		fmt.Println("更新任务", tsk+1, task.Stat, task.Locfile.Name, task.Ufile.Size, "Byte", task.UMark, task.Ufile.Date, task.Ufile.FilePath, "->", task.Locfile.FilePath, task.Locfile.Version, task.Ufile.Version)
		//下载更新
		param := url.QueryEscape(task.Ufile.FilePath)

		urldown = webhost + "download?file=" + param
		tempdown = "./downpack/" + task.Ufile.Name
		//tempdown = strings.Replace(tempdown, " ", "%20", -1)
		totalsize += task.Ufile.Size
		downerr := DownloadFile(urldown, tempdown)
		dsize := int(GetFileSize(tempdown))
		if downerr != nil {
			fmt.Println("文件下载失败", downerr, urldown, tempdown)
			tasklist[tsk].Stat = "下载失败"
			tasklist[tsk].UMark = downerr.Error()
			uiMultiline1.Append("\n下载失败 " + strconv.Itoa(tsk) + " " + task.Locfile.Name)
		} else {
			//fmt.Println("文件下载成功", tempdown)
			tasklist[tsk].Stat = "下载完成"
			tasklist[tsk].UMark = "准备更新"
			uiMultiline1.Append("\n下载完成 " + strconv.Itoa(tsk) + " " + task.Locfile.Name + " " + strconv.Itoa(dsize) + "B")
			//os.Rename(tempdown, strings.Replace(tempdown, "%20", " ", -1))
		}
	}

	itme := time.Now()
	ms1 := (itme.UnixNano() - itms.UnixNano()) / 1e6
	fvalue := float64(totalsize) / 1024 / 1024
	if len(tasklist) == 0 {
		fmt.Printf("> 已就绪,无需更新\n")
		PutResult(webhost + "result?act=0")
		uiMultiline1.Append("\n> 已就绪,无需更新\n")
		//可以关闭了

	} else {
		strresult := fmt.Sprintf("> %d 个文件已下载,耗时：%v ms 合计大小: %.2f MB\n", len(tasklist), NumberFormat(strconv.FormatInt(ms1, 10)), fvalue)
		fmtstr := fmt.Sprintf("result?act=%d&time=%v&size=%.2f", len(tasklist), NumberFormat(strconv.FormatInt(ms1, 10)), fvalue)
		PutResult(webhost + fmtstr)
		uiMultiline1.Append("\n" + strresult + "\n")
	}
	//time.Sleep(500000000) //等待0.5秒,显示内容
	//替换前检查是否有进程驻留内存
	uiPrograss1.SetValue(80)
	uiMultiline1.Append("\n检查运行环境:" + killtask)
	if killtask == "" {

	} else {
		ibexist, isname, ipid := isProcessExist(killtask)
		if len(tasklist) > 0 {
			//有更新时,显示说明
			if openwebpage != "" {
				RunEXE(openwebpage, "")
				uiMultiline1.Append("\n打开更新说明页面")
			}
		}

		if ibexist && len(tasklist) > 0 {
			//SysmsgBox("更新提示", "检查到程序正在运行\n应用程序名："+killtask+" 有新的版本\n请提前保存数据，系统准备强制退出以完成更新。")
			uiMultiline1.Append("\n检查到程序 " + killtask + " 正在运行，先关闭再更新")

			ui.MsgBox(uiWindow1, "更新服务提示", "检查到程序正在运行，立即关闭吗？\n应用程序名："+killtask+" 有新的版本\n请提前保存数据，系统准备强制退出以完成更新。")
			yesno := true
			//dlgs.Question 无法前置窗口
			//yesno, _ := dlgs.Question("更新服务提示", "检查到程序正在运行，立即关闭吗？\n应用程序名："+killtask+" 有新的版本\n请提前保存数据，系统准备强制退出以完成更新。", false)
			if yesno == true {
				//同意退出
				//cmd2 := exec.Command("cmd.exe", "/C", "taskkill", "/IM", killtask)
				cmd2 := exec.Command("cmd.exe", "/C", "start taskkill /F /IM", killtask) //
				//隐藏黑屏执行
				cmd2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				//errkill := cmd2.Start()
				errkill := cmd2.Run()
				if errkill != nil {
					fmt.Println(errkill, isname, ipid)
					uiMultiline1.Append("\n无法关闭进程:" + killtask + errkill.Error())
				} else {
					//fmt.Println("已关闭进程 cmd.exe ", ipid, cmd2)
					uiMultiline1.Append("\n已关闭进程:" + killtask)
				}

			}

		} else {
			//无进程
			uiMultiline1.Append("\n已就绪:" + killtask)
		}
	}
	//taskself := CreateTask(selfjson)
	//CreateTask(searchlocal,selfjson.List)
	uiPrograss1.SetValue(90)
	//替换文件
	for tcopy, copyfile := range tasklist {
		if copyfile.Stat == "下载完成" {
			tempdown = "./downpack/" + copyfile.Ufile.Name //缓存路径
			//_, copyerr := Copy(tempdown, copyfile.Locfile.FilePath) //实际需要替换的文件
			_, copyerr := ClipPaste(tempdown, copyfile.Locfile.FilePath) //实际需要替换的文件
			//_, copyerr := Copy(tempdown, "./upload/"+copyfile.Ufile.Name) //测试路径
			if copyerr != nil {
				fmt.Println(tcopy+1, "文件复制失败", tempdown, "->", copyfile.Locfile.FilePath)
				dlgs.Error("文件覆盖失败", "检查到程序文件替换错误\n文件名："+tempdown+" -> "+copyfile.Locfile.FilePath)
				uiMultiline1.Append("\n覆盖失败:" + strconv.Itoa(tcopy+1) + "  " + tempdown + " -> " + copyfile.Locfile.FilePath)

			} else {
				fmt.Println(tcopy+1, "覆盖文件:", copyfile.Locfile.FilePath)
				uiMultiline1.Append("\n已覆盖文件:" + strconv.Itoa(tcopy+1) + "  " + copyfile.Locfile.FilePath)
			}

		}
	}
	//启动exe
	//结束
	// var inputs string
	// fmt.Println("请输入回车键结束：")
	// fmt.Scanln(&inputs)
	uiPrograss1.SetValue(100)
	uiBtn1.SetText("  完成  ")

	if exeParam != "" {
		exeParam = " " + exeParam
	}
	if exepath != "" {
		cmd := exec.Command("cmd.exe", "/C", "start "+exepath+exeParam)
		//隐藏黑屏执行
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		//file, _ := exec.LookPath(os.Args[0])
		//path, _ := filepath.Abs(file)
		//index := strings.LastIndex(path, string(os.PathSeparator))
		//cmd.Dir = `D:\Workspace\MIS\CMIS8\BIN`
		uiMultiline1.Append("\n准备启动程序:" + exepath + exeParam)
		fmt.Println(cmd)

		if err := cmd.Run(); err != nil {
			fmt.Println("调用病案程序: ", err)
		} else {
			fmt.Println("执行完毕，自动退出:")
			uiMultiline1.Append("\n已执行完成!")
			if debug {
				//uiMultiline1.Append("\n启动信息:" + "start " + exepath + " " + exeParam)
			} else {
				os.Exit(0) //退出
			}
		}
	} else {
		uiMultiline1.Append("\n执行完成!")
		//os.Exit(0)
	}
	//自动退出
	if autoRun == true && debug != true {
		//写日志记录导出内容
		os.Exit(0)
	}
	//点关闭退出
	result := <-chuiMsg
	fmt.Println(result)
	//var sinput string
	//fmt.Scanf("字符", sinput)
	//释放内存
	close(chuiMsg)
	chuiMsg = nil

	return
}

//请求获取文件清单
func Getjson(url string, getresult *FileDir) error {
	//var getresult FileDir
	//请求：filelist?target=source
	//开始执行请求
	var resp *http.Response
	var err error

	resp, err = http.Get(url)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close() //延时释放,必须在http.Get以后
	body, err1 := ioutil.ReadAll(resp.Body)
	if err1 != nil {
		fmt.Println(err1)
	}
	//fmt.Println("resp=", resp.Header)
	//fmt.Println("BODY=", string(body))

	err = json.Unmarshal(body, &getresult)

	//解析失败会报错，如json字符串格式不对，缺"号，缺}等。
	if err != nil {
		fmt.Println("解析Json错误 Unmarshal ", err)
	}
	return err
}

//发送更新结果
func PutResult(url string) error {
	//请求：result?act=1&time=10&size=1024
	//开始执行请求
	var resp *http.Response
	var err error

	resp, err = http.Get(url)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close() //延时释放,必须在http.Get以后
	_, err1 := ioutil.ReadAll(resp.Body)
	if err1 != nil {
		fmt.Println(err1)
	}
	//fmt.Println("resp=", resp.Header)
	//fmt.Println("BODY=", string(body))

	return err
}

//取文件名
func GetAllFile(pathname string, s []string) ([]string, error) {
	rd, err := ioutil.ReadDir(pathname)
	if err != nil {
		fmt.Println("read dir fail:", err)
		return s, err
	}
	for _, fi := range rd {
		if fi.IsDir() {
			fullDir := pathname + "/" + fi.Name()
			s, err = GetAllFile(fullDir, s)
			if err != nil {
				fmt.Println("read dir fail:", err)
				return s, err
			}
		} else {
			fullName := pathname + "/" + fi.Name()
			s = append(s, fullName)
		}
	}
	return s, nil
}

// Chdir 将程序工作路径修改成程序所在位置
func Chdir() (err error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return
	}

	err = os.Chdir(dir)
	return
}

//取系统OS名称和版本
func sysver() (string, string) {

	sysver, _ := osversion.GetString() //版本号

	sysdisp, _ := osversion.GetHumanReadable() //标识
	fmt.Println("系统：", sysdisp, "版本：", sysver)
	return sysdisp, sysver
}

//对话框窗口 12种预置交互窗口
func SysmsgBox(title string, content string) string {

	var sysMsg string
	//1.标准消息框  无返回 同类型还有 ：Warning , Error , Info
	_, err := dlgs.Info(title, content)
	if err != nil {
		panic(err)
	}

	//2.提问消息框
	// yes, err := dlgs.Question(title, content, true)
	// if err != nil {
	// 	panic(err)
	// }
	// if yes {
	// 	sysMsg = "Y"
	// } else {
	// 	sysMsg = "N"
	// }

	//3.列表选择框
	// sysMsg, _, err := dlgs.List(title, content, []string{"确定", "取消"})
	// if err != nil {
	// 	panic(err)
	// }

	//4.录入框
	// sysMsg, _, err := dlgs.Entry(title, content, "")
	// if err != nil {
	// 	panic(err)
	// }

	fmt.Println("msgbox：", sysMsg)
	return sysMsg
}

func SysmsgFile() {
	_, _, err := dlgs.File("Select file", "", false)
	if err != nil {
		panic(err)
	}
}

func SysmsgFileMulti() {
	_, _, err := dlgs.FileMulti("Select files", "")
	if err != nil {
		panic(err)
	}
}

func SysmsgList() {
	_, _, err := dlgs.List("List", "Select item from list:", []string{"Bug", "New Feature", "Improvement"})
	if err != nil {
		panic(err)
	}
}

func SysmsgListMulti() {
	_, _, err := dlgs.ListMulti("ListMulti", "Select languages from list:", []string{"PHP", "Go", "Python", "Bash"})
	//如果不选择直接点了确定会报异常
	if err != nil {
		panic(err)
	}
}

//格式化数值    1,234,567,898.55
func NumberFormat(str string) string {
	length := len(str)
	if length < 4 {
		return str
	}
	arr := strings.Split(str, ".") //用小数点符号分割字符串,为数组接收
	length1 := len(arr[0])
	if length1 < 4 {
		return str
	}
	count := (length1 - 1) / 3
	for i := 0; i < count; i++ {
		arr[0] = arr[0][:length1-(i+1)*3] + "," + arr[0][length1-(i+1)*3:]
	}
	return strings.Join(arr, ".") //将一系列字符串连接为一个字符串，之间用sep来分隔。
}

//获取文件信息
func GetFileSize(filename string) int64 {
	fileInfo, _ := os.Stat(filename)
	//文件大小
	filesize := fileInfo.Size()
	//fmt.Println(filesize) //返回的是字节
	return filesize
}
func GetFileModTime(filename string) time.Time {
	fileInfo, _ := os.Stat(filename)
	//修改时间
	modTime := fileInfo.ModTime()
	//fmt.Println(modTime)
	return modTime
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
func GetFileInfor(filename string) FileInfor {
	var flinfor FileInfor
	fileInfo, err := os.Stat(filename)
	if err != nil {
		fmt.Println("read fail:", filename)
		flinfor.Version = "error"
		return flinfor
	}
	flinfor.Name = fileInfo.Name()
	flinfor.Date = fileInfo.ModTime()
	flinfor.Size = fileInfo.Size()
	flinfor.Version = "ver1"
	if fileInfo.IsDir() {
		flinfor.Type = "Dir"
	} else {
		flinfor.Type = "File"
	}
	flinfor.FilePath = filename
	return flinfor
}

//从目录中找目标文件对应的本地路径
func SearchFile(pathlist []string, searchfile string) (string, int) {
	//取目录，反向比较 用服务器地址做比较对象,是否包含UPDATE#数组目录名
	//###注意：如果存在目录嵌套,要求下级目录(子目录)放在上面,否则不会访问到子目录
	//###注意：符号问题 window下路径符号是 \  而 web地址中路径符号是 /
	//server路径符号需要转换/
	searchfile = strings.Replace(searchfile, "/", "\\", -1)

	//字符串匹配
	for i, pathstr := range pathlist {
		//注意有目录嵌套时，需找到子目录
		if strings.Contains(searchfile, pathstr) {
			return pathstr, i
		}
	}

	return "", 0
}

//比对服务器和本地文件创建任务列表,自动维护目录
func CreateUpdateTask(serverfile FileDir) []UpdateTask {
	var tasklist []UpdateTask //任务清单
	var taskone UpdateTask    //任务
	//检查文件状态
	var searchlocal string
	var scp int
	var localjson FileInfor //本地文件信息--供对比
	for f, ifile := range serverfile.List {
		//fmt.Println("文件", f+1, ifile.Name, ifile.Size, "Byte", ifile.Date, ifile.FilePath, ifile.Version)
		//文件是否存在
		searchlocal, scp = SearchFile(updateFolder, ifile.FilePath)
		//fmt.Println("查找文件", ifile.FilePath, " 在本地目录 ", searchlocal)
		if searchlocal != "" {
			searchlocal = updatePath[scp] + "\\" + ifile.Name
			//如果文件夹不存在则创建
			_, iscreated := PathnoFoundCreate(ifile.FilePath, updatePath[scp], ifile.Name)
			if iscreated {
				fmt.Println("警告", "已重建目录:", searchlocal)
			}
		} else {

			//为下载创建新文件夹
			searchlocal, _ = PathnoFoundCreate(ifile.FilePath, updatePath[0], ifile.Name)
			//存在没有配置路径的目录,可下载但会更新到第一个BIN目录
			fmt.Println("警告", "没有配置更新路径:", ifile.FilePath, " 创建目录并重定向到", searchlocal)
		}

		if "" == GetFileName(searchlocal) {
			fmt.Println("        ", f+1, updateFolder[scp], "不存在文件", searchlocal)

			taskone.Taskid = len(tasklist) + 1
			taskone.Stat = "新增"
			taskone.UMark = "文件不存在"

			//本地文件 -- 不存在
			localjson.Name = ifile.Name
			localjson.FilePath = searchlocal
			localjson.Size = 0
			localjson.Version = ""
			localjson.Date = time.Unix(0, 0) //时间置空

			taskone.Locfile = localjson
			taskone.Ufile = ifile
			//新增任务
			tasklist = append(tasklist, taskone)
		} else {
			localsize := GetFileSize(searchlocal)
			localtime := GetFileModTime(searchlocal)
			//fmt.Println("        ", scp, "已找到文件", searchlocal, localsize, "Byte", localtime, updateVer[scp])
			//fmt.Println("        ", f+1, "服务器文件", ifile.Name, ifile.Size, "Byte", ifile.Date, ifile.Version)
			//比较差异,记录待更新文件清单
			if ifile.Date.After(localtime) {
				//fmt.Println("        ", searchlocal, "文件已过期需要下载更新")
				taskone.Taskid = len(tasklist) + 1
				taskone.Stat = "更新"
				taskone.UMark = "文件已过期"

				//本地文件 -- 有最新版
				localjson.Name = ifile.Name
				localjson.FilePath = searchlocal
				localjson.Size = localsize
				localjson.Version = updateVer[scp]
				localjson.Date = localtime //时间

				taskone.Locfile = localjson
				taskone.Ufile = ifile
				//新增任务
				tasklist = append(tasklist, taskone)
			} else if localsize != ifile.Size {
				fmt.Println("        ", f+1, searchlocal, "文件有修改,略过")
				//不按文件大小
			}

		}
	}
	return tasklist
}

//自更新文件,比对服务器和本地更新程序,自动维护目录
func CreateSelfTask(serverfile FileDir) []UpdateTask {
	var tasklist []UpdateTask //任务清单
	var taskone UpdateTask    //任务
	//检查文件状态
	var searchlocal string
	var scp int
	var localjson FileInfor //本地文件信息--供对比
	for f, ifile := range serverfile.List {
		//fmt.Println("文件", f+1, ifile.Name, ifile.Size, "Byte", ifile.Date, ifile.FilePath, ifile.Version)
		//文件是否存在
		searchlocal = "./" + ifile.Name
		//如果文件夹不存在则创建
		// _, iscreated := PathnoFoundCreate(ifile.FilePath, "/", ifile.Name)
		// if iscreated {
		// 	fmt.Println("警告", "已重建目录:", searchlocal)
		// }

		if "" == GetFileName(searchlocal) {
			fmt.Println("        ", f+1, "不存在文件", searchlocal)
			uiMultiline1.Append("\n不存在文件" + searchlocal)
			taskone.Taskid = len(tasklist) + 1
			taskone.Stat = "新增"
			taskone.UMark = "文件不存在"

			//本地文件 -- 不存在
			localjson.Name = ifile.Name
			localjson.FilePath = searchlocal
			localjson.Size = 0
			localjson.Version = ""
			localjson.Date = time.Unix(0, 0) //时间置空

			taskone.Locfile = localjson
			taskone.Ufile = ifile
			//新增任务
			tasklist = append(tasklist, taskone)
		} else {
			localsize := GetFileSize(searchlocal)
			localtime := GetFileModTime(searchlocal)
			uiMultiline1.Append("\n已找到文件" + searchlocal)
			//fmt.Println("        ", scp, "已找到文件", searchlocal, localsize, "Byte", localtime, updateVer[scp])
			//fmt.Println("        ", f+1, "服务器文件", ifile.Name, ifile.Size, "Byte", ifile.Date, ifile.Version)
			//比较差异,记录待更新文件清单
			if ifile.Date.After(localtime) {
				//fmt.Println("        ", searchlocal, "文件已过期需要下载更新")
				taskone.Taskid = len(tasklist) + 1
				taskone.Stat = "更新"
				taskone.UMark = "文件已过期"

				//本地文件 -- 有最新版
				localjson.Name = ifile.Name
				localjson.FilePath = searchlocal
				localjson.Size = localsize
				localjson.Version = updateVer[scp]
				localjson.Date = localtime //时间

				taskone.Locfile = localjson
				taskone.Ufile = ifile
				//新增任务
				tasklist = append(tasklist, taskone)
			} else if localsize != ifile.Size {
				fmt.Println("        ", f+1, searchlocal, "文件有修改,略过")
				//不按文件大小
				taskone.Taskid = len(tasklist) + 1
				taskone.Stat = "更新"
				taskone.UMark = "文件不一致"

				//本地文件 -- 有最新版
				localjson.Name = ifile.Name
				localjson.FilePath = searchlocal
				localjson.Size = localsize
				localjson.Version = updateVer[scp]
				localjson.Date = localtime //时间

				taskone.Locfile = localjson
				taskone.Ufile = ifile
				//新增任务
				tasklist = append(tasklist, taskone)
			}

		}
	}
	return tasklist
}

//获取source的子串,如果start小于0或者end大于source长度则返回""
//start:开始index，从0开始，包括0
//slen:截取长度
func substring(source string, start int, slen int) string {
	var end int
	var r = []rune(source)
	length := len(r)

	if start+slen > length {
		end = length
	} else {
		end = start + slen
	}

	if start < 0 || end > length || start > end {
		return ""
	}

	if start == 0 && end == length {
		return source
	}

	return string(r[start:end])
}

//下载文件
func DownloadFile(urls string, pathname string) error {
	itms := time.Now()
	//fmt.Println("Start Transing", itms.Format("2006-01-02 03:04:05"))
	//替换文件名中空格为%20
	//urls = strings.Replace(urls, "%", "%25", -1)
	//urls = strings.Replace(urls, " ", "%20", -1)
	//urls = strings.Replace(urls, "&", "%26", -1)
	//urls = strings.Replace(urls, "?", "%3F", -1)

	res, err := http.Get(urls)
	if err != nil {
		//panic(err)
		//fmt.Println(err.Error())
	}

	defer res.Body.Close() //延时释放,必须在http.Get以后
	newfile, err := os.Create(pathname)
	if err != nil {
		panic(err)
		fmt.Println(err.Error())
	}
	defer newfile.Close()
	_, err = io.Copy(newfile, res.Body)
	if err != nil {
		fmt.Println(err.Error())
	}

	itme := time.Now()
	ms1 := (itme.UnixNano() - itms.UnixNano()) / 1e6
	fmt.Printf(pathname+"下载耗时：%v ms\n", NumberFormat(strconv.FormatInt(ms1, 10)))
	return err
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

//剪贴
func ClipPaste(src, dst string) (int64, error) {

	nBytes, err := Copy(src, dst)
	if err != nil {
		return 0, err
	}
	//删除文件
	errdelete := os.Remove(src)

	if errdelete != nil {
		// 删除失败
	}
	return nBytes, err
}

// 判断文件夹是否存在
// 创建文件夹  err := os.Mkdir("./dir", os.ModePerm)
// 创建多级目录 os.MkdirAll("dir1/dir2/dir3", os.ModePerm)
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func PathnoFoundCreate(serverpath string, updatepath string, filename string) (string, bool) {
	//为下载创建新文件夹
	var searchlocal string
	newpath := path.Dir(serverpath)
	dirname := path.Base(newpath)
	olddir := updatepath
	if path.IsAbs(olddir) {
		//绝对路径 路径符号不是/而是\\
	}
	//local路径符号需要转换/
	olddir = strings.Replace(olddir, "\\", "/", -1)

	olddir = path.Dir(olddir)
	newdir := path.Join(olddir, dirname)

	newdir = strings.Replace(newdir, "/", "\\", -1) //转回来
	isExt, _ := PathExists(newdir)
	if !isExt {
		os.MkdirAll(newdir, os.ModePerm)
		searchlocal = newdir + "\\" + filename
		//fmt.Println("        创建文件夹：", newdir, searchlocal)
		//存在没有配置路径的目录,可下载但会更新到第一个BIN目录
		//fmt.Println("警告", "没有配置更新路径:", serverpath, " 创建目录并重定向到", searchlocal)
		return searchlocal, true
	} else {
		//已存在
		searchlocal = newdir + "\\" + filename
		//fmt.Println("警告", "没有配置更新路径:", serverpath, " 已重定向到", searchlocal)
		return searchlocal, false
	}

}

//调用外部exe程序
func RunEXE(exepath1 string, exeParam1 string) {
	if exeParam1 != "" {
		exeParam1 = " " + exeParam1
	}
	if exepath1 == "" {
		return
	}
	cmd := exec.Command("cmd.exe", "/C", "start "+exepath1+exeParam1)
	//隐藏黑屏执行
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	//file, _ := exec.LookPath(os.Args[0])
	//path, _ := filepath.Abs(file)
	//index := strings.LastIndex(path, string(os.PathSeparator))
	//cmd.Dir = `D:\Workspace\MIS\CMIS8\BIN`
	//uiMultiline1.Append("\n准备启动程序:" + exepath1 + exeParam1)
	fmt.Println(cmd)
	if err := cmd.Run(); err != nil {
		fmt.Println("调用病案程序: ", err)
	} else {
		fmt.Println("执行完毕，自动退出:")
		if debug {
			//uiMultiline1.Append("\n启动信息:" + "start " + exepath1 + " " + exeParam1)
		} else {
			os.Exit(0) //退出
		}
	}
}

//检查进程是否存在,提示退出
func isProcessExist(appName string) (bool, string, int) {
	appary := make(map[string]int)
	cmd := exec.Command("cmd", "/C", "tasklist")
	//隐藏黑屏执行
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, _ := cmd.Output()
	//fmt.Printf("fields: %v\n", output)
	n := strings.Index(string(output), "System")
	if n == -1 {
		//fmt.Println("tasklist no find")
		//os.Exit(1)
		return false, appName, -1
	}
	data := string(output)[n:]
	fields := strings.Fields(data)
	for k, v := range fields {
		if v == appName {
			appary[appName], _ = strconv.Atoi(fields[k+1])
			//fmt.Println("tasklist find :", appName, appary[appName])
			return true, appName, appary[appName]
		}
	}

	return false, appName, -1
}

//ui部分
//打开窗口，显示任务列表界面
func openWindow() {
	err := ui.Main(setupUI)
	if err != nil {
		chuiMsg <- "quit:selfUpdateClient"
		panic(err)
	}
	chuiMsg <- "quit:selfUpdateClient"
}

func setupUI() {
	// 定义图元
	var titlestring string
	titlestring = "自动更新 v1.1"
	if autoRun == true || debug == true {
		titlestring = "自动更新 v1.1  author: Robin (调试模式)"
	}

	//input := ui.NewLabel(titlestring) //ui.NewEntry()
	input := ui.NewMultilineEntry() //多行文本框
	input.SetText(titlestring)

	uiMultiline1 = input
	processbar := ui.NewProgressBar()
	processbar.SetValue(-1)
	uiPrograss1 = processbar

	container2 := ui.NewGroup("获取新版本")
	container2.SetChild(processbar)
	//------垂直排列的容器---------
	div := ui.NewVerticalBox()
	//------水平排列的容器
	// boxs_1 := ui.NewHorizontalBox()
	// boxs_1.Append(container1, true)
	// boxs_1.SetPadded(true)

	boxs_2 := ui.NewHorizontalBox()
	boxs_2.Append(container2, true)
	boxs_2.SetPadded(true)

	btnQuit := ui.NewButton("  停止  ")
	entry := ui.NewEntry()
	entry.SetReadOnly(true)
	//按钮事件  停止
	btnQuit.OnClicked(func(*ui.Button) {
		if 0 > processbar.Value() {
			processbar.SetValue(0)

			btnQuit.SetText("  更新  ")
		} else if 100 == processbar.Value() {
			ui.Quit()
		} else {
			processbar.SetValue(-1)

			btnQuit.SetText("  停止  ")
		}

	})
	boxs_2.Append(btnQuit, false)
	uiBtn1 = btnQuit
	//组合 从上到下一次布局,true 弹性高度
	//div.Append(boxs_1, true)

	div.Append(boxs_2, false)
	div.Append(input, true) //false)  //true 弹性高度和宽度
	//infotext := ui.NewLabel(sysOSName + " " + sysOSVer)
	//div.Append(infotext, false)
	div.SetPadded(true)

	//创建窗口
	window := ui.NewWindow("自动更新服务 -[ "+sysOSName+"] 版本 "+sysOSVer, winWidth, winHeight, true)
	window.SetChild(div)
	uiWindow1 = window
	window.SetMargined(true) //窗口边框

	window.OnClosing(func(*ui.Window) bool {
		ui.Quit()
		return true
	})

	ui.OnShouldQuit(func() bool {
		window.Destroy()
		return true
	})
	//显示窗口
	window.Show()
	uiMultiline1.Append("\n" + sysOSName + " 版本 " + sysOSVer)
	if autoRun == true || debug == true {
		//time.Sleep(500000000) //等待0.5秒,显示内容
	} else {
		uiMultiline1.Append("\n获取更新中... ")

	}

}

//ui与任务交互
var uiPrograss1 *ui.ProgressBar
var uiBtn1 *ui.Button
var uiWindow1 *ui.Window
var uiMultiline1 *ui.MultilineEntry
