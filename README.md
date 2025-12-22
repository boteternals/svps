
![Build Status](https://img.shields.io/badge/build-passing-brightgreen)
![Go Version](https://img.shields.io/badge/go-1.21-blue)
![Python Version](https://img.shields.io/badge/python-3.x-yellow)
![Mode](https://img.shields.io/badge/mode-NITRO-red)

**SVPS** adalah tiruan VPS berbasis kontainer yang dirancang untuk lingkungan PaaS (Zeabur, Railway, Render). Ini mengubah kontainer Docker biasa menjadi *remote shell* penuh dengan performa yang dipaksa maksimal (*Overclocked*).

> **Warning:** Alat ini dilengkapi yang memaksa penggunaan seluruh Core CPU dan menaikkan limit File Descriptor. Gunakan dengan bijak.

---

## 🔥 Fitur Utama

### Server Side (Engine)
* **(start_span)NITRO Mode:** Memaksa runtime Go menggunakan seluruh `numCPU` yang tersedia dan menaikkan `ulimit` ke 65535(end_span).
* **(start_span)Heartbeat System:** Melakukan *self-ping* setiap 2 menit untuk mencegah penyedia hosting mematikan kontainer karena *idling*(end_span).
* **Dual Face Architecture:**
    * `GET /`: Reverse Proxy ke aplikasi lokal (port 3000) atau tampilan status sistem.
    * (start_span)`WS /sussh`: Jalur masuk WebSocket Shell terenkripsi(end_span).
* **(start_span)Full PTY Support:** Mendukung `vim`, `htop`, `nano`, dan interaksi terminal penuh(end_span).

### Client Side (Sussh)
* **(start_span)Smart Paste:** Mendukung copy-paste teks panjang (hingga 4KB chunk) tanpa lag atau karakter hilang(end_span).
* **(start_span)File Upload:** Transfer file dari lokal ke server tanpa SCP/FTP, murni via WebSocket stream(end_span).
* **(start_span)Profile Manager:** Simpan target dan kredensial untuk akses cepat(end_span).

---

## 🛠️ Instalasi & Deployment

### 1. Server (Deploy ke PaaS)
Gunakan `Dockerfile` yang tersedia. (start_span)Kontainer berbasis `ubuntu:22.04` dan sudah menyertakan `curl`, `git`, `vim`, `htop`, dll(end_span).

**Wajib Set Environment Variables:**
Agar SVPS berjalan aman dan sesuai identitasmu, atur variabel berikut di dashboard hostingmu (Zeabur/Railway/dll):

| Variable | Wajib? | Deskripsi | Default |
| :--- | :---: | :--- | :--- |
| `PASS` | **YA** | Token/Password rahasia untuk akses shell. Jika salah, akses ditolak (403). | - |
| `NAMES` | Tidak | Nama user yang tampil di terminal prompt. | `ROOT` |
| `ALIASE` | Tidak | Nama host/alias yang tampil di terminal prompt. | `VPS` |
| `PORT` | Tidak | Port aplikasi berjalan. | `8080` |

Cntoh Tampilan Terminal jika `NAMES=Xycan` dan `ALIASE=Eternals`:
(start_span)`Xycan@Eternals:~/ $`(end_span)

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

<p align="center">
  <a href="https://www.tiktok.com/@anakkecil_s">
    <img src="https://img.shields.io/badge/TikTok-%23000000.svg?style=for-the-badge&logo=TikTok&logoColor=white" />
  </a>
  <a href="mailto:helpme.eternals@gmail.com">
    <img src="https://img.shields.io/badge/Email-D14836?style=for-the-badge&logo=gmail&logoColor=white" />
  </a>
  <a href="https://whatsapp.com/channel/0029VaZLpqf8aKvHckUi4f1z">
    <img src="https://img.shields.io/badge/WhatsApp_Channel-25D366?style=for-the-badge&logo=whatsapp&logoColor=white" />
  </a>
</p>
