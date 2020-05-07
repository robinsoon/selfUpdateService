// selfUpdateServer 自更新程序--服务器
// 2020.03.21 by Robin
// 文件存储到目录 upload 要求存在文件夹
// 根据客户端请求将upload中的文件发送给客户端
// md页面显示更新内容
// 获取文件列表

package main

import (
	//"encoding/json"
	//"encoding/xml" //处理xml结果
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"

	//"github.com/kataras/iris/v12/cache"
	"archive/zip"
	"path/filepath"

	"github.com/widuu/goini"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
)

var upcount int
var markdownFile string = "readme.md"
var iport int = 8386
var iniconfig string = "Serverconfig.ini"
var iniversion string
var iniverbin string
var iniverupdate string

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

//信源中间件
func timeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//处理请求前
		timeStart := time.Now()
		// next handler interface
		next.ServeHTTP(w, r)
		//路由处理之后
		timeElapsed := time.Since(timeStart)
		fmt.Println(timeElapsed)
	})
}

//不应在包含动态数据的处理程序上使用缓存。
//缓存是静态内容的一个好的和必须的功能，即“关于页面”或整个博客网站，静态网站非常适合。
func writeMarkdown(ctx iris.Context) {
	// 服务清单
	println("Service List. MarkDown Content :" + markdownFile)

	markdownContents, err := ioutil.ReadFile(markdownFile)
	if err == nil {
		//处理输出
		ctx.Application().Logger().Infof("#IP=%s  ViewMarkdown / ", GetClientIP(ctx))
		unsafe := blackfriday.MarkdownCommon(markdownContents) //markdown转为html
		html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)   //安全性过滤
		ctx.HTML(string(html))

	} else {
		ctx.Application().Logger().Errorf("#IP=%s  ViewMarkdown / %s 文件读取失败！", GetClientIP(ctx), markdownFile)
		println(markdownFile + " 文件读取失败！")
	}

}

//下载文件 支持zip
func downloads(ctx iris.Context) {
	//指定下载某一文件
	param := ctx.URLParam("file")
	//处理文件名
	param, _ = url.QueryUnescape(param)
	//param = strings.Replace(param, "%20", " ", -1)
	if param != "" {
		fmt.Println(">请求下载文件：" + param)
		lsName := GetFileName(param)
		ctx.Application().Logger().Infof("#IP=%s  Get /download?file=  %s ", GetClientIP(ctx), param)
		if lsName == "" {
			ctx.JSON(iris.Map{"message": param + " 文件不存在！"})
		} else {
			ctx.SendFile(param, lsName)
		}
		return
	}

	fmt.Println(">请求下载文件夹：source -all")
	listjson := GetAllFilesJson("./source")
	//ctx.JSON(listjson)
	for i, file := range listjson {
		//ctx.SendFile(file.FilePath, file.Name)
		fmt.Println(">>>>已下载文件：", i, file.FilePath, file.Size)
	}
	//压缩文件  --可以使用未过期的文件避免重复压缩
	file := "./source.zip"
	search := GetFileName(file)
	if search != "" {
		//不存在 或 在有效期
		mdtime := GetFileModTime(file)
		nowtime := time.Now()
		dt := nowtime.Sub(mdtime).Minutes() //两个时间相减
		//ms1 := (nowtime.UnixNano() - mdtime.UnixNano()) / 1e6
		//fmt.Println("   文件时效1：" + strconv.FormatInt(ms1+1, 10) + "ms")
		fmt.Println("   "+file+"文件时效：", dt, "分钟")
		if dt > 19 {
			fmt.Println(">>>>重新压缩文件：", file, "  距离上次压缩 ", dt, "分钟")
			Zip("./source", "./source.zip")
		}

	} else {
		fmt.Println(">>>>创建压缩文件：", file)
		Zip("./source", "./source.zip")
	}
	ctx.Application().Logger().Infof("#IP=%s  Get /download all   %s  IP=%s", GetClientIP(ctx), "update.zip")
	derr := ctx.SendFile(file, "update.zip")
	if derr != nil {
		fmt.Println(file+"文件未下载：", derr)
		ctx.Application().Logger().Errorf("#IP=%s  Get /download   %s 文件未下载！", GetClientIP(ctx), file)
	}

}

func main() {
	//读取配置文件
	conf := goini.SetConfig(iniconfig)
	iniPort := conf.GetValue("UpdateServer", "webport")
	if iniPort != "no value" && iniPort != "" {
		iport, _ = strconv.Atoi(iniPort)
		fmt.Println("set Server port @", iport, iniconfig)
	} else {
		iport = 8386
		fmt.Println("no ini ,Default port @", iport)
	}

	iniversion = conf.GetValue("UpdateServer", "Version")
	iniverbin = conf.GetValue("UpdateServer", "VerBIN")
	iniverupdate = conf.GetValue("UpdateServer", "VerUpdate")
	//路由表处理规则
	//http.HandleFunc("/", helloHandler)

	//http.HandleFunc("/upload", uploadHandler)
	//使用中间件在业务前后处理一些基础控制
	//http.Handle("/upload", timeMiddleware(http.HandlerFunc(uploadHandler)))
	//http.Handle("/download", timeMiddleware(http.HandlerFunc(downloadHandler)))
	logname := TodayFilename()
	flog, errflog := os.OpenFile(logname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if errflog != nil {
		fmt.Println("Create new ", logname, errflog.Error())
		flog, _ = os.Create(logname)
	}
	defer flog.Close()

	app := iris.New()
	app.Logger().SetLevel("info") //("debug")
	app.Logger().SetOutput(io.MultiWriter(flog, os.Stdout))
	app.Use(recover.New())
	app.Use(logger.New())
	//默认通知关闭服务
	// iris.RegisterOnInterrupt(func() {
	// 	timeout := 5 * time.Second
	// 	ctx, cancel := iris.Context.WithTimeout(iris.Context.Background(), timeout)
	// 	defer cancel()
	// 	//关闭所有主机
	// 	app.Shutdown(ctx)
	// })
	// app.Get("/", func(ctx iris.Context) {
	// 	file := "./files/first.zip"
	// 	ctx.SendFile(file, "c.zip")
	// })

	//服务列表
	app.Get("/", writeMarkdown)

	//了解更多
	app.Get("/about", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"message": "SelfUpdateService Iris Frame!"})
	})
	//checkver 检查版本接口
	app.Get("/checkver", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"message": "检查版本接口 checkver"})
	})

	//filelist 获取文件目录接口
	app.Get("/filelist", func(ctx iris.Context) {
		//source / selfupdate
		//获取get参数
		path := ctx.Path()
		param := ctx.URLParam("target")
		switch param {
		case "source":
			fmt.Println("更新目录：" + path + " " + param)
		case "selfupdate":
			fmt.Println("自更新目录：" + param)
		default:
			fmt.Println("获取目录：" + param)
		}
		//dirjson := GetDirJson("./" + param)
		dirjson := GetFilelistJson("./" + param)

		ctx.Application().Logger().Infof("#IP=%s  Get /filelist  %s /... %d files", GetClientIP(ctx), param, len(dirjson.List))
		//fmt.Println(dirjson)
		//ctx.JSON(iris.Map{"message": "获取文件目录接口 " + path, "参数": param})
		//ctx.JSON(dirjson) //格式化
		ctx.JSON(dirjson, context.JSON{Indent: " "}) //格式化
		//ctx.JSON(dirjson, context.JSON{Indent: " ", UnescapeHTML: true})
	})

	//selfver  自更新接口调用
	app.Get("/selfver", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"message": "selfver  自更新接口调用"})
	})
	//download 下载文件
	app.Get("/download", downloads)
	//download/selfupdate 下载自更新文件
	app.Get("/download/selfupdate", func(ctx iris.Context) {
		//ctx.JSON(iris.Map{"message": "download/selfupdate 下载自更新文件"})
		file := "./selfupdate/selfUpdateClient.exe"
		ctx.Application().Logger().Infof("Get /download  %s", file)
		ctx.SendFile(file, "suclient.exe")
	})
	app.Get("/selfupdate", func(ctx iris.Context) {
		file := "./selfupdate/selfUpdateClient.exe"
		ctx.SendFile(file, "suclient.exe")
	})

	app.Get("/result", func(ctx iris.Context) {
		//获取get参数
		//path := ctx.Path()
		paramact := ctx.URLParam("act")
		if paramact == "0" || paramact == "" {
			ctx.Application().Logger().Infof("#IP=%s  Get /result  > Not updated ", GetClientIP(ctx))
		} else {
			paramtime := ctx.URLParam("time")
			paramsize := ctx.URLParam("size")
			ctx.Application().Logger().Infof("#IP=%s  Get /result  > %s 个文件已下载,耗时：%s ms 合计大小: %s MB", GetClientIP(ctx), paramact, paramtime, paramsize)
		}

		ctx.JSON(iris.Map{"message": "1"})
	})

	//upload  上传
	app.Get("/upload", func(ctx iris.Context) {
		ctx.Application().Logger().Infof("#IP=%s  Get /upload ", GetClientIP(ctx))
		ctx.JSON(iris.Map{"message": "upload  上传"})
	})

	app.Get("/syncfiles", func(ctx iris.Context) {
		ctx.Application().Logger().Infof("#IP=%s  Get /syncfiles ", GetClientIP(ctx))
		ctx.JSON(iris.Map{"message": "syncfiles  推送供下载使用, 移动到source"})
	})

	fmt.Printf("SelfUpdate Server Start Listen @：%d \n", iport)
	itms := time.Now()
	fmt.Println("Start date : ", itms.Format("2006-01-02 03:04:05"))
	fmt.Printf("================================ \n")
	app.Logger().Infof("SelfUpdate Server Start Listen @：%d \n", iport)

	//启动监听服务
	app.Run(iris.Addr(":"+strconv.Itoa(iport)), iris.WithoutServerError(iris.ErrServerClosed))
	//之后的代码不会被执行
	//app.Logger().Error("Update Server Closed！")
}

//取得客户端IP地址
func GetClientIP(ctx iris.Context) string {
	yourip := ctx.RemoteAddr()
	if yourip == "::1" {
		yourip = "127.0.0.1"
	}
	return yourip
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

// 按天生成日志文件
func TodayFilename() string {
	today := time.Now().Format("20060102")
	return "gDs." + today + ".log"
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
		fmt.Println(filename, err)
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
	//版本分支 BIN,selfupdate,其他
	flinfor.Version = iniversion
	if strings.Contains(filename, "BIN") {
		flinfor.Version = iniverbin
	}
	if strings.Contains(filename, "selfupdate") {
		flinfor.Version = iniverupdate
	}

	if fileInfo.IsDir() {
		flinfor.Type = "Dir"
	} else {
		flinfor.Type = "File"
	}
	flinfor.FilePath = filename
	return flinfor
}
func GetDirJson(pathname string) FileDir {
	var dir FileDir
	var list []FileInfor
	var s []string
	var d int
	dir.DirPath = pathname
	fileInfo, errfound := os.Stat(pathname)
	if errfound != nil {
		fmt.Println("read fail:", pathname)
		dir.Memo = "error"
		return dir
	}
	dir.DirName = fileInfo.Name()
	rd, err := ioutil.ReadDir(pathname)
	if err != nil {
		fmt.Println("read dir fail:", err)
		dir.Memo = "read dir fail " + pathname
		return dir
	}

	//提取文件列表
	for i, fi := range rd {
		if fi.IsDir() {
			//文件夹需要再钻取下一层
			fullDir := pathname + "/" + fi.Name()
			fmt.Println("   文件夹", i+1, fullDir)
			d = d + 1
			sub, err2 := ioutil.ReadDir(fullDir)
			if err2 != nil {
				fmt.Println("read dir fail:", err)
				dir.Memo = "read subdir fail " + fullDir
				return dir
			}
			//子文件夹遍历下一层文件
			for k, subfi := range sub {
				subName := pathname + "/" + fi.Name() + "/" + subfi.Name()
				if subfi.IsDir() {
					//嵌套文件夹
					fmt.Println("      file", k, subName, "是文件夹,略过")
				} else {
					s = append(s, subName)
					filejson := GetFileInfor(subName)
					fmt.Println("      file", k, subName, filejson.Size, "Byte")
					list = append(list, filejson)
				}

			}
		} else {
			//提取文件信息
			fullName := pathname + "/" + fi.Name()

			s = append(s, fullName)
			filejson := GetFileInfor(fullName)
			fmt.Println("   file", i+1, fullName, filejson.Size, "Byte")
			list = append(list, filejson)

			//fmt.Println(i, filejson)
		}
	}
	dir.FileCount = len(s)
	dir.Date = GetFileModTime(pathname)
	dir.Memo = "update"
	dir.List = list
	fmt.Println(pathname, ">已提取文件摘要，包含:", len(s), "个文件")
	return dir
}

//回调递归遍历所有子目录下的文件
func GetAllFilesJson(pathname string) []FileInfor {
	var list []FileInfor
	fmt.Println("   >文件夹：", pathname)
	rd, err := ioutil.ReadDir(pathname)
	if err != nil {
		fmt.Println("read dir fail:", err)
		return list
	}

	//提取文件列表
	for i, fi := range rd {
		fullName := pathname + "/" + fi.Name()
		if fi.IsDir() {
			//文件夹需要再钻取下一层
			//fmt.Println("   >文件夹:", i+1, fullName)
			listTemp := GetAllFilesJson(fullName)
			for _, listme := range listTemp {
				list = append(list, listme)
			}

		} else {
			//提取文件信息
			filejson := GetFileInfor(fullName)
			fmt.Println("   file", i+1, fullName, filejson.Size, "Byte")
			list = append(list, filejson)
		}
	}
	fmt.Println(pathname, ">已提取文件摘要，包含:", len(list), "个文件")
	return list
}

//提取文件列表(所有)
func GetFilelistJson(pathname string) FileDir {
	var dir FileDir
	var list []FileInfor

	dir.DirPath = pathname
	fileInfo, errfound := os.Stat(pathname)
	if errfound != nil {
		fmt.Println("read fail:", pathname)
		dir.Memo = "error"
		return dir
	}
	dir.DirName = fileInfo.Name()
	_, err := ioutil.ReadDir(pathname)
	if err != nil {
		fmt.Println("read dir fail:", err)
		dir.Memo = "read dir fail " + pathname
		return dir
	}

	//提取文件列表
	list = GetAllFilesJson(pathname)

	dir.FileCount = len(list)
	dir.Date = GetFileModTime(pathname)
	dir.Memo = "update"
	dir.List = list
	fmt.Println(pathname, ">已提取文件摘要，包含:", len(list), "个文件")
	return dir
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

//路由处理
//接收客户端上传
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	itms := time.Now()
	fmt.Println("Start Transing upload", itms.Format("2006-01-02 03:04:05"))
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		upcount = upcount + 1
		fmt.Printf("%d . FileName=[ %s ], FormName=[%s]\n", upcount, part.FileName(), part.FormName())
		if part.FileName() == "" { // this is FormData
			data, _ := ioutil.ReadAll(part)
			fmt.Printf("FormData=[%s]\n", string(data))
		} else { // This is FileData
			dst, _ := os.Create("./upload/" + part.FileName())
			defer dst.Close()
			iwrit, err1 := io.Copy(dst, part)
			if err1 != nil {
				fmt.Println(err1.Error(), iwrit)
			} else {
				fmt.Printf("\tTransfer Complete!\t%d . FileName=./upload/%s\n", upcount, part.FileName())
			}
		}
	}
	itme := time.Now()
	ms1 := (itme.UnixNano() - itms.UnixNano()) / 1e6
	fmt.Printf("传输耗时：%v ms\n", NumberFormat(strconv.FormatInt(ms1, 10)))
	println("")
}

//下载 向客户端发送文件
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	// reader, err := r.MultipartReader()
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	itms := time.Now()
	fmt.Println("Start Transing download", itms.Format("2006-01-02 03:04:05"))
	//body 字节流缓存
	// bodyBuffer := &bytes.Buffer{}
	// bodyWriter := multipart.NewWriter(bodyBuffer)

	// var lsFilePath string = "upload" //上传文件夹
	// var filelist []string
	// var filefullname string //路径名+文件名
	// var filename string     //文件名
	// //Get File
	// filelist, error1 := GetAllFile(lsFilePath, filelist)
	// if error1 != nil {
	// 	println(error1.Error())
	// }
	// var idx int
	// //读取文件夹中的文档
	// for idx, filefullname = range filelist {

	// 	if strings.HasSuffix(filefullname, ".csv") || strings.HasSuffix(filefullname, ".xlsx") || strings.HasSuffix(filefullname, ".dbf") || strings.HasSuffix(filefullname, ".png") {
	// 		p := strings.LastIndex(filefullname, "/")
	// 		if p == -1 {
	// 			filename = filefullname
	// 		} else {
	// 			filename = substring(filefullname, p+1, len(filefullname)-p-1)
	// 		}
	// 	} else {
	// 		continue
	// 	}
	// 	fmt.Println(idx, "\t", filefullname, "\t", filename)
	// 	fileWriter, _ := bodyWriter.CreateFormFile("files", filename)

	// 	file, _ := os.Open(filefullname)
	// 	defer file.Close()

	// 	//io.Copy(fileWriter, file)
	// 	//推送文件
	// 	io.Copy(ResponseWriter, file)
	// }

	// contentType := bodyWriter.FormDataContentType()
	// bodyWriter.Close()

	//连接服务器
	//resp, _ := http.Post(urlPost, contentType, bodyBuffer)
	//defer resp.Body.Close()
	//发数据
	//resp_body, _ := ioutil.ReadAll(resp.Body)
	//println(" 传输完成！ 共计: ", idx, " 个文件")
	//log.Println(" resp.Status= ", resp.Status)
	//log.Println(string(resp_body))

	itme := time.Now()
	ms1 := (itme.UnixNano() - itms.UnixNano()) / 1e6
	fmt.Printf("传输耗时：%v ms\n", NumberFormat(strconv.FormatInt(ms1, 10)))
	println("")
}
func helloHandler(w http.ResponseWriter, r *http.Request) {
	timeStart := time.Now()
	var msg1 string
	msg1 = "Your connection is accepted,Congratulate!"
	w.Write([]byte(msg1))
	timeElapsed := time.Since(timeStart)
	fmt.Println(timeElapsed, msg1)
}

//zip压缩 filePath 为需要压缩的文件路径，zipPath为压缩后文件路径
func FileToZip(filePath string, zipPath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	z, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer z.Close()

	wr := zip.NewWriter(z)
	// 因为filePath是一个路径，所以会创建路径中的所有文件夹
	w, err := wr.Create(filePath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	if err != nil {
		return err
	}
	return nil
}

//压缩文件夹
func Zip(srcFile string, destZip string) error {
	zipfile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	filepath.Walk(srcFile, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(path, filepath.Dir(srcFile)+"/")
		// header.Name = path
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
		}
		return err
	})

	return err
}
