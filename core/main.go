package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// KITA NAIKKAN VERSI BIAR JELAS
const SVPS_PROTOCOL_VERSION = "3.1"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
	// 1. SECURITY CHECK LEVEL 1: Cek Versi Client
	clientVersion := r.Header.Get("X-SVPS-VERSION")
	// Kita longgarkan sedikit, v3.0 dan v3.1 boleh masuk (backward compatible dikit)
	if clientVersion != "3.0" && clientVersion != "3.1" {
		log.Printf("[!] Tolak Client Versi: %s", clientVersion)
		http.Error(w, "CLIENT_OUTDATED_UPDATE_NOW", 426)
		return
	}

	// 2. SECURITY CHECK LEVEL 2: Wajib Ada Password Server
	serverPass := os.Getenv("PASS")
	if serverPass == "" {
		// JIKA PASS KOSONG -> MATIKAN AKSES. JANGAN BIARKAN MASUK.
		log.Println("[CRITICAL] ENV 'PASS' BELUM DISET DI ZEABUR!")
		http.Error(w, "SERVER_MISCONFIGURED_NO_PASS_SET", 500)
		return
	}

	// 3. SECURITY CHECK LEVEL 3: Validasi Password
	clientKey := r.Header.Get("X-SVPS-TOKEN")
	if clientKey != serverPass {
		log.Printf("[!] Password Salah dari: %s", r.RemoteAddr)
		// Kasih delay 1 detik biar brute force lambat
		time.Sleep(1 * time.Second)
		http.Error(w, "WRONG_PASSWORD_GET_OUT", 403)
		return
	}

	// 4. CONFIG NAMA (Branding)
	serverName := os.Getenv("NAMES")
	if serverName == "" { serverName = "ROOT" } // Default Uppercase biar beda
	
	serverAlias := os.Getenv("ALIASE")
	if serverAlias == "" { serverAlias = "BOX" }

	// Prioritas Nama: Request Client > ENV Server
	clientRequestUser := r.Header.Get("X-SVPS-USER")
	finalUser := serverName
	if clientRequestUser != "" && clientRequestUser != "root" {
		finalUser = clientRequestUser
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }
	defer conn.Close()

	// 5. MEMAKSA UBAH TAMPILAN (Force .bashrc overwrite)
	// Kita tulis langsung config prompt ke file startup bash
	// Warna: User(Hijau) @ Alias(Merah) : Path(Biru)
	customPS1 := fmt.Sprintf("export PS1='\\[\\033[01;32m\\]%s@%s\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]\\$ '", finalUser, serverAlias)
	
	// Tulis ke /root/.bashrc (Ini cara paling kejam dan pasti berhasil)
	fBash, _ := os.OpenFile("/root/.bashrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fBash != nil {
		fBash.WriteString("\n" + customPS1 + "\n")
		fBash.Close()
	}

	// Siapkan Command Bash
	c := exec.Command("bash")
	c.Env = append(os.Environ(), 
		"TERM=xterm-256color",
		"HOME=/root",
	)

	f, err := pty.Start(c)
	if err != nil { return }
	defer f.Close()

	// Kirim Banner Selamat Datang (Bukti kode baru jalan)
	welcomeMsg := fmt.Sprintf("\r\n\033[1;36m=== WELCOME TO SVPS SECURE SHELL v%s ===\033[0m\r\n", SVPS_PROTOCOL_VERSION)
	conn.WriteMessage(websocket.TextMessage, []byte(welcomeMsg))

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
		w.Write([]byte("SVPS v3.1 LOCKED & SECURE."))
	})

	log.Printf("[*] SVPS v3.1 Running on port %s", port)
	http.ListenAndServe(":"+port, nil)
}

