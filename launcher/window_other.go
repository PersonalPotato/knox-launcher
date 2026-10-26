//go:build !windows

package main

import "fmt"

func openWindow(app *App, url string) {
	fmt.Println("Knox Launcher (dev) running at", url)
	app.quit = func() { fmt.Println("(dev) app would close now") }
	select {}
}
