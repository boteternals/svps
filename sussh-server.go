package main

import (
    "flag"
    "log"
    "net/http"
    "os/exec"
    "github.com/creack/pty"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

func handleSussh(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Print("Upgrade error:", err)
        return
    }
    defer conn.Close()

    // Membuka shell bash asli di dalam kontainer
    c := exec.Command("bash")
    f, err := pty.Start(c)
    if err != nil {
        log.Print("PTY start error:", err)
        return
    }

    // Bridge antara WebSocket dan Bash
    go func() {
        for {
            _, message, err := conn.ReadMessage()
            if err != nil { return }
            f.Write(message)
        }
    }()

    buf := make([]byte, 1024)
    for {
        n, err := f.Read(buf)
        if err != nil { return }
        conn.WriteMessage(websocket.TextMessage, buf[:n])
    }
}

func main() {
    port := flag.String("port", "8080", "Port to listen on")
    flag.Parse()

    http.HandleFunc("/sussh", handleSussh)
    log.Printf("SVPS Engine (sussh) started on port %s", *port)
    log.Fatal(http.ListenAndServe(":"+*port, nil))
}
