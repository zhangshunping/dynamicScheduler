package utils

import (
	"fmt"
	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

var Log = logrus.New()

func init() {
	// 为当前logrus实例设置消息的输出,同样地,
	// 可以设置logrus实例的输出到任意io.writer
	dir,_:=os.Getwd()
	logName:=fmt.Sprintf("%s/logs/dynamic-%v.log",dir,time.Now().Format("2006-01-02"))
	fmt.Println(dir)
	fmt.Println(logName)
	logfile,err:=os.OpenFile(logName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err!=nil{
		panic("打开日志文件失败")
	}
	Log.SetOutput(logfile)
	// 为当前logrus实例设置消息输出格式为json格式.
	// 同样地,也可以单独为某个logrus实例设置日志级别和hook,这里不详细叙述.
	//	Log.Formatter = &logrus.JSONFormatter{}
	//Log.Formatter = &logrus.TextFormatter{}
	Log.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		TimestampFormat: time.RFC3339,
		TrimMessages: true,
		NoColors: true,
	})
	// 日志等级
	Log.SetLevel(logrus.InfoLevel)
	// 日志输出 执行的程序函数名和路径
	//Log.SetReportCaller(true)
}



