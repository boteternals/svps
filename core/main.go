package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

const SVPS_VERSION = "6.0-NITRO"
const APP_PORT = "3000"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool { return true },
}

func optimizeSystem() {
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	log.Printf("[NITRO] Detected %d CPU Cores. Maximizing usage...", numCPU)

	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err == nil {
		rLimit.Cur = 65535
		rLimit.Max = 65535
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		log.Printf("[NITRO] Ulimit raised to 65535 (Game Server Ready)")
	}
}

func startHeartbeat(port string) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			http.Get(fmt.Sprintf("http://127.0.0.1:%s/", port))
		}
	}()
	log.Println("[NITRO] Heartbeat System Active (Anti-Sleep Mode On)")
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	targetURL, _ := url.Parse("http://127.0.0.1:" + APP_PORT)
	
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+APP_PORT, 50*time.Millisecond)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html><body style="background:#000;color:#f00;font-family:monospace;text-align:center;padding-top:20vh;">
			<h1>SVPS %s OVERCLOCKED</h1>
			<p>CORE: %d CPU | RAM: OPTIMIZED</p>
			<p>Status: Waiting for App on Port %s...</p>
			</body></html>
		`, SVPS_VERSION, runtime.NumCPU(), APP_PORT)
		return
	}
	conn.Close()

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {}
	
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Real-IP", r.RemoteAddr)
	proxy.ServeHTTP(w, r)
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	serverPass := os.Getenv("PASS")
	if serverPass == "" || r.Header.Get("X-SVPS-TOKEN") != serverPass {
		http.Error(w, "ACCESS DENIED", 403)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }
	defer conn.Close()

	name := os.Getenv("NAMES"); if name == "" { name = "ROOT" }
	alias := os.Getenv("ALIASE"); if alias == "" { alias = "VPS" }
	
	ps1 := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", name, alias)
	f, _ := os.OpenFile("/root/.bashrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil { f.WriteString("\n" + ps1 + "\n"); f.Close() }

	c := exec.Command("bash")
	c.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/root")
	fPty, err := pty.Start(c)
	if err != nil { return }
	defer fPty.Close()

	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\033[1;31m[SVPS %s] CPU: %d Cores | NITRO MODE ACTIVE\033[0m\r\n", SVPS_VERSION, runtime.NumCPU())))

	go func() {
		for {
			_, msg, err := conn.ReadMessage(); if err != nil { return }
			fPty.Write(msg)
		}
	}()
	
	buf := make([]byte, 4096)
	for {
		n, err := fPty.Read(buf)
		if err != nil { break }
		conn.WriteMessage(websocket.TextMessage, buf[:n])
	}
}

func main() {
	optimizeSystem()

	port := os.Getenv("PORT"); if port == "" { port = "8080" }

	startHeartbeat(port)

	http.HandleFunc("/sussh", handleSussh)
	http.HandleFunc("/", handleProxy)

	log.Printf("[*] SVPS %s Running on port %s", SVPS_VERSION, port)
	http.ListenAndServe(":"+port, nil)
}

