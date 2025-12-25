package main

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	EngineVer      = "13.6-STABLE"
	ConfigPath     = "/etc/svps/config.json"
	ProtocolMagic  = 0x5650
	ProtocolVer    = 0x0D
	MaxPacketSize  = 1024 * 1024
	OpAuthChal     = 0x02
	OpAuthResp     = 0x03
	OpAuthOK       = 0x04
	OpAuthFail     = 0x05
	OpShellData    = 0x10
	OpResize       = 0x11
)

type SystemConfig struct {
	Users map[string]string `json:"users"`
}

type Session struct {
	ID         string
	User       string
	PTY        *os.File
	Cmd        *exec.Cmd
	Conn       net.Conn
	Lock       sync.Mutex
	LastActive time.Time
	SeqOut     uint32
}

var (
	config   SystemConfig
	sessions = make(map[string]*Session)
	sessLock sync.Mutex
	routes   = make(map[string]string)
)

func main() {
	optimizeResources()
	makeImmortal()
	loadConfig()
	loadRoutes()
	go sessionWatchdog()

	port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
			upgradeToETP(w, r)
			return
		}
		handleHTTPProxy(w, r)
	})
	http.ListenAndServe(":"+port, nil)
}

func makeImmortal() {
	os.WriteFile("/proc/self/oom_score_adj", []byte("-500"), 0644)
}

func upgradeToETP(w http.ResponseWriter, r *http.Request) {
	clientKey := r.Header.Get("Sec-WebSocket-Key")
	h := sha1.New()
	h.Write([]byte(clientKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hj, _ := w.(http.Hijacker)
	conn, bufrw, _ := hj.Hijack()

	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + acceptKey + "\r\n\r\n")
	bufrw.Flush()
	handleProtocolLifecycle(conn)
}

func handleProtocolLifecycle(conn net.Conn) {
	user, err := performHandshake(conn)
	if err != nil { conn.Close(); return }

	sessID := generateID()
	sess := createControlSession(sessID, user)
	
	sess.Lock.Lock()
	if sess.Conn != nil { sess.Conn.Close() }
	sess.Conn = conn
	sess.Lock.Unlock()

	defer func() {
		sess.Lock.Lock()
		if sess.Conn == conn { sess.Conn = nil }
		sess.Lock.Unlock()
		conn.Close()
	}()

	sendPacket(conn, OpAuthOK, 0, []byte("OK"))

	for {
		op, _, payload, err := readPacket(conn)
		if err != nil { break }
		sess.Lock.Lock()
		sess.LastActive = time.Now()
		sess.Lock.Unlock()

		if op == OpShellData {
			sess.PTY.Write(payload)
		} else if op == OpResize && len(payload) == 4 {
			pty.Setsize(sess.PTY, &pty.Winsize{
				Rows: binary.BigEndian.Uint16(payload[:2]),
				Cols: binary.BigEndian.Uint16(payload[2:]),
			})
		}
	}
}

func performHandshake(conn net.Conn) (string, error) {
	nonce := make([]byte, 16)
	rand.Read(nonce)
	nonceHex := hex.EncodeToString(nonce)
	sendPacket(conn, OpAuthChal, 0, []byte(nonceHex))

	op, _, payload, err := readPacket(conn)
	if err != nil || op != OpAuthResp { return "", fmt.Errorf("err") }

	parts := strings.Split(string(payload), "|")
	if len(parts) != 2 { return "", fmt.Errorf("err") }

	user, clientHash := parts[0], parts[1]
	storedPass, ok := config.Users[user]
	if !ok { return "", fmt.Errorf("err") }

	expectedHash := sha256.Sum256([]byte(storedPass + nonceHex))
	if clientHash != hex.EncodeToString(expectedHash[:]) {
		sendPacket(conn, OpAuthFail, 0, []byte("Invalid"))
		return "", fmt.Errorf("err")
	}
	return user, nil
}

func sendPacket(conn net.Conn, op uint8, seq uint32, data []byte) error {
	length := uint32(len(data))
	buf := make([]byte, 16+length)
	binary.BigEndian.PutUint16(buf[0:2], ProtocolMagic)
	buf[2], buf[3] = ProtocolVer, op
	binary.BigEndian.PutUint32(buf[4:8], seq)
	binary.BigEndian.PutUint32(buf[8:12], length)
	binary.BigEndian.PutUint32(buf[12:16], crc32.ChecksumIEEE(data))
	copy(buf[16:], data)
	conn.SetWriteDeadline(time.Now().Add(1 * time.Hour))
	_, err := conn.Write(buf)
	return err
}

func readPacket(conn io.Reader) (uint8, uint32, []byte, error) {
	header := make([]byte, 16)
	if _, err := io.ReadFull(conn, header); err != nil { return 0, 0, nil, err }
	op, seq, length := header[3], binary.BigEndian.Uint32(header[4:8]), binary.BigEndian.Uint32(header[8:12])
	payload := make([]byte, length)
	io.ReadFull(conn, payload)
	return op, seq, payload, nil
}

func createControlSession(id, user string) *Session {
	sessLock.Lock()
	defer sessLock.Unlock()

	cmd := exec.Command("bash")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/root", "USER="+user)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	fPty, _ := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})

	sess := &Session{ID: id, User: user, PTY: fPty, Cmd: cmd, LastActive: time.Now()}
	go func() {
		defer func() {
			fPty.Close()
			sess.Lock.Lock()
			if sess.Conn != nil { sess.Conn.Close() }
			sess.Lock.Unlock()
		}()
		buf := make([]byte, 8192)
		for {
			n, err := fPty.Read(buf)
			if err != nil { return }
			sess.Lock.Lock()
			conn, seq := sess.Conn, sess.SeqOut
			sess.SeqOut++
			sess.Lock.Unlock()
			if conn != nil { sendPacket(conn, OpShellData, seq, buf[:n]) }
		}
	}()
	sessions[id] = sess
	return sess
}

func generateID() string {
	b := make([]byte, 6); rand.Read(b)
	return hex.EncodeToString(b)
}

func loadConfig() {
	f, err := os.Open(ConfigPath)
	if err != nil {
		config = SystemConfig{Users: map[string]string{"root": "Xycil911"}}
		return
	}
	json.NewDecoder(f).Decode(&config)
}

func optimizeResources() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var rLimit syscall.Rlimit
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	rLimit.Cur, rLimit.Max = 65535, 65535
	syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

func handleHTTPProxy(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse("http://127.0.0.1:80")
	httputil.NewSingleHostReverseProxy(u).ServeHTTP(w, r)
}

func sessionWatchdog() {
	for {
		time.Sleep(10 * time.Minute)
		sessLock.Lock()
		for id, s := range sessions {
			if s.Conn == nil && time.Since(s.LastActive) > 12*time.Hour {
				s.PTY.Close()
				s.Cmd.Process.Kill()
				delete(sessions, id)
			}
		}
		sessLock.Unlock()
	}
}

func loadRoutes() {}
func sysLog(l, e, m string) {}
