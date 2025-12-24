#!/usr/bin/python3
import sys
import os
import subprocess

# ANSI Colors for professional logging
GREEN = "\033[32m"
YELLOW = "\033[33m"
RED = "\033[31m"
RESET = "\033[0m"

def log(level, message):
    print(f"{level}[SVPS-SHIM] {message}{RESET}")

def main():
    if len(sys.argv) < 2:
        print("Usage: systemctl [command] [service]")
        return

    action = sys.argv[1]
    service = sys.argv[2] if len(sys.argv) > 2 else ""

    # COMPATIBILITY LAYER: Container Runtime Interception
    # Mencegah user menjalankan service docker daemon secara manual
    # karena kita menggunakan Podman (Daemonless).
    if service in ["docker", "podman"]:
        log(GREEN, "Container Runtime (Podman) is active and managed by kernel.")
        return

    # COMPATIBILITY LAYER: Legacy Init Fallback
    # Meneruskan perintah ke binary service legacy (/usr/sbin/service)
    # S6-Overlay menjaga PID 1, script ini menjaga user experience.
    
    cmd = f"service {service} {action}"
    
    log(GREEN, f"Executing control: {cmd}")
    
    try:
        result = os.system(cmd)
        if result != 0:
            # Silent fail is bad, explicit error is better
            log(RED, f"Operation failed with exit code {result}")
            sys.exit(result)
    except Exception as e:
        log(RED, f"System error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
