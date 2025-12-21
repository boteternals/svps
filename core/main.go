package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	// Ambil kunci dari Environment Variable
	serverKey := os.Getenv("KEYS")
	clientKey := r.Header.Get("X-SVPS-TOKEN")

	// Validasi token
	if serverKey == "" || clientKey != serverKey {
		log.Printf("[!] Unauthorized access from: %s", r.RemoteAddr)
		http.NotFound(w, r)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[!] Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Siapkan command bash
	c := exec.Command("bash")
	c.Env = append(os.Environ(), "TERM=xterm-256color")

	// Mulai PTY
	f, err := pty.Start(c)
	if err != nil {
		log.Printf("[!] PTY Start error: %v", err)
		return
	}
	defer f.Close()

	// Goroutine: Baca dari WebSocket -> Tulis ke Terminal (Input User)
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			f.Write(message)
		}
	}()

	// Loop Utama: Baca dari Terminal -> Tulis ke WebSocket (Output Layar)
	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		if err != nil {
			// Kirim sinyal tutup jika bash mati
			conn.WriteMessage(websocket.TextMessage, []byte("\r\n[SVPS] Session Closed.\r\n"))
			break
		}
		err = conn.WriteMessage(websocket.TextMessage, buf[:n])
		if err != nil {
			break
		}
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/sussh", handleSussh)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("SVPS Engine is active."))
	})

	log.Printf("[*] SVPS Engine running on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

