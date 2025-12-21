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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	serverKey := os.Getenv("KEYS")
	clientKey := r.Header.Get("X-SVPS-TOKEN")

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

	c := exec.Command("bash")
	c.Env = append(os.Environ(), "TERM=xterm-256color")

	f, err := pty.Start(c)
	if err != nil {
		log.Printf("[!] PTY Start error: %v", err)
		return
	}
	defer f.Close()

	// Goroutine untuk membaca dari WebSocket dan menulis ke PTY
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			f.Write(message)
		}
	}()

	// Membaca dari PTY dan menulis ke WebSocket
	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		if err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[SVPS] Session Closed.\r\n"))
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

