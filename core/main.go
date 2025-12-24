package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	SVPS_VERSION = "10.1-TITAN-FIX"
	APP_PORT     = "3000"
	IDLE_TIMEOUT = 24 * time.Hour
	ETP_MAGIC    = 0xE7
	ETP_OP_DATA  = 0x01
	ETP_OP_INPUT = 0x02
)

type Session struct {
	ID         string
	PTY        *os.File
	Cmd        *exec.Cmd
	Client     net.Conn
	Lock       sync.Mutex
	LastActive time.Time
}

var (
	sessions = make(map[string]*Session)
	sessLock sync.Mutex
	routeMap = make(map[string]string)
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

	info := fmt.Sprintf("\r\n  SVPS %s | TITAN EDITION\r\n  Kernel: Linux (Podman Enabled)\r\n  Init System: S6-Overlay\r\n  Support: helpme.eternals@gmail.com\r\n  --------------------------------------------------\r\n",
		SVPS_VERSION)

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

func loadRoutes() {
	raw := os.Getenv("ROUTES")
	if raw == "" {
		return
	}
	rules := strings.Split(raw, ";")
	for _, rule := range rules {
		parts := strings.Split(rule, ":")
		if len(parts) >= 2 {
			domain := strings.TrimSpace(parts[0])
			targetPort := strings.TrimSpace(parts[1])
			routeMap[domain] = targetPort
			log.Printf("[ROUTER] Mapping %s -> 127.0.0.1:%s", domain, targetPort)
		}
	}
}

func startCleaner() {
	for {
		time.Sleep(1 * time.Minute)
		sessLock.Lock()
		for id, s := range sessions {
			s.Lock.Lock()
			if s.Client == nil && time.Since(s.LastActive) > IDLE_TIMEOUT {
				s.PTY.Close()
				s.Cmd.Process.Kill()
				delete(sessions, id)
			}
			s.Lock.Unlock()
		}
		sessLock.Unlock()
	}
}

func sendETPPacket(conn net.Conn, op byte, data []byte) error {
	if conn == nil {
		return fmt.Errorf("no connection")
	}
	length := uint16(len(data))
	packet := make([]byte, 4+len(data))

	packet[0] = ETP_MAGIC
	packet[1] = op
	binary.BigEndian.PutUint16(packet[2:4], length)
	copy(packet[4:], data)

	conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	_, err := conn.Write(packet)
	return err
}

func GetSession(id string) *Session {
	sessLock.Lock()
	defer sessLock.Unlock()
	if s, ok := sessions[id]; ok {
		return s
	}

	name := os.Getenv("NAMES")
	if name == "" {
		name = "TITAN"
	}
	alias := os.Getenv("ALIASE")
	if alias == "" {
		alias = "VPS"
	}

	f, _ := os.OpenFile("/root/.bashrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	ps1 := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", name, alias)
	f.WriteString("\n" + ps1 + "\n")
	f.Close()

	c := exec.Command("bash")
	c.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/root")
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	fPty, err := pty.StartWithSize(c, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil
	}

	sess := &Session{
		ID:         id,
		PTY:        fPty,
		Cmd:        c,
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
				if sess.Client != nil {
					sess.Client.Close()
				}
				sess.Lock.Unlock()
				return
			}

			sess.Lock.Lock()
			client := sess.Client
			sess.Lock.Unlock()

			if client != nil {
				if err := sendETPPacket(client, ETP_OP_DATA, buf[:n]); err != nil {
					sess.Lock.Lock()
					if sess.Client == client {
						sess.Client = nil
					}
					sess.Lock.Unlock()
				}
			}
		}
	}()
	sessions[id] = sess
	return sess
}

func handleETP(w http.ResponseWriter, r *http.Request) {
	pass := os.Getenv("PASS")
	if pass != "" && r.Header.Get("X-SVPS-TOKEN") != pass {
		http.Error(w, "Forbidden", 403)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		// Log error buat debugging
		log.Printf("[ETP] Hijack Failed for %s", r.RemoteAddr)
		http.Error(w, "Server Error", 500)
		return
	}
	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		return
	}

	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: eternals-protocol\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n\r\n")
	bufrw.Flush()

	sid := r.Header.Get("X-SESSION-ID")
	if sid == "" {
		sid = "main"
	}
	sess := GetSession(sid)

	sess.Lock.Lock()
	if sess.Client != nil {
		sess.Client.Close()
	}
	sess.Client = conn
	sess.LastActive = time.Now()
	sess.Lock.Unlock()

	sendETPPacket(conn, ETP_OP_DATA, []byte(getBanner()))

	header := make([]byte, 4)
	for {
		_, err := io.ReadFull(bufrw, header)
		if err != nil {
			break
		}

		if header[0] != ETP_MAGIC {
			break
		}
		op := header[1]
		length := binary.BigEndian.Uint16(header[2:4])

		payload := make([]byte, length)
		_, err = io.ReadFull(bufrw, payload)
		if err != nil {
			break
		}

		sess.Lock.Lock()
		sess.LastActive = time.Now()
		sess.Lock.Unlock()

		if op == ETP_OP_INPUT {
			sess.PTY.Write(payload)
		}
	}

	sess.Lock.Lock()
	if sess.Client == conn {
		sess.Client = nil
	}
	sess.Lock.Unlock()
	conn.Close()
}

func proxyToPort(w http.ResponseWriter, r *http.Request, targetPort string) {
	targetURL, _ := url.Parse("http://127.0.0.1:" + targetPort)

	if conn, err := net.DialTimeout("tcp", targetURL.Host, 100*time.Millisecond); err == nil {
		conn.Close()
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = r.Host
			req.Header.Set("X-Forwarded-Host", r.Host)
			req.Header.Set("X-Forwarded-Proto", "https")
		}
		proxy.ServeHTTP(w, r)
	} else {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "SVPS ROUTER: Service on port %s is OFFLINE", targetPort)
	}
}

// HANDLER UTAMA: KITA GABUNG DISINI (FORCE ROUTING)
func handleMaster(w http.ResponseWriter, r *http.Request) {
	// DEBUG: Cek Path yang diterima
	// log.Printf("Incoming: %s %s", r.Method, r.URL.Path)

	// 1. FORCE CHECK: Apakah ini request ETP?
	// Cek Path persis "/etp" ATAU path "/etp/"
	if r.URL.Path == "/etp" || strings.HasPrefix(r.URL.Path, "/etp/") {
		handleETP(w, r)
		return
	}

	// 2. Kalau bukan ETP, jalankan Logic Router (Domain)
	host := r.Host
	if strings.Contains(host, ":") {
		h, _, err := net.SplitHostPort(host)
		if err == nil {
			host = h
		}
	}

	if targetPort, exists := routeMap[host]; exists {
		proxyToPort(w, r, targetPort)
		return
	}

	// 3. Fallback (Router Error)
	proxyToPort(w, r, APP_PORT)
}

func main() {
	optimizeSystem()
	loadRoutes()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		for {
			time.Sleep(2 * time.Minute)
			http.Get(fmt.Sprintf("http://127.0.0.1:%s/", port))
		}
	}()
	go startCleaner()

	// GANTI: Gunakan handler tunggal untuk menangkap SEMUA request
	http.HandleFunc("/", handleMaster)

	log.Printf("SVPS %s [ETP-ONLY] Listening on %s", SVPS_VERSION, port)
	http.ListenAndServe(":"+port, nil)
}
