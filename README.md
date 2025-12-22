
![Build Status](https://img.shields.io/badge/build-passing-brightgreen)
![Go Version](https://img.shields.io/badge/go-1.21-blue)
![Python Version](https://img.shields.io/badge/python-3.x-yellow)
![Mode](https://img.shields.io/badge/mode-NITRO-red)

**SVPS** adalah tiruan VPS berbasis kontainer yang dirancang untuk lingkungan PaaS (Zeabur, Railway, Render). Ini mengubah kontainer Docker biasa menjadi *remote shell* penuh dengan performa yang dipaksa maksimal (*Overclocked*).

> **Warning:** Alat ini dilengkapi yang memaksa penggunaan seluruh Core CPU dan menaikkan limit File Descriptor. Gunakan dengan bijak.

---

## 🔥 Fitur Utama

### Server Side (Engine)
* **[span_0](start_span)NITRO Mode:** Memaksa runtime Go menggunakan seluruh `numCPU` yang tersedia dan menaikkan `ulimit` ke 65535[span_0](end_span).
* **[span_1](start_span)Heartbeat System:** Melakukan *self-ping* setiap 2 menit untuk mencegah penyedia hosting mematikan kontainer karena *idling*[span_1](end_span).
* **Dual Face Architecture:**
    * `GET /`: Reverse Proxy ke aplikasi lokal (port 3000) atau tampilan status sistem.
    * [span_2](start_span)`WS /sussh`: Jalur masuk WebSocket Shell terenkripsi[span_2](end_span).
* **[span_3](start_span)Full PTY Support:** Mendukung `vim`, `htop`, `nano`, dan interaksi terminal penuh[span_3](end_span).

### Client Side (Sussh)
* **[span_4](start_span)Smart Paste:** Mendukung copy-paste teks panjang (hingga 4KB chunk) tanpa lag atau karakter hilang[span_4](end_span).
* **[span_5](start_span)File Upload:** Transfer file dari lokal ke server tanpa SCP/FTP, murni via WebSocket stream[span_5](end_span).
* **[span_6](start_span)Profile Manager:** Simpan target dan kredensial untuk akses cepat[span_6](end_span).

---

## 🛠️ Instalasi & Deployment

### 1. Server (Deploy ke PaaS)
Gunakan `Dockerfile` yang tersedia. [span_7](start_span)Kontainer berbasis `ubuntu:22.04` dan sudah menyertakan `curl`, `git`, `vim`, `htop`, dll[span_7](end_span).

**Wajib Set Environment Variables:**
Agar SVPS berjalan aman dan sesuai identitasmu, atur variabel berikut di dashboard hostingmu (Zeabur/Railway/dll):

| Variable | Wajib? | Deskripsi | Default |
| :--- | :---: | :--- | :--- |
| `PASS` | **YA** | Token/Password rahasia untuk akses shell. Jika salah, akses ditolak (403). | - |
| `NAMES` | Tidak | Nama user yang tampil di terminal prompt. | `ROOT` |
| `ALIASE` | Tidak | Nama host/alias yang tampil di terminal prompt. | `VPS` |
| `PORT` | Tidak | Port aplikasi berjalan. | `8080` |

Cntoh Tampilan Terminal jika `NAMES=Xycan` dan `ALIASE=Eternals`:
[span_8](start_span)`Xycan@Eternals:~/ $`[span_8](end_span)

### 2. Client (Local Machine)
Pastikan Python 3 terinstal, lalu instal `sussh` client:

```pip
# Install tool
pip install itssussh
```

Dependencies: websocket-client
🚀 Cara Penggunaan
Connect ke SVPS
Setelah server aktif dan Environment Variable diset, jalankan perintah berikut di terminal lokalmu:
Cara Langsung:
```
sussh <domain-target> -p <password-kamu>
```
Contoh: sussh my-svps.zeabur.app -p pwmu
Cara Menu Interaktif:
Cukup ketik sussh tanpa argumen untuk masuk ke menu profil:
```
$ sussh
Select:
 1) Project-Zeabur
 2) Server-Kantor
 N) New
 U) Upload
>
```

Upload File
Pilih opsi U pada menu atau gunakan mode upload. File akan diubah ke base64 dan direkonstruksi ulang di server secara otomatis.
```
> Local path: /home/user/script.py
> Remote path (opt): /root/script.py
Uploading... 
Done
```

⚠️ Struktur Direktori
```
├── Dockerfile          # Multi-stage build (Go Builder - Ubuntu Runtime)
├── go.mod              # Go Dependencies
├── core/
│   └── main.go         # code Server (Engine)
├── sussh/              # code Client (Python Package)
│   ├── core.py         # Logika utama Client
│   └── ...
└── setup.py            # Instller Client
```

🛡️ Disclaimer
Tool ini dibuat oleh Eternals untuk tujuan edukasi dan administrasi sistem secara efisien. Penggunaan untuk aktivitas ilegal di luar tanggung jawab pembuat.
Powered by Eternals|Vlazars.