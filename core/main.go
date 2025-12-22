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
	LICENSE      = "Licensed by Eternals"
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
	ascii := "\n   .x+=:.      _                          .x+=:.\n" +
		"  z`    ^%    u                          z`    ^%\n" +
		"     .   <k  88Nu.   u.   .d``              .   <k\n" +
		"   .@8Ned8\" '88888.o888c  @8Ne.   .u      .@8Ned8\"\n" +
		" .@^%8888\"   ^8888  8888  %8888:u@88N   .@^%8888\"\n" +
		"x88:  `)8b.   8888  8888   `888I  888. x88:  `)8b.\n" +
		"8888N=*8888   8888  8888    888I  888I 8888N=*8888\n" +
		" %8\"    R88   8888  8888    888I  888I  %8\"    R88\n" +
		"  @8Wou 9%   .8888b.888P  uW888L  888'   @8Wou 9%\n" +
		".888888P`     ^Y8888*\"\"  '*88888Nu88P  .888888P` \n" +
		"`   ^\"F         `Y\"      ~ '88888F`    `   ^\"F   \n" +
		"                            888 ^                \n" +
		"                            *8E                  \n" +
		"                            '8>                  \n" +
		"                             \"                   \n"
	
	return fmt.Sprintf("%s\n  SVPS %s | %s\n  CPU: %d Cores | Runtime: Optimized\n  Support: %s\n  --------------------------------------------------\n",
		ascii, SVPS_VERSION, LICENSE, runtime.NumCPU(), EMAIL)
}

func injectEternalCommands() {
	[span_4](start_span)
	script := `#!/bin/bash
echo -e "\e[1;33m[RECOVERY] Restoring environment...\e[0m"
name=${NAMES:-ROOT}
alias=${ALIASE:-VPS}
echo "export PS1='\[\033[01;32m\]$name@$alias\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '" > /root/.bashrc
echo -e "\e[1;32m[DONE] Restored. Please relogin shell.\e[0m"
`
	_ = os.WriteFile("/usr/local/bin/nitro-fix", []byte(script), 0755)
}

func optimizeSystem() {
	numCPU := runtime.NumCPU()
	[span_5](start_span)[span_6](start_span)runtime.GOMAXPROCS(numCPU)
	log.Printf("[NITRO] Detected %d CPU Cores. Maximizing usage...", numCPU)

	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		[span_7](start_span)[span_8](start_span)rLimit.Cur = 65535
		rLimit.Max = 65535
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		log.Printf("[NITRO] Ulimit raised to 65535")
	}
}

func startHeartbeat(port string) {
	[span_9](start_span)ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			_, _ = http.Get(fmt.Sprintf("http://127.0.0.1:%s/", port))
		}
	}()
	log.Println("[NITRO] Heartbeat System Active")
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	targetURL, _ := url.Parse("http://127.0.0.1:" + APP_PORT)
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+APP_PORT, 50*time.Millisecond)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html><body style="background:#000;color:#0f0;font-family:monospace;text-align:center;padding-top:20vh;">
			<h1>SVPS %s</h1>
			<p>CORE: %d CPU | STATUS: ACTIVE</p>
			</body></html>
		`, SVPS_VERSION, runtime.NumCPU())
		return
	}
	conn.Close()

	[span_10](start_span)proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	[span_11](start_span)[span_12](start_span)serverPass := os.Getenv("PASS")
	if serverPass == "" || r.Header.Get("X-SVPS-TOKEN") != serverPass {
		http.Error(w, "ACCESS DENIED", 403)
		return
	}

	sessionMux.Lock()
	if isBusy {
		sessionMux.Unlock()
		http.Error(w, "SERVER BUSY: ANOTHER USER IS CONNECTED", 429)
		return
	}
	isBusy = true
	sessionMux.Unlock()

	defer func() {
		sessionMux.Lock()
		isBusy = false
		sessionMux.Unlock()
		log.Println("[SVPS] Connection closed, isBusy reset to false")
	}()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }
	defer conn.Close()

	[span_13](start_span)[span_14](start_span)name := os.Getenv("NAMES"); if name == "" { name = "ROOT" }
	[span_15](start_span)[span_16](start_span)alias := os.Getenv("ALIASE"); if alias == "" { alias = "VPS" }

	ps1 := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", name, alias)
	f, _ := os.OpenFile("/root/.bashrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		_, _ = f.WriteString("\n" + ps1 + "\n")
		f.Close()
	}

	c := exec.Command("bash")
	c.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/root")
	[span_17](start_span)[span_18](start_span)fPty, err := pty.Start(c)
	if err != nil { return }
	defer fPty.Close()

	_ = conn.WriteMessage(websocket.TextMessage, []byte(getBanner()))

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil { return }
			_, _ = fPty.Write(msg)
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := fPty.Read(buf)
		if err != nil { break }
		_ = conn.WriteMessage(websocket.TextMessage, buf[:n])
	}
}

func main() {
	optimizeSystem()
	injectEternalCommands()

	[span_19](start_span)[span_20](start_span)port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	startHeartbeat(port)

	[span_21](start_span)http.HandleFunc("/sussh", handleSussh)
	[span_22](start_span)http.HandleFunc("/", handleProxy)

	log.Printf("[*] SVPS %s Running on port %s", SVPS_VERSION, port)
	_ = http.ListenAndServe(":"+port, nil)
}

