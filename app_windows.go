//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/energye/systray"
	"github.com/go-toast/toast"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	defer PanicHandler()
	runtime.EventsOn(ctx, "frontendError", func(optionalData ...interface{}) {
		logger.SugaredLogger.Errorf("Frontend error: %v\n", optionalData)
	})
	logger.SugaredLogger.Infof("Version:%s", Version)
	// Perform your setup here
	a.ctx = ctx

	// 创建系统托盘
	//systray.RunWithExternalLoop(func() {
	//	onReady(a)
	//}, func() {
	//	onExit(a)
	//})
	runtime.EventsOn(ctx, "updateSettings", func(optionalData ...interface{}) {
		logger.SugaredLogger.Infof("updateSettings : %v\n", optionalData)
		config := data.GetSettingConfig()
		//setMap := optionalData[0].(map[string]interface{})
		//
		//// 将 map 转换为 JSON 字节切片
		//jsonData, err := json.Marshal(setMap)
		//if err != nil {
		//	logger.SugaredLogger.Errorf("Marshal error:%s", err.Error())
		//	return
		//}
		//// 将 JSON 字节切片解析到结构体中
		//err = json.Unmarshal(jsonData, config)
		//if err != nil {
		//	logger.SugaredLogger.Errorf("Unmarshal error:%s", err.Error())
		//	return
		//}

		logger.SugaredLogger.Infof("updateSettings config:%+v", config)
		if config.DarkTheme {
			runtime.WindowSetBackgroundColour(ctx, 27, 38, 54, 1)
			runtime.WindowSetDarkTheme(ctx)
		} else {
			runtime.WindowSetBackgroundColour(ctx, 255, 255, 255, 1)
			runtime.WindowSetLightTheme(ctx)
		}
		runtime.WindowReloadApp(ctx)

	})
	go systray.Run(func() {
		onReady(a)
	}, func() {
		onExit(a)
	})

	logger.SugaredLogger.Infof(" application startup Version:%s", Version)
}

func OnSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	notification := toast.Notification{
		AppID:    "go-stock",
		Title:    "go-stock",
		Message:  "程序已经在运行了",
		Icon:     "",
		Duration: "short",
		Audio:    toast.Default,
	}
	err := notification.Push()
	if err != nil {
		logger.SugaredLogger.Error(err)
	}
	time.Sleep(time.Second * 3)
}

func MonitorStockPrices(a *App) {
	dest := &[]data.FollowedStock{}
	db.Dao.Model(&data.FollowedStock{}).Find(dest)
	total := float64(0)
	//for _, follow := range *dest {
	//	stockData := getStockInfo(follow)
	//	total += stockData.ProfitAmountToday
	//	price, _ := convertor.ToFloat(stockData.Price)
	//	if stockData.PrePrice != price {
	//		go runtime.EventsEmit(a.ctx, "stock_price", stockData)
	//	}
	//}

	stockInfos := GetStockInfos(*dest...)
	for _, stockInfo := range *stockInfos {
		if strutil.HasPrefixAny(stockInfo.Code, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}

		total += stockInfo.ProfitAmountToday
		price, _ := convertor.ToFloat(stockInfo.Price)

		if stockInfo.PrePrice != price {
			//logger.SugaredLogger.Infof("-----------sz------------股票代码: %s, 股票名称: %s, 股票价格: %s,盘前盘后:%s", stockInfo.Code, stockInfo.Name, stockInfo.Price, stockInfo.BA)
			go runtime.EventsEmit(a.ctx, "stock_price", stockInfo)
		}

	}
	if total != 0 {
		title := "go-stock " + time.Now().Format(time.DateTime) + fmt.Sprintf("  %.2f¥", total)
		systray.SetTooltip(title)
	}

	go runtime.EventsEmit(a.ctx, "realtime_profit", fmt.Sprintf("  %.2f", total))
	//runtime.WindowSetTitle(a.ctx, title)

}

func onReady(a *App) {

	// 初始化操作
	logger.SugaredLogger.Infof("systray onReady")
	systray.SetIcon(icon2)
	systray.SetTitle("go-stock")
	systray.SetTooltip("go-stock 股票行情实时获取")
	// 创建菜单项
	show := systray.AddMenuItem("显示", "显示应用程序")
	show.Click(func() {
		//logger.SugaredLogger.Infof("显示应用程序")
		runtime.WindowShow(a.ctx)
	})
	hide := systray.AddMenuItem("隐藏", "隐藏应用程序")
	hide.Click(func() {
		//logger.SugaredLogger.Infof("隐藏应用程序")
		runtime.WindowHide(a.ctx)
	})
	systray.AddSeparator()
	mQuitOrig := systray.AddMenuItem("退出", "退出应用程序")
	mQuitOrig.Click(func() {
		//logger.SugaredLogger.Infof("退出应用程序")
		runtime.Quit(a.ctx)
	})
	systray.SetOnRClick(func(menu systray.IMenu) {
		menu.ShowMenu()
		//logger.SugaredLogger.Infof("SetOnRClick")
	})
	systray.SetOnClick(func(menu systray.IMenu) {
		//logger.SugaredLogger.Infof("SetOnClick")
		menu.ShowMenu()
	})
	systray.SetOnDClick(func(menu systray.IMenu) {
		menu.ShowMenu()
		//logger.SugaredLogger.Infof("SetOnDClick")
	})
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	defer PanicHandler()

	dialog, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:         runtime.QuestionDialog,
		Title:        "go-stock",
		Message:      "确定关闭吗？",
		Buttons:      []string{"确定"},
		Icon:         icon,
		CancelButton: "取消",
	})

	if err != nil {
		logger.SugaredLogger.Errorf("dialog error:%s", err.Error())
		return false
	}
	logger.SugaredLogger.Debugf("dialog:%s", dialog)
	if dialog == "No" {
		return true
	} else {
		systray.Quit()
		a.cron.Stop()
		return false
	}
}

func getFrameless() bool {
	return true
}

func getScreenResolution() (int, int, int, int, error) {
	//user32 := syscall.NewLazyDLL("user32.dll")
	//getSystemMetrics := user32.NewProc("GetSystemMetrics")
	//
	//width, _, _ := getSystemMetrics.Call(0)
	//height, _, _ := getSystemMetrics.Call(1)
	//return int(width), int(height), 1456, 768, nil

	return int(1366), int(768), 1456, 768, nil
}
