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
	SVPS_VERSION = "7.8-STABLE"
	APP_PORT     = "3000"
	PING_INT     = 10 * time.Second
	IDLE_TIMEOUT = 30 * time.Minute
)

type Session struct {
	ID         string
	PTY        *os.File
	Cmd        *exec.Cmd
	Clients    map[*websocket.Conn]bool
	Lock       sync.Mutex
	LastActive time.Time
}

var (
	sessions = make(map[string]*Session)
	sessLock sync.Mutex
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func getBanner() string {
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
		"`   ^\"F         `Y\"      ~ '88888F`    `   ^\"F   \r\n" 

	info := fmt.Sprintf("\r\n  SVPS %s | Licensed by Eternals\r\n  CPU: %d Cores | REALTIME MODE (No Logs)\r\n  Support: helpme.eternals@gmail.com\r\n  --------------------------------------------------\r\n",
		SVPS_VERSION, runtime.NumCPU())
	
	return ascii + info
}

func optimizeSystem() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		rLimit.Cur = 65535
		rLimit.Max = 65535
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	}
}

func startCleaner() {
	for {
		time.Sleep(1 * time.Minute)
		sessLock.Lock()
		for id, s := range sessions {
			s.Lock.Lock()
			if len(s.Clients) == 0 && time.Since(s.LastActive) > IDLE_TIMEOUT {
				log.Printf("[GC] Killing idle session: %s", id)
				s.PTY.Close()
				s.Cmd.Process.Kill()
				delete(sessions, id)
			}
			s.Lock.Unlock()
		}
		sessLock.Unlock()
	}
}

func GetSession(id string) *Session {
	sessLock.Lock()
	defer sessLock.Unlock()

	if s, ok := sessions[id]; ok { return s }

	name := os.Getenv("NAMES"); if name == "" { name = "ROOT" }
	alias := os.Getenv("ALIASE"); if alias == "" { alias = "VPS" }
	
	f, err := os.OpenFile("/root/.bashrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		ps1 := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", name, alias)
		f.WriteString("\n" + ps1 + "\n")
		f.Close()
	}

	c := exec.Command("bash")
	c.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/root")
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	fPty, err := pty.StartWithSize(c, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil { return nil }

	sess := &Session{
		ID:         id,
		PTY:        fPty,
		Cmd:        c,
		Clients:    make(map[*websocket.Conn]bool),
		LastActive: time.Now(),
	}

	go func() {
		defer func() {
			sessLock.Lock()
			delete(sessions, id)
			sessLock.Unlock()
			fPty.Close()
		}()

		buf := make([]byte, 8192)
		for {
			n, err := fPty.Read(buf)
			if err != nil { 
				sess.Lock.Lock()
				for conn := range sess.Clients {
					conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Session Ended"))
					conn.Close()
					delete(sess.Clients, conn)
				}
				sess.Lock.Unlock()
				return 
			}

			sess.Lock.Lock()
			sess.LastActive = time.Now()
			
			activeConns := make([]*websocket.Conn, 0, len(sess.Clients))
			for c := range sess.Clients {
				activeConns = append(activeConns, c)
			}
			sess.Lock.Unlock()

			data := buf[:n]
			for _, conn := range activeConns {
				conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					conn.Close()
					sess.Lock.Lock()
					delete(sess.Clients, conn)
					sess.Lock.Unlock()
				}
			}
		}
	}()

	sessions[id] = sess
	return sess
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	pass := os.Getenv("PASS")
	if pass != "" && r.Header.Get("X-SVPS-TOKEN") != pass {
		http.Error(w, "Forbidden", 403)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }

	sid := r.Header.Get("X-SESSION-ID")
	if sid == "" { sid = "main" }

	sess := GetSession(sid)
	if sess == nil {
		conn.Close()
		return
	}

	sess.Lock.Lock()
	sess.Clients[conn] = true
	sess.LastActive = time.Now() 
	sess.Lock.Unlock()

	conn.WriteMessage(websocket.TextMessage, []byte(getBanner()))

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil { break }
		sess.Lock.Lock()
		sess.LastActive = time.Now()
		sess.Lock.Unlock()
		
		if sess.PTY != nil {
			sess.PTY.Write(msg)
		}
	}

	sess.Lock.Lock()
	delete(sess.Clients, conn)
	sess.Lock.Unlock()
	conn.Close()
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse("http://127.0.0.1:" + APP_PORT)
	if c, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond); err == nil {
		c.Close()
		httputil.NewSingleHostReverseProxy(u).ServeHTTP(w, r)
	} else {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "SVPS %s READY", SVPS_VERSION)
	}
}

func main() {
	optimizeSystem()
	port := os.Getenv("PORT"); if port == "" { port = "8080" }

	go func() {
		for {
			time.Sleep(2 * time.Minute)
			http.Get(fmt.Sprintf("http://127.0.0.1:%s/", port))
		}
	}()

	go startCleaner()

	http.HandleFunc("/sussh", handleSussh)
	http.HandleFunc("/", handleProxy)

	log.Printf("Listening on %s", port)
	http.ListenAndServe(":"+port, nil)
}

