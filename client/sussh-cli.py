import websocket
import threading
import sys
import os
import argparse

# Menangani output dari SVPS ke terminal kamu
def on_message(ws, message):
    sys.stdout.write(message if isinstance(message, str) else message.decode('utf-8'))
    sys.stdout.flush()

def on_error(ws, error):
    print(f"\n[!] Error: {error}")

def on_close(ws, close_status_code, close_msg):
    print("\n[!] Connection Closed.")

# Mengirim input dari keyboard kamu ke SVPS
def send_input(ws):
    while True:
        try:
            # Mengambil input karakter per karakter (agar interaktif)
            char = sys.stdin.read(1)
            if char:
                ws.send(char)
        except EOFError:
            break

def connect_svps(host, token):
    # Membangun URL WebSocket (wss:// untuk keamanan TLS)
    url = f"wss://{host}/sussh"
    
    headers = {
        "X-SVPS-TOKEN": token
    }

    print(f"[*] Connecting to SVPS at {host}...")
    
    ws = websocket.WebSocketApp(
        url,
        header=headers,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close
    )

    # Menjalankan thread untuk input agar tidak blocking
    input_thread = threading.Thread(target=send_input, args=(ws,))
    input_thread.daemon = True
    input_thread.start()

    ws.run_forever()

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="SVPS sussh-client")
    parser.add_argument("--host", required=True, help="Domain Zeabur (misal: svps.zeabur.app)")
    parser.add_argument("--token", required=True, help="Token rahasia SVPS_TOKEN")
    
    args = parser.parse_args()
    
    try:
        connect_svps(args.host, args.token)
    except KeyboardInterrupt:
        print("\n[!] Disconnected.")
        sys.exit(0)
