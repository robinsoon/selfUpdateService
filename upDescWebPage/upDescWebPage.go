//查看更新说明Webpage  2020-04-09 Robin
package main

import (
	"strconv"

	"github.com/widuu/goini"
	"github.com/zserge/webview"
)

var iniconfig string = "selfUpdate.ini"       //配置文件
var webhost string = "http://localhost:8386/" //更新服务器地址 来自ini配置
var winWidth int = 1000
var winHeight int = 600

func main() {

	//读取配置文件
	conf := goini.SetConfig(iniconfig)
	webhost = conf.GetValue("UpdateTrans", "webhost")
	if webhost != "no value" && webhost != "" {

	} else {
		return
	}
	swidth := conf.GetValue("UpdateTrans", "webWidth")
	sheight := conf.GetValue("UpdateTrans", "webHeight")
	if swidth != "no value" && swidth != "" {
		winWidth, _ = strconv.Atoi(swidth)
		if winWidth <= 0 {
			winWidth = 1000
		}
	}
	if sheight != "no value" && sheight != "" {
		winHeight, _ = strconv.Atoi(sheight)
		if winHeight <= 0 {
			winHeight = 600
		}
	}
	//标题不能使用中文
	webview.Open("Introduction "+webhost,
		webhost, winWidth, winHeight, true)

	// Open wikipedia in a 800x600 resizable window
	//webview.Open("Minimal webview example",
	//	"https://en.m.wikipedia.org/wiki/Main_Page", 800, 600, true)

	// webview.Open("GOlang webview example",
	// 	"https://studygolang.com/books", 1000, 800, true)

	//html5 兼容性
	// webview.Open("html5 webview example",
	// 	"http://web.chacuo.net/testhtml5", 1000, 800, true)

	//浏览器内核
	// webview.Open("html5 webview example",
	// 	"https://ie.icoa.cn", 1000, 800, true)

	// webview.Open("selfupdate Server",
	// 	"http://localhost:8386/", 1000, 800, true)
}
