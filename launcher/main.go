package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	port := flag.Int("port", 0, "local port for the UI (0 = any)")
	headless := flag.Bool("headless", false, "don't open a window, just print the address")
	flag.Parse()

	paths := detectPaths()
	logPath := filepath.Join(paths.Data, "knoxlauncher.log")
	if info, err := os.Stat(logPath); err == nil && info.Size() > 4<<20 {
		_ = os.Rename(logPath, logPath+".old")
	}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(f)
	}
	// steam_api looks for steam_appid.txt in the working folder
	_ = os.Chdir(paths.Data)
	log.Printf("---- Knox Launcher %s, paths %+v", appVersion, paths)

	app := newApp(paths)
	srv := newServer(app)
	url, err := srv.Listen(*port)
	if err != nil {
		fmt.Println("couldn't start:", err)
		os.Exit(1)
	}
	if *headless {
		fmt.Println(url)
		select {}
	}
	openWindow(app, url)
}
