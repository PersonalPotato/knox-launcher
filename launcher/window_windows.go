//go:build windows

package main

import (
	"log"
	"path/filepath"

	webview2 "github.com/jchv/go-webview2"
)

func openWindow(app *App, url string) {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  filepath.Join(app.paths.Data, "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Knox Launcher - Easy Zomboid Servers",
			Width:  1280,
			Height: 800,
			IconId: 1,
			Center: true,
		},
	})
	if w == nil {
		// no WebView2 runtime: use the normal browser instead
		log.Println("WebView2 missing, opening the browser")
		_ = openURL(url)
		select {}
	}
	defer w.Destroy()
	w.SetSize(1100, 700, webview2.HintMin)
	app.quit = func() { w.Dispatch(w.Terminate) }
	w.Navigate(url)
	w.Run()
	app.shutdown()
}
