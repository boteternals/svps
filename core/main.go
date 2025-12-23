package main

import (
	"bytes"
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
	SVPS_VERSION = "7.1"
	APP_PORT     = "3000"
	HIST_SIZE    = 16 * 1024
	PING_INT     = 10 * time.Second
)

type Session struct {
	ID        string
	PTY       *os.File
	Cmd       *exec.Cmd
	History   *bytes.Buffer
	Clients   map[*websocket.Conn]bool
	Lock      sync.Mutex
	ActiveAt  time.Time
}

var (
	sessions    = make(map[string]*Session)
	sessLock    sync.Mutex
	upgrader    = websocket.Upgrader{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		CheckOrigin:     func(r *http.Request) bool { return true },
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
		"`   ^\"F         `Y\"      ~ '88888F`    `   ^\"F   \r\n" +
		"                            888 ^                \r\n" +
		"                            *8E                  \r\n" +
		"                            '8>                  \r\n" +
		"                             \"                   \r\n"

	info := fmt.Sprintf("\r\n  SVPS %s | Licensed by Eternals\r\n  CPU: %d Cores\r\n  Support: helpme.eternals@gmail.com\r\n  --------------------------------------------------\r\n",
		SVPS_VERSION, runtime.NumCPU())
	
	return ascii + info
}

func optimizeSystem() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		rLimit.Cur = 65535; rLimit.Max = 65535
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	}
}

func (s *Session) Broadcast(data []byte) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	
	if s.History.Len()+len(data) > HIST_SIZE {
		s.History.Reset()
	}
	s.History.Write(data)

	for conn := range s.Clients {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(s.Clients, conn)
		}
	}
	s.ActiveAt = time.Now()
}

func GetSession(id string) *Session {
	sessLock.Lock()
	defer sessLock.Unlock()

	if s, ok := sessions[id]; ok { return s }

	name := os.Getenv("NAMES"); if name == "" { name = "ROOT" }
	alias := os.Getenv("ALIASE"); if alias == "" { alias = "VPS" }
	
	// FIX PROMPT: Paksa tulis ke .bashrc agar prompt sesuai keinginan
	f, err := os.OpenFile("/root/.bashrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		ps1 := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", name, alias)
		f.WriteString("\n" + ps1 + "\n")
		f.Close()
	}

	c := exec.Command("bash")
	c.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/root")
	
	fPty, err := pty.Start(c)
	if err != nil { return nil }

	sess := &Session{
		ID: id, PTY: fPty, Cmd: c,
		History: bytes.NewBuffer(make([]byte, 0, HIST_SIZE)),
		Clients: make(map[*websocket.Conn]bool),
		ActiveAt: time.Now(),
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := fPty.Read(buf)
			if err != nil {
				sessLock.Lock(); delete(sessions, id); sessLock.Unlock()
				fPty.Close(); return
			}
			sess.Broadcast(buf[:n])
		}
	}()

	sessions[id] = sess
	return sess
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	pass := os.Getenv("PASS")
	if pass != "" && r.Header.Get("X-SVPS-TOKEN") != pass {
		http.Error(w, "", 403); return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }

	sid := r.Header.Get("X-SESSION-ID")
	if sid == "" { sid = "main" }

	sess := GetSession(sid)
	if sess == nil { conn.Close(); return }

	sess.Lock.Lock()
	sess.Clients[conn] = true
	conn.WriteMessage(websocket.TextMessage, []byte(getBanner()))
	conn.WriteMessage(websocket.TextMessage, sess.History.Bytes())
	sess.Lock.Unlock()

	exit := make(chan bool)
	go func() {
		tk := time.NewTicker(PING_INT)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				sess.Lock.Lock()
				if _, ok := sess.Clients[conn]; !ok { sess.Lock.Unlock(); return }
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					sess.Lock.Unlock(); return
				}
				sess.Lock.Unlock()
			case <-exit: return
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil { break }
		if sess.PTY != nil { sess.PTY.Write(msg) }
	}

	close(exit)
	sess.Lock.Lock(); delete(sess.Clients, conn); sess.Lock.Unlock()
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

	http.HandleFunc("/sussh", handleSussh)
	http.HandleFunc("/", handleProxy)

	log.Printf("Listening on %s", port)
	http.ListenAndServe(":"+port, nil)
}

