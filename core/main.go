package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// KONSTANTA VERSI (Protocol Lock)
const SVPS_PROTOCOL_VERSION = "3.0"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	// 1. CEK VERSI PROTOKOL (Wajib Update)
	clientVersion := r.Header.Get("X-SVPS-VERSION")
	if clientVersion != SVPS_PROTOCOL_VERSION {
		log.Printf("[!] Version Mismatch. Client: %s, Server: %s", clientVersion, SVPS_PROTOCOL_VERSION)
		http.Error(w, "CLIENT_OUTDATED_PLEASE_UPDATE", http.StatusUpgradeRequired) // Error 426
		return
	}

	// 2. Ambil Config dari ENV (Sesuai Request Kamu)
	serverPass := os.Getenv("PASS")     // Dulu KEYS
	serverName := os.Getenv("NAMES")    // Username Default
	serverAlias := os.Getenv("ALIASE")  // Hostname Palsu (Branding)

	// Fallback jika lupa setting ENV
	if serverName == "" { serverName = "root" }
	if serverAlias == "" { serverAlias = "svps-box" }

	// 3. Validasi Password
	clientKey := r.Header.Get("X-SVPS-TOKEN")
	if serverPass != "" && clientKey != serverPass { // Jika PASS kosong, anggap open (opsional) atau tolak
		if serverPass != "" {
			log.Printf("[!] Unauthorized access attempt from: %s", r.RemoteAddr)
			http.NotFound(w, r) // Kasih 404 biar bingung
			return
		}
	}

	// 4. Tentukan Username Tampilan
	// Priority: Request Client > ENV NAMES
	clientRequestUser := r.Header.Get("X-SVPS-USER")
	finalUser := serverName
	if clientRequestUser != "" && clientRequestUser != "root" {
		finalUser = clientRequestUser
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }
	defer conn.Close()

	// 5. MANIPULASI TAMPILAN PROMPT (THE MAGIC) 🪄
	c := exec.Command("bash")
	
	// Kita rakit PS1 secara manual.
	// Format: [Hijau]User@[Ungu]Alias:[Biru]Path[Reset]$ 
	// \w = Path (menampilkan ~ jika di home)
	customPrompt := fmt.Sprintf("PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", finalUser, serverAlias)

	// Inject ke Environment
	c.Env = append(os.Environ(), 
		"TERM=xterm-256color",
		"HOME=/root",
		customPrompt, // Ini yang bikin hostname panjang hilang!
	)

	f, err := pty.Start(c)
	if err != nil { return }
	defer f.Close()

	// Bridge WebSocket
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil { return }
			f.Write(msg)
		}
	}()

	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		if err != nil { break }
		conn.WriteMessage(websocket.TextMessage, buf[:n])
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	http.HandleFunc("/sussh", handleSussh)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("SVPS Secure Core v3.0"))
	})

	log.Printf("[*] SVPS v3.0 Running on port %s", port)
	http.ListenAndServe(":"+port, nil)
}

