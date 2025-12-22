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
	// Menambahkan \r agar kursor kembali ke awal baris (mencegah teks miring/berantakan)
	ascii := "\r\n   .x+=:.      _                          .x+=:.\r\n" +
		"  z`    ^%    u                          z`    ^%\r\n" +
		"     .   <k  88Nu.   u.   .d``              .   <k\r\n" +
		"   .@8Ned8\" '88888.o888c  @8Ne.   .u      .@8Ned8\"\r\n" +
		" .@^%8888\"   ^8888  8888  %8888:u@88N   .@^%8888\"\r\n" +
		"x88:  `)8b.   8888  8888   `888I  888. x88:  `)8b.\r\n" +
		"8888N=*8888   8888  8888    888I  888I 8888N=*8888\r\n" +
		" %8\"    R88   8888  8888    888I  888I  %8\"    R88\r\n" +
		"  @8Wou 9%   .8888b.888P  uW888L  888'   @8Wou 9%\r\n" +
		".888888P`     ^Y8888*\"\"  '*88888Nu88P  .888888P` \r\n" +
		"`   ^\"F         `Y\"      ~ '88888F`    `   ^\"F   \r\n" +
		"                            888 ^                \r\n" +
		"                            *8E                  \r\n" +
		"                            '8>                  \r\n" +
		"                             \"                   \r\n"

	info := fmt.Sprintf("\r\n  SVPS %s | %s\r\n  CPU: %d Cores | Runtime: Optimized\r\n  Support: %s\r\n  --------------------------------------------------\r\n",
		SVPS_VERSION, LICENSE, runtime.NumCPU(), EMAIL)
	
	return ascii + info
}

func injectEternalCommands() {
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
	[span_3](start_span)[span_4](start_span)runtime.GOMAXPROCS(numCPU)[span_3](end_span)[span_4](end_span)
	[span_5](start_span)log.Printf("[NITRO] Detected %d CPU Cores", numCPU)[span_5](end_span)

	var rLimit syscall.Rlimit
	[span_6](start_span)err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)[span_6](end_span)
	if err == nil {
		[span_7](start_span)[span_8](start_span)rLimit.Cur = 65535[span_7](end_span)[span_8](end_span)
		[span_9](start_span)rLimit.Max = 65535[span_9](end_span)
		_[span_10](start_span)[span_11](start_span) = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)[span_10](end_span)[span_11](end_span)
	}
}

func startHeartbeat(port string) {
	[span_12](start_span)[span_13](start_span)ticker := time.NewTicker(2 * time.Minute)[span_12](end_span)[span_13](end_span)
	go func() {
		for range ticker.C {
			_[span_14](start_span)[span_15](start_span), _ = http.Get(fmt.Sprintf("http://127.0.0.1:%s/", port))[span_14](end_span)[span_15](end_span)
		}
	}()
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	[span_16](start_span)[span_17](start_span)targetURL, _ := url.Parse("http://127.0.0.1:" + APP_PORT)[span_16](end_span)[span_17](end_span)
	[span_18](start_span)conn, err := net.DialTimeout("tcp", "127.0.0.1:"+APP_PORT, 50*time.Millisecond)[span_18](end_span)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body style='background:#000;color:#0f0;font-family:monospace;text-align:center;padding-top:20vh;'>"+
			[span_19](start_span)"<h1>SVPS %s</h1><p>Status: ACTIVE</p></body></html>", SVPS_VERSION)[span_19](end_span)
		return
	}
	conn.Close()
	[span_20](start_span)[span_21](start_span)proxy := httputil.NewSingleHostReverseProxy(targetURL)[span_20](end_span)[span_21](end_span)
	[span_22](start_span)proxy.ServeHTTP(w, r)[span_22](end_span)
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	[span_23](start_span)[span_24](start_span)serverPass := os.Getenv("PASS")[span_23](end_span)[span_24](end_span)
	[span_25](start_span)token := r.Header.Get("X-SVPS-TOKEN")[span_25](end_span)
	if serverPass == "" || token != serverPass {
		[span_26](start_span)http.Error(w, "ACCESS DENIED", 403)[span_26](end_span)
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

	[span_27](start_span)conn, err := upgrader.Upgrade(w, r, nil)[span_27](end_span)
	if err != nil {
		return
	}
	[span_28](start_span)defer conn.Close()[span_28](end_span)

	[span_29](start_span)[span_30](start_span)name := os.Getenv("NAMES")[span_29](end_span)[span_30](end_span)
	if name == "" {
		[span_31](start_span)name = "ROOT"[span_31](end_span)
	}
	[span_32](start_span)[span_33](start_span)alias := os.Getenv("ALIASE")[span_32](end_span)[span_33](end_span)
	if alias == "" {
		[span_34](start_span)alias = "VPS"[span_34](end_span)
	}

	[span_35](start_span)ps1 := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", name, alias)[span_35](end_span)
	[span_36](start_span)f, err := os.OpenFile("/root/.bashrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)[span_36](end_span)
	if err == nil {
		[span_37](start_span)f.WriteString("\n" + ps1 + "\n")[span_37](end_span)
		[span_38](start_span)f.Close()[span_38](end_span)
	}

	[span_39](start_span)c := exec.Command("bash")[span_39](end_span)
	[span_40](start_span)c.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/root")[span_40](end_span)
	[span_41](start_span)[span_42](start_span)fPty, err := pty.Start(c)[span_41](end_span)[span_42](end_span)
	if err != nil {
		return
	}
	[span_43](start_span)defer fPty.Close()[span_43](end_span)

	_[span_44](start_span) = conn.WriteMessage(websocket.TextMessage, []byte(getBanner()))[span_44](end_span)

	go func() {
		for {
			_[span_45](start_span), msg, err := conn.ReadMessage()[span_45](end_span)
			if err != nil {
				return
			}
			[span_46](start_span)fPty.Write(msg)[span_46](end_span)
		}
	}()

	buf := make([]byte, 4096)
	for {
		[span_47](start_span)n, err := fPty.Read(buf)[span_47](end_span)
		if err != nil {
			break
		}
		_[span_48](start_span) = conn.WriteMessage(websocket.TextMessage, buf[:n])[span_48](end_span)
	}
}

func main() {
	[span_49](start_span)optimizeSystem()[span_49](end_span)
	injectEternalCommands()

	[span_50](start_span)[span_51](start_span)port := os.Getenv("PORT")[span_50](end_span)[span_51](end_span)
	if port == "" {
		[span_52](start_span)port = "8080"[span_52](end_span)
	}

	[span_53](start_span)startHeartbeat(port)[span_53](end_span)

	[span_54](start_span)http.HandleFunc("/sussh", handleSussh)[span_54](end_span)
	[span_55](start_span)http.HandleFunc("/", handleProxy)[span_55](end_span)

	[span_56](start_span)log.Printf("[*] SVPS %s Engine Started on port %s", SVPS_VERSION, port)[span_56](end_span)
	_[span_57](start_span) = http.ListenAndServe(":"+port, nil)[span_57](end_span)
}

