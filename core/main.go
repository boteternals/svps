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

// --- CONFIGURATION ---

const (
	EngineVer      = "13.0-IRONCLAD"
	ConfigPath     = "/etc/svps/config.json"
	ProtocolMagic  = 0x5650 // 'V' 'P'
	ProtocolVer    = 0x0D   // 13
	MaxPacketSize  = 1024 * 1024 // 1MB Limit
	
	// OPCODES (Internal Control Protocol)
	OpAuthInit     = 0x01
	OpAuthChal     = 0x02
	OpAuthResp     = 0x03
	OpAuthOK       = 0x04
	OpAuthFail     = 0x05
	OpShellData    = 0x10
	OpResize       = 0x11
	OpPing         = 0x99
)

type SystemConfig struct {
	Users map[string]string `json:"users"`
}

type PacketHeader struct {
	Magic    uint16
	Version  uint8
	OpCode   uint8
	Sequence uint32
	Length   uint32
	Checksum uint32
}

type Session struct {
	ID         string
	User       string
	PTY        *os.File
	Cmd        *exec.Cmd
	Conn       net.Conn
	Lock       sync.Mutex
	LastActive time.Time
	SeqIn      uint32
	SeqOut     uint32
}

var (
	config   SystemConfig
	sessions = make(map[string]*Session)
	sessLock sync.Mutex
	routes   = make(map[string]string)
	publicIP string
)

// --- INITIALIZATION ---

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
}

func main() {
	sysLog("INFO", "SystemBoot", "Initializing SVPS V13 Control Plane...")
	
	optimizeResources()
	loadConfig()
	loadRoutes()
	go resolvePublicIP()
	go sessionWatchdog()

	port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	http.HandleFunc("/", trafficDispatcher)

	sysLog("INFO", "NetworkReady", fmt.Sprintf("Listening on port %s", port))
	http.ListenAndServe(":"+port, nil)
}

// --- TRANSPORT LAYER (The Gatekeeper) ---

func trafficDispatcher(w http.ResponseWriter, r *http.Request) {
	// 1. WebSocket Upgrade Detection (Strict RFC 6455)
	// Kita mematuhi handshake WS hanya agar Load Balancer Zeabur mengizinkan lewat.
	// Setelah handshake selesai, kita beralih ke Binary ETP.
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	if upgrade == "websocket" {
		upgradeToETP(w, r)
		return
	}

	// 2. HTTP Reverse Proxy (Fallback / Web)
	handleHTTPProxy(w, r)
}

func upgradeToETP(w http.ResponseWriter, r *http.Request) {
	clientKey := r.Header.Get("Sec-WebSocket-Key")
	if clientKey == "" {
		http.Error(w, "Bad Request", 400)
		return
	}

	// Calculate Accept Key
	h := sha1.New()
	h.Write([]byte(clientKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Internal Error", 500)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil { return }

	// Send 101 Switching Protocols
	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: " + acceptKey + "\r\n")
	bufrw.WriteString("\r\n")
	bufrw.Flush()

	// Handover to Protocol Layer
	handleProtocolLifecycle(conn)
}

// --- PROTOCOL LAYER (The Translator) ---

func handleProtocolLifecycle(conn net.Conn) {
	defer conn.Close()
	
	// PHASE 1: AUTHENTICATION
	user, err := performHandshake(conn)
	if err != nil {
		sysLog("WARN", "AuthFailed", err.Error())
		return
	}

	// PHASE 2: SESSION ATTACHMENT
	// Generate or Retrieve Session
	sessID := generateID() // For V13, we enforce new session for security
	sess := createControlSession(sessID, user)
	if sess == nil { return }

	// Link Connection
	sess.Lock.Lock()
	if sess.Conn != nil { sess.Conn.Close() } // Force disconnect old client
	sess.Conn = conn
	sess.Lock.Unlock()

	// Notify Client
	sendPacket(conn, OpAuthOK, 0, []byte(sessID))
	sysLog("INFO", "SessionAttached", fmt.Sprintf("User:%s ID:%s", user, sessID))

	// PHASE 3: DATA LOOP
	for {
		op, _, payload, err := readPacket(conn)
		if err != nil { break }

		sess.Lock.Lock()
		sess.LastActive = time.Now()
		sess.Lock.Unlock()

		// Dispatch based on OpCode
		switch op {
		case OpShellData:
			sess.PTY.Write(payload)
		case OpResize:
			if len(payload) == 4 {
				rows := binary.BigEndian.Uint16(payload[:2])
				cols := binary.BigEndian.Uint16(payload[2:])
				pty.Setsize(sess.PTY, &pty.Winsize{Rows: rows, Cols: cols})
			}
		case OpPing:
			// Keepalive
		}
	}
}

func performHandshake(conn net.Conn) (string, error) {
	// 1. Send Challenge
	nonce := make([]byte, 16)
	rand.Read(nonce)
	nonceHex := hex.EncodeToString(nonce)
	
	if err := sendPacket(conn, OpAuthChal, 0, []byte(nonceHex)); err != nil {
		return "", err
	}

	// 2. Read Response
	op, _, payload, err := readPacket(conn)
	if err != nil || op != OpAuthResp {
		return "", fmt.Errorf("protocol violation during auth")
	}

	// Payload: "user|hash"
	data := string(payload)
	parts := strings.Split(data, "|")
	if len(parts) != 2 {
		sendPacket(conn, OpAuthFail, 0, []byte("Invalid Format"))
		return "", fmt.Errorf("invalid auth format")
	}

	user, clientHash := parts[0], parts[1]
	storedPass, ok := config.Users[user]
	if !ok {
		time.Sleep(200 * time.Millisecond) // Fake delay
		sendPacket(conn, OpAuthFail, 0, []byte("Invalid Creds"))
		return "", fmt.Errorf("unknown user %s", user)
	}

	// 3. Verify: SHA256(Pass + Nonce)
	expectedHash := sha256.Sum256([]byte(storedPass + nonceHex))
	expectedStr := hex.EncodeToString(expectedHash[:])

	if clientHash != expectedStr {
		sendPacket(conn, OpAuthFail, 0, []byte("Invalid Creds"))
		return "", fmt.Errorf("bad password for %s", user)
	}

	return user, nil
}

// --- PACKET HANDLING (Strict Framing & Integrity) ---

func sendPacket(conn net.Conn, op uint8, seq uint32, data []byte) error {
	length := uint32(len(data))
	// Header: Magic(2) + Ver(1) + Op(1) + Seq(4) + Len(4) + CRC(4) = 16 Bytes
	buf := make([]byte, 16+length)

	binary.BigEndian.PutUint16(buf[0:2], ProtocolMagic)
	buf[2] = ProtocolVer
	buf[3] = op
	binary.BigEndian.PutUint32(buf[4:8], seq)
	binary.BigEndian.PutUint32(buf[8:12], length)
	
	// Calculate CRC32 of Data
	crc := crc32.ChecksumIEEE(data)
	binary.BigEndian.PutUint32(buf[12:16], crc)

	copy(buf[16:], data)

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := conn.Write(buf)
	return err
}

func readPacket(conn io.Reader) (uint8, uint32, []byte, error) {
	header := make([]byte, 16)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, 0, nil, err
	}

	magic := binary.BigEndian.Uint16(header[0:2])
	if magic != ProtocolMagic {
		return 0, 0, nil, fmt.Errorf("invalid magic bytes")
	}

	op := header[3]
	seq := binary.BigEndian.Uint32(header[4:8])
	length := binary.BigEndian.Uint32(header[8:12])
	expectedCRC := binary.BigEndian.Uint32(header[12:16])

	if length > MaxPacketSize {
		return 0, 0, nil, fmt.Errorf("packet too large")
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, 0, nil, err
	}

	// Verify Integrity
	if crc32.ChecksumIEEE(payload) != expectedCRC {
		return 0, 0, nil, fmt.Errorf("checksum mismatch - data corrupted")
	}

	return op, seq, payload, nil
}

// --- CONTROL LAYER (PTY & Session) ---

func createControlSession(id, user string) *Session {
	sessLock.Lock()
	defer sessLock.Unlock()

	// Environment Injection (The Clean Way)
	// No .bashrc pollution. Direct memory injection.
	cmd := exec.Command("bash")
	
	// PS1 format: [user@public_ip]:~#
	prompt := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:~# '", user, publicIP)
	
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"HOME=/root",
		"USER="+user,
		"PROMPT_COMMAND="+prompt, // Force prompt every line
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	fPty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil { return nil }

	sess := &Session{ID: id, User: user, PTY: fPty, Cmd: cmd, LastActive: time.Now()}

	// Output Pump (PTY -> Network)
	go func() {
		defer func() {
			sessLock.Lock()
			delete(sessions, id)
			sessLock.Unlock()
			fPty.Close()
		}()

		buf := make([]byte, 4096)
		for {
			n, err := fPty.Read(buf)
			if err != nil { return } // Bash died

			sess.Lock.Lock()
			conn := sess.Conn
			seq := sess.SeqOut
			sess.SeqOut++
			sess.Lock.Unlock()

			if conn != nil {
				// Ignore errors here (if client disconnects, we just wait for reconnect)
				sendPacket(conn, OpShellData, seq, buf[:n])
			}
		}
	}()

	sessions[id] = sess
	return sess
}

// --- UTILITIES ---

func sysLog(level, event, msg string) {
	fmt.Printf(`{"ts":"%s","lvl":"%s","evt":"%s","msg":"%s"}`+"\n", 
		time.Now().Format(time.RFC3339), level, event, msg)
}

func generateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func loadConfig() {
	dir := filepath.Dir(ConfigPath)
	os.MkdirAll(dir, 0755)

	f, err := os.Open(ConfigPath)
	if err != nil {
		// Default secure config
		config = SystemConfig{Users: map[string]string{"root": "Xycil911"}}
		saveConfig()
		return
	}
	defer f.Close()
	json.NewDecoder(f).Decode(&config)
}

func saveConfig() {
	f, _ := os.Create(ConfigPath)
	defer f.Close()
	json.NewEncoder(f).Encode(config)
}

func resolvePublicIP() {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err == nil {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		publicIP = string(b)
	} else {
		publicIP = "127.0.0.1"
	}
}

func optimizeResources() {
	var rLimit syscall.Rlimit
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	rLimit.Cur = 65535
	rLimit.Max = 65535
	syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

func loadRoutes() {
	raw := os.Getenv("ROUTES")
	if raw == "" { return }
	for _, r := range strings.Split(raw, ";") {
		p := strings.Split(r, ":")
		if len(p) >= 2 { routes[strings.TrimSpace(p[0])] = strings.TrimSpace(p[1]) }
	}
}

func handleHTTPProxy(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil { host = h }
	target := "80"
	if t, ok := routes[host]; ok { target = t }
	u, _ := url.Parse("http://127.0.0.1:" + target)
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ServeHTTP(w, r)
}

func sessionWatchdog() {
	for {
		time.Sleep(10 * time.Minute)
		sessLock.Lock()
		for id, s := range sessions {
			s.Lock.Lock()
			if s.Conn == nil && time.Since(s.LastActive) > 12*time.Hour {
				s.PTY.Close()
				s.Cmd.Process.Kill()
				delete(sessions, id)
			}
			s.Lock.Unlock()
		}
		sessLock.Unlock()
	}
}
