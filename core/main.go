package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// Konfigurasi WebSocket Upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Mengizinkan koneksi dari mana saja untuk fleksibilitas PaaS
		return true
	},
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	// 1. Ambil Kunci dari Environment Variable 'KEYS'
	serverKey := os.Getenv("KEYS")
	clientKey := r.Header.Get("X-SVPS-TOKEN")

	// Keamanan: Jika KEYS tidak diatur di Zeabur, tutup akses demi keamanan
	if serverKey == "" {
		log.Println("[!] ERROR: Environment variable 'KEYS' belum diatur di Zeabur!")
		http.Error(w, "Internal Server Configuration Error", http.StatusInternalServerError)
		return
	}

	// 2. Validasi Token (Ghost Mode)
	if clientKey != serverKey {
		log.Printf("[!] Unauthorized access attempt from: %s", r.RemoteAddr)
		// Menyamar sebagai 404 agar penyerang mengira endpoint tidak ada
		http.NotFound(w, r)
		return
	}

	// 3. Upgrade HTTP ke WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[!] Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[+] SVPS: Session established for %s", r.RemoteAddr)

	// 4. Inisialisasi Pseudo-Terminal (PTY)
	// Kita menjalankan /bin/bash sebagai shell utama
	c := exec.Command("bash")
	
	// Set environment agar mendukung warna terminal 256
	c.Env = append(os.Environ(), "TERM=xterm-256color")

	f, err := pty.Start(c)
	if err != nil {
		log.Printf("[!] PTY Start error: %v", err)
		return
	}
	defer f.Close()

	// 5. Bridge Data (Dua Arah)

	// Goroutine: Input dari Client (Keyboard) -> Terminal (Bash)
	go func() {
		_, _ = io.Copy(f, struct{ io.Reader }{r: &wsReader{conn}})
	}()

	// Output dari Terminal (Bash) -> Client (Layar)
	// Menggunakan buffer untuk efisiensi transfer data
	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		if err != nil {
			// Jika user mengetik 'exit', sesi berakhir
			_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[SVPS] Connection closed by remote host.\r\n"))
			break
		}
		// Kirim output terminal ke websocket client
		err = conn.WriteMessage(websocket.TextMessage, buf[:n])
		if err != nil {
			break
		}
	}
	log.Printf("[-] SVPS: Session ended for %s", r.RemoteAddr)
}

// Wrapper untuk membaca pesan dari WebSocket
type wsReader struct {
	conn *websocket.Conn
}

func (r *wsReader) Read(p []byte) (n int, err error) {
	_, message, err := r.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	copy(p, message)
	return len(message), nil
}

func main() {
	// Zeabur akan otomatis memberikan port melalui env PORT
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Endpoint Utama sussh
	http.HandleFunc("/sussh", handleSussh)

	// Penyamaran: Halaman utama sebagai decoy (umpan)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("SVPS Engine is active. System status: Healthy."))
	})

	log.Printf("[*] SVPS Engine (sussh) running on port %s", port)
	log.Printf("[*] Security: KEYS protection is ENABLED.")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[!] Failed to start server: %v", err)
	}
}
