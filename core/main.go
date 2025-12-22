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
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

const (
	SVPS_VERSION = "6.3"
	APP_PORT     = "3000"
	LICENSE      = "Licensed by Eternals (Vlazars)"
	CREATOR      = "Eternals"
	EMAIL        = "helpme.eternals@gmail.com"
)

var (
	isBusy     bool
	sessionMux sync.Mutex
	upgrader   = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

func getBanner() string {
	return fmt.Sprintf(`
   .x+=:.      _                          .x+=:.   
  z`    ^%    u                          z`    ^%  
     .   <k  88Nu.   u.   .d``              .   <k 
   .@8Ned8" '88888.o888c  @8Ne.   .u      .@8Ned8" 
 .@^%8888"   ^8888  8888  %8888:u@88N   .@^%8888"  
x88:  `)8b.   8888  8888   `888I  888. x88:  `)8b. 
8888N=*8888   8888  8888    888I  888I 8888N=*8888 
 %8"    R88   8888  8888    888I  888I  %8"    R88 
  @8Wou 9%   .8888b.888P  uW888L  888'   @8Wou 9%  
.888888P`     ^Y8888*""  '*88888Nu88P  .888888P`   
`   ^"F         `Y"      ~ '88888F`    `   ^"F     
                            888 ^                  
                            *8E                    
                            '8>                    
                             "                     

  %s | %s
  CPU: %d Cores | Runtime: Optimized
  Support: %s
  --------------------------------------------------
`, SVPS_VERSION, LICENSE, runtime.NumCPU(), EMAIL)
}

func injectEternalCommands() {
	eternalScript := fmt.Sprintf(`#!/bin/bash
echo -e "\e[1;33m[ETERNAL RECOVERY] Restoring environment...\e[0m"
name=${NAMES:-ROOT}
alias=${ALIASE:-VPS}
echo "export PS1='\[\033[01;32m\]$name@$alias\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '" > /root/.bashrc
echo -e "\e[1;32m[DONE] Restored. Please relogin shell.\e[0m"
`)
	os.WriteFile("/usr/local/bin/nitro-fix", []byte(eternalScript), 0755)
}

func optimizeSystem() {
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	log.Printf("[NITRO] Core: %d | Status: Overclocked", numCPU)

	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		rLimit.Cur = 65535
		rLimit.Max = 65535
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	}
}

func startHeartbeat(port string) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			http.Get(fmt.Sprintf("http://127.0.0.1:%s/", port))
		}
	}()
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	targetURL, _ := url.Parse("http://127.0.0.1:" + APP_PORT)
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+APP_PORT, 50*time.Millisecond)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body style='background:#000;color:#0f0;font-family:monospace;text-align:center;padding-top:20vh;'>"+
			"<h1>SVPS %s</h1><p>Status: Running</p></body></html>", SVPS_VERSION)
		return
	}
	conn.Close()
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	serverPass := os.Getenv("PASS")
	if serverPass == "" || r.Header.Get("X-SVPS-TOKEN") != serverPass {
		http.Error(w, "ACCESS DENIED", 403)
		return
	}

	sessionMux.Lock()
	if isBusy {
		sessionMux.Unlock()
		http.Error(w, "SERVER BUSY", 429)
		return
	}
	isBusy = true
	sessionMux.Unlock()

	defer func() {
		sessionMux.Lock()
		isBusy = false
		sessionMux.Unlock()
	}()

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

	conn.WriteMessage(websocket.TextMessage, []byte(getBanner()))

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil { return }
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
	injectEternalCommands()

	port := os.Getenv("PORT"); if port == "" { port = "8080" }
	startHeartbeat(port)

	http.HandleFunc("/sussh", handleSussh)
	http.HandleFunc("/", handleProxy)
	log.Printf("[*] SVPS %s Engine Started", SVPS_VERSION)
	http.ListenAndServe(":"+port, nil)
}

