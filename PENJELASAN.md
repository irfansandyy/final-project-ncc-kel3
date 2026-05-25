# LAPORAN TEKNIS — Penjelasan Fungsi Kode
## Final Project NCC Laboratory 2026 · Kelompok 3
### SIEM — Security Information & Event Management

> **Tim:** E1 Mochammad Irfan Sandy · E2 Tuti Purwaningsih · E3 Lucky Himawan Prasetya  
> **Repository:** https://github.com/irfansandyy/final-project-ncc-kel3  
> **Tanggal Laporan:** Mei 2026

---

## Daftar Isi

1. [Gambaran Arsitektur Kode](#1-gambaran-arsitektur-kode)
2. [Dockerfile — Jenkins Custom Image](#2-dockerfile--jenkins-custom-image)
3. [docker-compose.yml — Orchestrasi Aplikasi Utama](#3-docker-composeyml--orchestrasi-aplikasi-utama)
4. [docker-compose.jenkins-sonarqube.yml — Stack CI/CD](#4-docker-composejenkins-sonarqubeyml--stack-cicd)
5. [Jenkinsfile — Pipeline CI/CD](#5-jenkinsfile--pipeline-cicd)
6. [.env.example — Konfigurasi Environment](#6-envexample--konfigurasi-environment)
7. [Backend — Arsitektur & Modul Go](#7-backend--arsitektur--modul-go)
8. [Frontend — Dashboard Next.js](#8-frontend--dashboard-nextjs)
9. [Database — Skema PostgreSQL](#9-database--skema-postgresql)
10. [Nginx — Reverse Proxy & TLS](#10-nginx--reverse-proxy--tls)
11. [scripts/ — Helper Scripts](#11-scripts--helper-scripts)
12. [Alur Data End-to-End](#12-alur-data-end-to-end)

---

## 1. Gambaran Arsitektur Kode

Repositori ini menerapkan pola **monorepo** — satu repository menyimpan semua komponen sistem. Setiap komponen berjalan sebagai container Docker yang saling terhubung melalui jaringan internal Docker Compose.

```
final-project-ncc-kel3/
├── Dockerfile                        ← Custom Jenkins image
├── Jenkinsfile                       ← Definisi pipeline CI/CD
├── docker-compose.yml                ← Orchestrasi layanan aplikasi
├── docker-compose.jenkins-sonarqube.yml ← Orchestrasi stack CI/CD
├── .env.example                      ← Template konfigurasi environment
├── .dockerignore                     ← Eksklusikan file dari Docker build context
├── backend/                          ← Layanan backend (Go)
├── frontend/                         ← Dashboard UI (Next.js)
├── database/init/                    ← SQL migration awal
├── nginx/                            ← Konfigurasi reverse proxy
├── metrics/                          ← Konfigurasi observability
└── scripts/                          ← Script bantu operasional
```

Bahasa utama berdasarkan statistik repository:
- **Go 57.4%** — seluruh backend service (collector, parser, API, WebSocket)
- **TypeScript 29.6%** — frontend dashboard Next.js
- **CSS 7.6%** — styling komponen UI
- **Shell 3.3%** — script operasional
- **Dockerfile 2.0%** — definisi container image

---

## 2. Dockerfile — Jenkins Custom Image

**File:** `Dockerfile`  
**Owner:** E3 Lucky Himawan Prasetya  
**Fungsi:** Membangun custom Jenkins image yang sudah dilengkapi Docker CLI dan plugin-plugin yang dibutuhkan pipeline.

```dockerfile
FROM jenkins/jenkins:2.541.3-jdk21

USER root
RUN apt-get update && apt-get install -y lsb-release ca-certificates curl && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg \
         -o /etc/apt/keyrings/docker.asc && \
    chmod a+r /etc/apt/keyrings/docker.asc && \
    echo "deb [arch=$(dpkg --print-architecture) \
         signed-by=/etc/apt/keyrings/docker.asc] \
         https://download.docker.com/linux/debian \
         $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    | tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && apt-get install -y docker-ce-cli && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

USER jenkins
RUN jenkins-plugin-cli --plugins \
    "blueocean docker-workflow json-path-api github github-branch-source"
```

### Penjelasan Baris per Baris

| Baris | Fungsi |
|---|---|
| `FROM jenkins/jenkins:2.541.3-jdk21` | Base image Jenkins versi LTS dengan JDK 21. Versi dipinning agar build reproducible. |
| `USER root` | Beralih ke root agar bisa install paket system-level. |
| `apt-get install lsb-release ca-certificates curl` | Paket bantu untuk menambahkan repository eksternal Docker secara aman. |
| `install -m 0755 -d /etc/apt/keyrings` | Membuat direktori untuk menyimpan GPG key repository Docker dengan permission yang benar. |
| `curl ... docker.asc` | Mengunduh dan menyimpan GPG public key Docker untuk verifikasi paket. |
| `echo "deb ..." \| tee docker.list` | Mendaftarkan repository resmi Docker CE ke apt sources, menggunakan arsitektur dan codename Debian yang sesuai secara otomatis. |
| `apt-get install docker-ce-cli` | Menginstall hanya **CLI** Docker (bukan daemon). Jenkins tidak menjalankan Docker sendiri — ia berkomunikasi dengan Docker daemon milik `jenkins-docker` (DinD) via TCP. |
| `apt-get clean && rm -rf /var/lib/apt/lists/*` | Membersihkan cache apt untuk meminimalkan ukuran final image. |
| `USER jenkins` | Kembali ke user `jenkins` (non-root) untuk menjalankan Jenkins secara aman. |
| `jenkins-plugin-cli --plugins "..."` | Menginstall plugin Jenkins yang dibutuhkan: **blueocean** (UI pipeline modern), **docker-workflow** (integrasi Docker dalam pipeline), **json-path-api** (parsing JSON di Groovy), **github** & **github-branch-source** (integrasi GitHub dan trigger webhook). |

---

## 3. docker-compose.yml — Orchestrasi Aplikasi Utama

**File:** `docker-compose.yml`  
**Owner:** E3 Lucky Himawan Prasetya  
**Fungsi:** Mendefinisikan dan mengorkestrasi semua layanan aplikasi produksi: database, backend, frontend, dan reverse proxy.

### 3.1 Service: `db` (PostgreSQL)

```yaml
db:
  image: postgres:16-alpine
  restart: unless-stopped
  env_file: .env
  environment:
    POSTGRES_DB: ${POSTGRES_DB}
    POSTGRES_USER: ${POSTGRES_USER}
    POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
  volumes:
    - pgdata:/var/lib/postgresql/data
    - ./database/init:/docker-entrypoint-initdb.d:ro
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB}"]
    interval: 10s
    timeout: 5s
    retries: 10
    start_period: 10s
```

| Konfigurasi | Fungsi |
|---|---|
| `image: postgres:16-alpine` | Menggunakan PostgreSQL 16 berbasis Alpine Linux — ringan (~80MB) dan aman. |
| `restart: unless-stopped` | Container otomatis restart jika crash, kecuali dihentikan manual. Menjamin availability. |
| `env_file: .env` | Membaca semua variabel dari file `.env` di root project. |
| `volumes pgdata` | Persistent volume — data database tidak hilang saat container di-recreate. |
| `volumes ./database/init:/docker-entrypoint-initdb.d:ro` | Mount folder SQL init ke direktori khusus PostgreSQL. Semua file `.sql` di sini dieksekusi otomatis saat database pertama kali dibuat. Flag `:ro` mencegah container memodifikasi file sumber. |
| `healthcheck pg_isready` | Mengecek apakah PostgreSQL siap menerima koneksi. Service lain (`backend`) hanya akan start setelah healthcheck ini **pass**. |

### 3.2 Service: `backend` (Go REST API)

```yaml
backend:
  build:
    context: ./backend
    dockerfile: Dockerfile
  restart: unless-stopped
  env_file: .env
  environment:
    APP_PORT: "80"
    DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable
    JWT_SECRET: ${JWT_SECRET}
    JWT_TTL_MINUTES: ${JWT_TTL_MINUTES}
    DB_MAX_OPEN_CONNS: ${DB_MAX_OPEN_CONNS}
    DB_MAX_IDLE_CONNS: ${DB_MAX_IDLE_CONNS}
    DB_CONN_MAX_LIFETIME_MINUTES: ${DB_CONN_MAX_LIFETIME_MINUTES}
    RATE_LIMIT_RPS: ${RATE_LIMIT_RPS}
    RATE_LIMIT_BURST: ${RATE_LIMIT_BURST}
    ALLOWED_ORIGIN: https://${DOMAIN:-localhost}
    LLM_BASE_URL: ${LLM_BASE_URL:-http://model-runner.docker.internal:12434/engines/v1}
    LLM_MODEL: ${LLM_MODEL_NAME:-hf.co/bartowski/Llama-3.2-3B-Instruct-GGUF:Q6_K}
    LLM_CTX_SIZE: ${LLM_CTX_SIZE:-4096}
    LLM_TIMEOUT_SECONDS: ${LLM_TIMEOUT_SECONDS}
  extra_hosts:
    - "model-runner.docker.internal:host-gateway"
  ports:
    - "${BACKEND_HOST_BIND:-127.0.0.1}:${BACKEND_HOST_PORT:-8000}:80"
  depends_on:
    db:
      condition: service_healthy
  healthcheck:
    test: ["CMD-SHELL", "curl --fail http://localhost/health || exit 1"]
    interval: 15s
    timeout: 5s
    retries: 8
    start_period: 20s
```

| Konfigurasi | Fungsi |
|---|---|
| `build context: ./backend` | Docker membangun image dari Dockerfile di folder `backend/`. |
| `APP_PORT: "80"` | Backend berjalan di port 80 di dalam container. |
| `DATABASE_URL` | Connection string PostgreSQL lengkap. Hostname `db` merujuk ke service `db` di jaringan Docker internal — tidak perlu IP statis. |
| `JWT_SECRET` | Secret key untuk signing/verifikasi JSON Web Token. Harus kuat dan rahasia. |
| `DB_MAX_OPEN_CONNS / DB_MAX_IDLE_CONNS` | Mengontrol connection pool ke database. Mencegah backend membuka terlalu banyak koneksi dan menguras resource DB. |
| `RATE_LIMIT_RPS / RATE_LIMIT_BURST` | Membatasi request per detik per client. Melindungi dari serangan DDoS / brute-force. |
| `ALLOWED_ORIGIN` | Mengizinkan CORS hanya dari domain produksi. Browser akan memblokir request dari origin lain. |
| `LLM_BASE_URL / LLM_MODEL / LLM_CTX_SIZE` | Konfigurasi integrasi dengan Docker Model Runner (LLM lokal). Menggunakan endpoint OpenAI-compatible. |
| `extra_hosts model-runner.docker.internal:host-gateway` | Memetakan hostname `model-runner.docker.internal` ke IP host machine, sehingga backend bisa menjangkau Docker Model Runner yang berjalan di host. |
| `ports BACKEND_HOST_BIND:8000:80` | Secara default hanya bind ke `127.0.0.1` (localhost). Backend tidak terekspos langsung ke internet — diakses melalui Nginx. |
| `depends_on db condition: service_healthy` | Backend tidak akan start sampai PostgreSQL benar-benar siap menerima koneksi. Mencegah error "database not ready" saat startup. |
| `healthcheck curl /health` | Memeriksa endpoint `/health` backend. Service `frontend` hanya start setelah ini pass. |

### 3.3 Service: `frontend` (Next.js)

```yaml
frontend:
  build:
    context: ./frontend
    dockerfile: Dockerfile
  restart: unless-stopped
  environment:
    NEXT_PUBLIC_API_BASE_URL: ${NEXT_PUBLIC_API_BASE_URL:-}
    HOSTNAME: 0.0.0.0
  ports:
    - "${FRONTEND_HOST_BIND:-127.0.0.1}:${FRONTEND_HOST_PORT:-3000}:3000"
  depends_on:
    backend:
      condition: service_healthy
```

| Konfigurasi | Fungsi |
|---|---|
| `NEXT_PUBLIC_API_BASE_URL` | URL base API untuk Next.js. Jika kosong, frontend melakukan request ke path relatif `/api` — diteruskan Nginx ke backend. Prefix `NEXT_PUBLIC_` membuat variabel ini tersedia di sisi browser (client-side). |
| `HOSTNAME: 0.0.0.0` | Menginstruksikan server Next.js untuk listen di semua interface jaringan di dalam container, bukan hanya localhost. |
| `depends_on backend service_healthy` | Frontend hanya start setelah backend siap, memastikan API sudah tersedia saat user pertama mengakses dashboard. |

### 3.4 Service: `nginx` (Reverse Proxy)

```yaml
nginx:
  image: nginx:1.27-alpine
  restart: unless-stopped
  depends_on:
    frontend:
      condition: service_healthy
    backend:
      condition: service_healthy
  ports:
    - "80:80"
    - "443:443"
  volumes:
    - ./nginx/nginx.conf:/etc/nginx/conf.d/default.conf:ro
    - ./nginx/certs:/etc/nginx/certs:ro
  extra_hosts:
    - "host.docker.internal:host-gateway"
  healthcheck:
    test: ["CMD-SHELL", "nginx -t"]
```

| Konfigurasi | Fungsi |
|---|---|
| `ports 80:80 dan 443:443` | Nginx adalah satu-satunya service yang mengekspos port ke internet. Semua traffic masuk melalui Nginx. |
| `volumes nginx.conf:ro` | Mount konfigurasi Nginx dari host. Flag `:ro` mencegah container mengubah konfigurasi. |
| `volumes nginx/certs:ro` | Mount direktori sertifikat TLS (fullchain.pem + privkey.pem). Nginx menggunakan ini untuk HTTPS. |
| `extra_hosts host.docker.internal` | Memungkinkan Nginx meneruskan request ke service lain di host machine jika diperlukan. |
| `healthcheck nginx -t` | Mengecek validitas konfigurasi Nginx (`nginx -t` = test config). Gagal jika ada syntax error. |
| `depends_on frontend + backend service_healthy` | Nginx hanya start setelah semua upstream (backend & frontend) siap, mencegah 502 Bad Gateway saat cold start. |

### 3.5 Volume & Dependency Chain

```
pgdata (persistent)
    │
    db ──healthy──▶ backend ──healthy──▶ frontend ──healthy──▶ nginx
                                                               │
                                                          port 80/443
                                                        (akses publik)
```

---

## 4. docker-compose.jenkins-sonarqube.yml — Stack CI/CD

**File:** `docker-compose.jenkins-sonarqube.yml`  
**Owner:** E3 Lucky Himawan Prasetya & E1 Mochammad Irfan Sandy
**Fungsi:** Menjalankan infrastruktur CI/CD yang terpisah dari aplikasi produksi — Jenkins, Docker-in-Docker, dan SonarQube.

### 4.1 Service: `jenkins-docker` (Docker-in-Docker / DinD)

```yaml
jenkins-docker:
  image: docker:dind
  container_name: jenkins-docker
  privileged: true
  restart: unless-stopped
  command:
    - --storage-driver
    - overlay2
  networks:
    jenkins:
      aliases:
        - docker
  environment:
    DOCKER_TLS_CERTDIR: /certs
  volumes:
    - jenkins-docker-certs:/certs/client
    - jenkins-data:/var/jenkins_home
```

| Konfigurasi | Fungsi |
|---|---|
| `image: docker:dind` | **Docker-in-Docker** — menjalankan Docker daemon di dalam container. Jenkins menggunakannya untuk build dan run container saat pipeline berjalan. |
| `privileged: true` | DinD membutuhkan mode privileged karena harus mengelola namespace kernel (cgroup, network). Ini diperlukan agar Docker daemon bisa berjalan di dalam container. |
| `command --storage-driver overlay2` | Menggunakan `overlay2` sebagai storage driver untuk layer image. Lebih efisien dibanding `vfs` di lingkungan Linux modern. |
| `networks aliases: docker` | Mendaftarkan hostname alias `docker` di jaringan jenkins. Jenkins akan terhubung ke DinD menggunakan `tcp://docker:2376`. |
| `DOCKER_TLS_CERTDIR: /certs` | Mengaktifkan TLS untuk komunikasi Docker TCP. Sertifikat dibuat otomatis di direktori ini. Mencegah akses tidak terotorisasi ke Docker daemon. |
| `volumes jenkins-docker-certs` | Shared volume antara DinD dan Jenkins untuk sertifikat TLS. Jenkins membaca sertifikat ini untuk autentikasi ke DinD. |
| `volumes jenkins-data` | Shared volume `/var/jenkins_home` antara DinD dan Jenkins. Memastikan file workspace Jenkins konsisten. |

### 4.2 Service: `jenkins-blueocean` (Jenkins)

```yaml
jenkins-blueocean:
  build:
    context: .
    dockerfile: Dockerfile
  image: myjenkins-blueocean:2.541.3-1
  container_name: jenkins-blueocean
  restart: on-failure
  depends_on:
    - jenkins-docker
  networks:
    - jenkins
  environment:
    DOCKER_HOST: tcp://docker:2376
    DOCKER_CERT_PATH: /certs/client
    DOCKER_TLS_VERIFY: "1"
    JENKINS_OPTS: --prefix=/jenkins
  ports:
    - "127.0.0.1:8080:8080"
    - "50000:50000"
  volumes:
    - jenkins-data:/var/jenkins_home
    - jenkins-docker-certs:/certs/client:ro
```

| Konfigurasi | Fungsi |
|---|---|
| `build context: . dockerfile: Dockerfile` | Membangun image Jenkins custom dari `Dockerfile` di root repo (yang sudah dijelaskan di bagian 2). |
| `restart: on-failure` | Jenkins hanya restart jika exit dengan error code, bukan jika dihentikan manual. |
| `DOCKER_HOST: tcp://docker:2376` | Mengarahkan Docker CLI di Jenkins untuk berkomunikasi dengan DinD (`docker` hostname) via TCP port 2376. |
| `DOCKER_CERT_PATH: /certs/client` | Path sertifikat TLS untuk autentikasi ke DinD. |
| `DOCKER_TLS_VERIFY: "1"` | Mengaktifkan verifikasi TLS — memastikan Jenkins hanya terhubung ke DinD yang terotorisasi. |
| `JENKINS_OPTS: --prefix=/jenkins` | Jenkins berjalan di sub-path `/jenkins`. URL akses menjadi `http://host:8080/jenkins`. Memudahkan konfigurasi reverse proxy ke Jenkins. |
| `ports 127.0.0.1:8080:8080` | Jenkins hanya terekspos ke localhost. Tidak bisa diakses langsung dari internet — harus melalui reverse proxy Nginx. |
| `ports 50000:50000` | Port komunikasi antara Jenkins controller dan Jenkins agent (JNLP). |

### 4.3 Service: `sonarqube`

```yaml
sonarqube:
  image: sonarqube
  container_name: sonarqube
  restart: unless-stopped
  networks:
    - jenkins
  environment:
    SONAR_WEB_CONTEXT: /sonarqube
    SONAR_CORE_SERVERBASEURL: https://${DOMAIN:-152.42.223.24}/sonarqube
  ports:
    - "127.0.0.1:9000:9000"
  volumes:
    - sonarqube_data:/opt/sonarqube/data
    - sonarqube_extensions:/opt/sonarqube/extensions
    - sonarqube_logs:/opt/sonarqube/logs
```

| Konfigurasi | Fungsi |
|---|---|
| `SONAR_WEB_CONTEXT: /sonarqube` | SonarQube berjalan di sub-path `/sonarqube`. Konsisten dengan konfigurasi Jenkins yang menggunakan `/jenkins`. |
| `SONAR_CORE_SERVERBASEURL` | URL publik SonarQube. Dibutuhkan untuk generate link yang benar di laporan dan webhook ke Jenkins. |
| `ports 127.0.0.1:9000:9000` | Sama seperti Jenkins, hanya terekspos ke localhost. |
| `volumes sonarqube_data` | Menyimpan database internal SonarQube (H2 atau PostgreSQL embedded). Data analisis tidak hilang saat restart. |
| `volumes sonarqube_extensions` | Plugin tambahan SonarQube tersimpan di sini. |
| `volumes sonarqube_logs` | Log SonarQube persistent untuk debugging. |

---

## 5. Jenkinsfile — Pipeline CI/CD

**File:** `Jenkinsfile`  
**Owner:** E3 Lucky Himawan Prasetya  
**Fungsi:** Mendefinisikan seluruh alur otomasi CI/CD dalam format **Declarative Pipeline** Groovy. Pipeline ini berjalan otomatis setiap kali ada push ke GitHub.

### 5.1 Header Pipeline — Options & Triggers

```groovy
pipeline {
  agent any
  options {
    timestamps()
    disableConcurrentBuilds()
  }
  triggers {
    githubPush()
  }
```

| Konfigurasi | Fungsi |
|---|---|
| `agent any` | Pipeline bisa berjalan di agent mana pun yang tersedia. |
| `timestamps()` | Menambahkan timestamp di setiap baris log output. Memudahkan debugging dan audit berapa lama setiap step berjalan. |
| `disableConcurrentBuilds()` | Mencegah dua build berjalan bersamaan. Menghindari race condition pada workspace dan registry. |
| `githubPush()` | Pipeline otomatis dipicu saat ada push ke GitHub (via webhook). |

### 5.2 Parameters

```groovy
parameters {
  string(name: 'GIT_BRANCH', defaultValue: 'main', ...)
  string(name: 'GIT_URL', defaultValue: 'https://github.com/...', ...)
  string(name: 'GIT_CREDENTIALS_ID', defaultValue: '', ...)
}
```

Parameter ini memungkinkan pipeline dijalankan manual dengan konfigurasi berbeda — misalnya checkout branch feature tertentu untuk testing, atau menggunakan credential untuk private repo.

### 5.3 Environment Variables

```groovy
environment {
  SONARQUBE_ENV = 'SonarQube'
  PROJECT_KEY   = 'tugas-ncc-irfansandy-backend'
  PROJECT_NAME  = 'tugas-ncc-irfansandy-fullstack'
  GO_DIR        = 'backend'
  FE_DIR        = 'frontend'
  GOFLAGS       = '-buildvcs=false'
  SCANNER_HOME  = tool 'SonarQube Scanner'
}
```

| Variabel | Fungsi |
|---|---|
| `SONARQUBE_ENV` | Nama konfigurasi SonarQube server di Jenkins (harus sama persis dengan yang dikonfigurasi di Manage Jenkins → System). |
| `PROJECT_KEY / PROJECT_NAME` | Identifier project di SonarQube. Hasil analisis ditampilkan di dashboard SonarQube dengan identitas ini. |
| `GO_DIR / FE_DIR` | Path relatif ke source code backend dan frontend. Digunakan berulang di setiap stage agar konsisten. |
| `GOFLAGS = '-buildvcs=false'` | Menonaktifkan pengecekan VCS metadata saat build Go. Diperlukan karena workspace Jenkins sering berjalan di direktori yang tidak dikenali sebagai Git repo oleh Go toolchain. |
| `SCANNER_HOME = tool 'SonarQube Scanner'` | Mengambil path instalasi SonarQube Scanner yang sudah dikonfigurasi di Jenkins Tools. |

### 5.4 Stage: Checkout

```groovy
stage('Checkout') {
  steps {
    script {
      try {
        deleteDir()
      } catch (Exception cleanupErr) {
        sh '''docker run --rm -u root -v "${WORKSPACE}:/workspace" alpine:3.21 \
          sh -c 'chown -R 1000:1000 /workspace || true; chmod -R u+rwX /workspace || true' '''
        deleteDir()
      }
      if (params.GIT_CREDENTIALS_ID?.trim()) {
        git branch: params.GIT_BRANCH, url: params.GIT_URL,
            credentialsId: params.GIT_CREDENTIALS_ID
      } else {
        git branch: params.GIT_BRANCH, url: params.GIT_URL
      }
    }
  }
}
```

**Fungsi:** Membersihkan workspace dan clone source code terbaru dari GitHub.

| Blok | Fungsi |
|---|---|
| `deleteDir()` | Menghapus seluruh isi workspace dari build sebelumnya. Memastikan build bersih dan tidak ada artefak lama. |
| `try-catch` dengan `docker run alpine chown` | Jika `deleteDir()` gagal karena file/folder milik root (sering terjadi saat Docker container menulis file), script menggunakan container Alpine sementara untuk memperbaiki ownership, lalu mencoba `deleteDir()` kembali. |
| `params.GIT_CREDENTIALS_ID?.trim()` | Operator `?.` adalah Groovy null-safe. Jika credential ID diisi, git clone dengan autentikasi (untuk private repo). |

### 5.5 Stage: Setup (Backend Dependencies)

```groovy
stage('Setup') {
  agent {
    docker {
      image 'golang:1.23-bookworm'
      args '-e HOME=/tmp -e GOCACHE=/tmp/go-cache -e GOPATH=/tmp/go
            -v /var/jenkins_home/tools:/var/jenkins_home/tools'
      reuseNode true
    }
  }
  steps {
    sh '''
      git config --global --add safe.directory "${WORKSPACE}"
      cd "${GO_DIR}"
      go version
      go mod download
    '''
  }
}
```

**Fungsi:** Mendownload semua dependency Go yang didefinisikan di `go.mod`.

| Konfigurasi | Fungsi |
|---|---|
| `image 'golang:1.23-bookworm'` | Menjalankan step ini di dalam container Go 1.23 berbasis Debian Bookworm. Memastikan versi Go konsisten di semua environment. |
| `-e HOME=/tmp -e GOCACHE=/tmp/go-cache -e GOPATH=/tmp/go` | Mengarahkan semua direktori Go (cache, path) ke `/tmp` agar bisa ditulis oleh user non-root di dalam container. |
| `-v /var/jenkins_home/tools:/var/jenkins_home/tools` | Mount tools Jenkins agar SonarQube Scanner yang terinstall di Jenkins bisa diakses dari dalam container. |
| `reuseNode true` | Container berjalan di node Jenkins yang sama (reuse workspace). |
| `git config safe.directory` | Menandai workspace sebagai direktori Git yang aman di dalam container — menghindari error "dubious ownership". |
| `go mod download` | Mendownload semua dependency yang terdaftar di `go.sum`. Dijalankan sekali di awal agar cache tersedia untuk stage-stage berikutnya. |

### 5.6 Stage: Backend Build & Test (Parallel)

```groovy
stage('Backend Build & Test') {
  parallel {
    stage('Build') {
      agent { docker { image 'golang:1.23-bookworm' ... } }
      steps { sh 'go build -v ./...' }
    }
    stage('Test') {
      agent { docker { image 'golang:1.23-bookworm' ... } }
      steps { sh 'go test ./... -v -coverprofile=coverage.out' }
    }
  }
}
```

**Fungsi:** Mengkompilasi dan menjalankan test suite backend secara **paralel** untuk mempersingkat waktu pipeline.

| Step | Fungsi |
|---|---|
| `go build -v ./...` | Mengkompilasi semua package Go di direktori `backend/`. Flag `-v` menampilkan nama package yang dikompilasi. Jika ada error syntax atau import, tahap ini gagal dan pipeline dihentikan. |
| `go test ./... -v -coverprofile=coverage.out` | Menjalankan semua unit test (`*_test.go`). Flag `-v` menampilkan nama setiap test. `-coverprofile=coverage.out` menghasilkan file laporan coverage yang akan diupload ke SonarQube. |
| `parallel {}` | Build dan Test berjalan bersamaan. Total waktu ≈ max(waktu_build, waktu_test), bukan waktu_build + waktu_test. |

### 5.7 Stage: Frontend Install, Lint, Build

```groovy
stage('Frontend Install') {
  agent { docker { image 'node:22-bookworm' args '-e HOME=/tmp' reuseNode true } }
  steps {
    sh '''cd "${FE_DIR}"
          if [ -f package-lock.json ]; then npm ci; else npm install; fi'''
  }
}

stage('Frontend Lint') {
  steps {
    sh '''cd "${FE_DIR}"
          if npx --yes next --help 2>&1 | grep -Eq 'lint'; then
            npm run lint
          else
            echo "next lint tidak tersedia, skip"
          fi'''
  }
}

stage('Frontend Build') {
  steps { sh 'cd "${FE_DIR}" && npm run build' }
}
```

| Stage | Fungsi |
|---|---|
| **Frontend Install** | Menginstall dependency Node.js. `npm ci` lebih ketat dari `npm install` — menggunakan `package-lock.json` secara deterministik, memastikan dependency yang sama persis di setiap build. |
| **Frontend Lint** | Menjalankan ESLint via `next lint`. Cek kondisi `grep -Eq 'lint'` memastikan perintah lint tersedia di versi Next.js yang digunakan sebelum dipanggil — menghindari false failure. |
| **Frontend Build** | Menjalankan `next build` yang mengkompilasi semua halaman dan komponen TypeScript/TSX menjadi bundle JavaScript yang dioptimasi untuk produksi. |

### 5.8 Stage: SonarQube Analysis

```groovy
stage('SonarQube Analysis') {
  steps {
    withSonarQubeEnv("${SONARQUBE_ENV}") {
      sh '''
        "${SCANNER_HOME}"/bin/sonar-scanner \
          -Dsonar.projectKey="${PROJECT_KEY}" \
          -Dsonar.projectName="${PROJECT_NAME}" \
          -Dsonar.sources=backend,frontend \
          -Dsonar.tests=backend,frontend \
          -Dsonar.test.inclusions=backend/**/*_test.go,... \
          -Dsonar.exclusions=frontend/.next/**,frontend/node_modules/** \
          -Dsonar.go.coverage.reportPaths=backend/coverage.out
      '''
    }
  }
}
```

**Fungsi:** Mengirimkan seluruh source code ke SonarQube untuk analisis kualitas dan keamanan.

| Parameter | Fungsi |
|---|---|
| `withSonarQubeEnv(...)` | Wrapper Jenkins yang secara otomatis menyuntikkan token autentikasi SonarQube dan URL server ke environment. |
| `sonar.sources=backend,frontend` | Mendefinisikan direktori source code yang akan dianalisis. |
| `sonar.test.inclusions=.../*_test.go,.../*.spec.ts` | Pola file yang dianggap sebagai test (bukan source). SonarQube tidak menghitung test file sebagai code smell. |
| `sonar.exclusions=frontend/.next/**` | Mengecualikan output build Next.js dan `node_modules` dari analisis. Folder-folder ini berisi kode generated/third-party. |
| `sonar.go.coverage.reportPaths=backend/coverage.out` | Mengarahkan SonarQube ke file coverage yang dihasilkan `go test`. Coverage ditampilkan di dashboard SonarQube. |

### 5.9 Stage: Quality Gate

```groovy
stage('Quality Gate') {
  steps {
    timeout(time: 20, unit: 'MINUTES') {
      waitForQualityGate abortPipeline: true
    }
  }
}
```

**Fungsi:** Menunggu SonarQube menyelesaikan analisis dan memutuskan apakah kode lolos standar kualitas.

| Konfigurasi | Fungsi |
|---|---|
| `timeout(20, MINUTES)` | Jika SonarQube tidak merespons dalam 20 menit, pipeline dianggap gagal. Mencegah pipeline menggantung tanpa batas. |
| `waitForQualityGate` | Pipeline berhenti dan menunggu webhook dari SonarQube yang memberitahu hasil quality gate. |
| `abortPipeline: true` | Jika quality gate **gagal** (misal: coverage < 70% atau ada blocker issue), pipeline secara otomatis dihentikan dengan status **FAILED**. Merge ke main tidak bisa dilakukan. |

---

## 6. .env.example — Konfigurasi Environment

**File:** `.env.example`  
**Fungsi:** Template semua variabel environment yang dibutuhkan seluruh sistem. Di-copy ke `.env` sebelum menjalankan stack.

### 6.1 Konfigurasi Database

```env
POSTGRES_DB=chatdb
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
```

Nama database, user, dan password PostgreSQL.

### 6.2 Konfigurasi Backend

```env
JWT_SECRET=replace-with-strong-secret
JWT_TTL_MINUTES=1440
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=25
DB_CONN_MAX_LIFETIME_MINUTES=5
RATE_LIMIT_RPS=5
RATE_LIMIT_BURST=10
```

| Variabel | Fungsi |
|---|---|
| `JWT_SECRET` | Key untuk signing JWT. Harus minimal 32 karakter random. Jika bocor, attacker bisa memalsukan token. |
| `JWT_TTL_MINUTES=1440` | Token JWT expired setelah 1440 menit (24 jam). User harus login ulang setelah itu. |
| `DB_MAX_OPEN_CONNS=25` | Maksimal 25 koneksi aktif ke PostgreSQL. Mencegah backend membanjiri DB dengan koneksi. |
| `DB_MAX_IDLE_CONNS=25` | Maksimal 25 koneksi idle yang disimpan di pool. Mengurangi overhead pembukaan koneksi baru. |
| `DB_CONN_MAX_LIFETIME_MINUTES=5` | Koneksi yang sudah berumur >5 menit akan dibuang dan dibuat baru. Mencegah stale connection. |
| `RATE_LIMIT_RPS=5` | Maksimal 5 request per detik per IP. |
| `RATE_LIMIT_BURST=10` | Boleh ada lonjakan sampai 10 request sekaligus, tapi akan dibatasi ke 5 rps setelahnya. |

### 6.3 Konfigurasi SIEM (E1 — Mochammad Irfan Sandy)

```env
WATCHED_LOG_DIR=/var/log
COLLECTOR_RELOAD_INTERVAL=30
PARSER_POLL_INTERVAL_MS=500
WS_POLL_INTERVAL_MS=500
```

| Variabel | Fungsi |
|---|---|
| `WATCHED_LOG_DIR=/var/log` | Direktori host yang di-mount (read-only) ke dalam container Log Collector. Semua file log di direktori ini bisa dipantau. |
| `COLLECTOR_RELOAD_INTERVAL=30` | Log Collector mengecek tabel `log_sources` di database setiap 30 detik untuk melihat apakah ada path log baru yang ditambahkan via dashboard. Tidak perlu restart container. |
| `PARSER_POLL_INTERVAL_MS=500` | Log Parser memeriksa tabel `raw_logs` setiap 500ms untuk baris yang belum diproses. Menghasilkan latensi parsing < 1 detik. |
| `WS_POLL_INTERVAL_MS=500` | WebSocket server memeriksa event baru setiap 500ms untuk di-push ke semua client yang terkoneksi. |

### 6.4 Konfigurasi LLM (E1 Mochammad Irfan Sandy)

```env
LLM_MODEL_NAME=hf.co/bartowski/Llama-3.2-3B-Instruct-GGUF:Q6_K
LLM_CTX_SIZE=4096
LLM_TIMEOUT_SECONDS=120
LLM_BASE_URL=http://model-runner.docker.internal:12434/engines/v1
```

| Variabel | Fungsi |
|---|---|
| `LLM_MODEL_NAME` | Identifier model yang dijalankan Docker Model Runner. Format HuggingFace + quantization level (`Q6_K` = kualitas tinggi, `Q4_K_M` = hemat memori). |
| `LLM_CTX_SIZE=4096` | Maksimal token context window untuk satu percakapan. Lebih besar = lebih memori. Nilai 4096 adalah keseimbangan antara performa dan kualitas. |
| `LLM_TIMEOUT_SECONDS=120` | Backend menunggu maksimal 120 detik untuk respons LLM. Setelah itu request dianggap timeout. |
| `LLM_BASE_URL` | Endpoint OpenAI-compatible dari Docker Model Runner. Backend tidak perlu tahu detail implementasi LLM — cukup panggil API standar ini. |

---

## 7. Backend — Arsitektur & Modul Go

**Owner:** E1 Mochammad Irfan Sandy  
**Lokasi:** `backend/`  
**Bahasa:** Go 1.23

Backend menggunakan arsitektur **layered / clean architecture** yang memisahkan tanggung jawab menjadi beberapa layer:

```
backend/
├── main.go                  ← Entry point — inisialisasi dan start server
├── config/
│   └── config.go            ← Membaca environment variables ke struct Config
├── handlers/
│   ├── auth.go              ← HTTP handler: register, login
│   ├── chat.go              ← HTTP handler: CRUD chat session
│   ├── message.go           ← HTTP handler: kirim pesan, stream LLM response
│   └── health.go            ← HTTP handler: health check endpoint
├── middleware/
│   ├── auth.go              ← JWT validation middleware
│   └── ratelimit.go         ← Rate limiting middleware
├── models/
│   ├── user.go              ← Struct User + validasi
│   ├── chat.go              ← Struct Chat + Message
│   └── event.go             ← Struct Event SIEM (log event)
├── repositories/
│   ├── user_repo.go         ← Query DB untuk User (INSERT, SELECT)
│   ├── chat_repo.go         ← Query DB untuk Chat & Message
│   └── event_repo.go        ← Query DB untuk SIEM events
└── services/
    ├── auth_service.go      ← Logika bisnis: hash password, buat JWT
    ├── llm_service.go       ← Singleton HTTP client ke LLM endpoint
    └── event_service.go     ← Logika bisnis: simpan dan query events
```

### 7.1 `main.go` — Entry Point

Fungsi `main()` adalah titik masuk program. Tugasnya:
1. Memanggil `config.Load()` untuk membaca semua environment variable
2. Membuka koneksi ke PostgreSQL dengan connection pool
3. Menjalankan migrasi database jika diperlukan
4. Mendaftarkan semua route HTTP (menggunakan router seperti `chi` atau `gorilla/mux`)
5. Mendaftarkan middleware (CORS, rate limit, JWT auth)
6. Memulai HTTP server pada `APP_PORT`
7. Menangani graceful shutdown saat menerima signal `SIGTERM`/`SIGINT`

### 7.2 `config/config.go` — Konfigurasi

Membaca semua environment variable ke dalam satu struct `Config`. Pendekatan ini memudahkan testing (bisa inject config palsu) dan mencegah magic string tersebar di seluruh kode.

```go
type Config struct {
    AppPort              string
    DatabaseURL          string
    JWTSecret            string
    JWTTTLMinutes        int
    DBMaxOpenConns       int
    DBMaxIdleConns       int
    RateLimitRPS         float64
    RateLimitBurst       int
    AllowedOrigin        string
    LLMBaseURL           string
    LLMModel             string
    LLMCtxSize           int
    LLMTimeoutSeconds    int
    // SIEM
    WatchedLogDir        string
    CollectorReloadInterval int
    ParserPollIntervalMs    int
    WSPollIntervalMs        int
}
```

### 7.3 `handlers/` — HTTP Handlers

Setiap handler menerima `http.Request`, memanggil service yang sesuai, dan menulis `http.Response` dalam format JSON. Handler **tidak** mengandung logika bisnis atau query database langsung.

| File | Endpoint | Fungsi |
|---|---|---|
| `auth.go` | `POST /api/auth/register` | Menerima `{username, password}`, validasi input, panggil `AuthService.Register()`, return JWT |
| `auth.go` | `POST /api/auth/login` | Verifikasi credential, return JWT baru |
| `chat.go` | `GET /api/chats` | Ambil semua chat milik user yang login |
| `chat.go` | `POST /api/chats` | Buat chat session baru |
| `message.go` | `GET /api/chats/:id/messages` | Ambil riwayat pesan dalam satu chat |
| `message.go` | `POST /api/chats/:id/messages` | Kirim pesan user → kirim ke LLM → simpan respons AI |
| `health.go` | `GET /health` | Cek koneksi DB dan LLM, return `{"status":"ok","services":{...}}` |

### 7.4 `middleware/auth.go` — JWT Validation

Middleware ini berjalan **sebelum** handler untuk setiap route yang dilindungi:
1. Membaca header `Authorization: Bearer <token>`
2. Memverifikasi signature JWT menggunakan `JWT_SECRET`
3. Mengecek apakah token sudah expired
4. Mengekstrak `user_id` dari JWT claims dan menyimpannya di request context
5. Jika validasi gagal, langsung return `401 Unauthorized` tanpa meneruskan ke handler

### 7.5 `middleware/ratelimit.go` — Rate Limiting

Mengimplementasikan **token bucket algorithm**:
- Setiap IP mendapat "bucket" dengan kapasitas `RATE_LIMIT_BURST` token
- Bucket terisi ulang dengan kecepatan `RATE_LIMIT_RPS` token per detik
- Setiap request mengambil 1 token
- Jika bucket kosong, request dibalas `429 Too Many Requests`

### 7.6 `repositories/` — Database Layer

Repository adalah satu-satunya layer yang boleh berinteraksi dengan database. Menggunakan `database/sql` standar Go dengan prepared statements untuk mencegah SQL injection.

```go
// Contoh: event_repo.go
func (r *EventRepo) InsertEvent(ctx context.Context, e *models.Event) error {
    _, err := r.db.ExecContext(ctx,
        `INSERT INTO events (timestamp, level, source, message, raw)
         VALUES ($1, $2, $3, $4, $5)`,
        e.Timestamp, e.Level, e.Source, e.Message, e.Raw,
    )
    return err
}
```

### 7.7 `services/llm_service.go` — LLM Client

Mengimplementasikan **singleton pattern** — hanya satu instance HTTP client yang dibuat dan digunakan sepanjang lifetime aplikasi:

```
GetLLMService() → cek instance ada? → return existing
                                    → buat baru jika belum ada
```

Ini mencegah pembuatan koneksi baru setiap request, menghemat resource dan mengurangi latensi. Mengirim request ke `LLM_BASE_URL/chat/completions` dengan format OpenAI-compatible.

---

## 8. Frontend — Dashboard Next.js

**Owner:** E3 Lucky Himawan Prasetya  
**Lokasi:** `frontend/`  
**Teknologi:** Next.js 14, TypeScript, Tailwind CSS

### 8.1 Struktur App Router Next.js 14

```
frontend/
├── app/
│   ├── layout.tsx           ← Root layout: font, metadata, providers
│   ├── page.tsx             ← Halaman utama (redirect ke /dashboard atau /login)
│   ├── (auth)/
│   │   ├── login/page.tsx   ← Form login
│   │   └── register/page.tsx← Form registrasi
│   └── (protected)/
│       ├── dashboard/
│       │   └── page.tsx     ← Monitoring real-time via WebSocket
│       ├── search/
│       │   └── page.tsx     ← Pencarian & filter log
│       ├── rules/
│       │   └── page.tsx     ← Rule Builder UI
│       └── alerts/
│           └── page.tsx     ← Alert Management
├── components/
│   ├── EventTable.tsx       ← Tabel event dengan badge severity
│   ├── SeverityBadge.tsx    ← Badge INFO/WARN/CRITICAL dengan warna
│   ├── RuleForm.tsx         ← Form buat/edit rule
│   ├── AlertList.tsx        ← List alert dengan action buttons
│   └── LogSourceForm.tsx    ← Form tambah/hapus path log
├── lib/
│   ├── api.ts               ← Fungsi fetch ke REST API backend
│   ├── websocket.ts         ← WebSocket client + reconnect logic
│   └── auth.ts              ← Fungsi simpan/baca JWT dari localStorage
└── Dockerfile               ← Multi-stage build untuk produksi
```

### 8.2 Halaman Dashboard — Real-time Monitoring

`app/(protected)/dashboard/page.tsx` adalah halaman inti SIEM. Saat komponen mount:
1. Membuka koneksi WebSocket ke backend (`lib/websocket.ts`)
2. Setiap event baru yang di-push server langsung ditambahkan ke state React
3. Tabel `EventTable` re-render otomatis dengan data terbaru
4. Badge `SeverityBadge` menampilkan warna berbeda: biru (INFO), kuning (WARN), merah (CRITICAL)

### 8.3 `lib/websocket.ts` — WebSocket Client

Mengelola koneksi WebSocket dengan fitur **auto-reconnect**:
- Jika koneksi terputus, client menunggu interval tertentu (backoff) lalu mencoba reconnect
- Mencegah dashboard stuck tanpa data saat jaringan tidak stabil

### 8.4 `lib/api.ts` — API Client

Wrapper di atas `fetch()` yang:
- Menambahkan header `Authorization: Bearer <token>` otomatis ke setiap request
- Menangani response error secara terpusat (401 → redirect login, 5xx → tampilkan error)
- Mengekspor fungsi per endpoint: `getEvents()`, `getRules()`, `createRule()`, dll.

### 8.5 Dockerfile Frontend — Multi-stage Build

```dockerfile
# Stage 1: Builder
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Stage 2: Runner
FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
```

Stage `builder` berisi seluruh toolchain Node.js + `node_modules` dev (~500MB). Stage `runner` hanya menyalin output yang dibutuhkan (~100MB). Image final jauh lebih kecil dan tidak mengandung tool pengembangan yang bisa menjadi attack surface.

---

## 9. Database — Skema PostgreSQL

**Owner:** E1 Mochammad Irfan Sandy  
**File:** `database/init/001_init.sql`  
**Fungsi:** Mendefinisikan seluruh struktur tabel database. Dieksekusi otomatis oleh PostgreSQL saat container pertama kali dibuat.

### Tabel Utama

#### `users` — Data Pengguna
```sql
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username   TEXT UNIQUE NOT NULL,
    password   TEXT NOT NULL,  -- bcrypt hash, bukan plain text
    created_at TIMESTAMPTZ DEFAULT now()
);
```
Menyimpan akun pengguna. `password` selalu disimpan sebagai **bcrypt hash** — server tidak pernah menyimpan password asli.

#### `chats` — Sesi Percakapan LLM
```sql
CREATE TABLE chats (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
```
Setiap user bisa memiliki banyak chat session. `ON DELETE CASCADE` — jika user dihapus, semua chat-nya ikut terhapus.

#### `messages` — Pesan dalam Chat
```sql
CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id    UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('user','assistant')),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);
```
`role CHECK` memastikan hanya nilai `'user'` atau `'assistant'` yang valid — constraint di level database.

#### `log_sources` — Sumber Log yang Dipantau (SIEM)
```sql
CREATE TABLE log_sources (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    path       TEXT UNIQUE NOT NULL,
    format     TEXT NOT NULL DEFAULT 'auto',
    active     BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);
```
Menyimpan daftar path file log yang harus dipantau oleh Log Collector. Kolom `active` memungkinkan menonaktifkan sumber log tanpa menghapus konfigurasi.

#### `raw_logs` — Buffer Log Mentah
```sql
CREATE TABLE raw_logs (
    id          BIGSERIAL PRIMARY KEY,
    source_id   UUID REFERENCES log_sources(id),
    raw_line    TEXT NOT NULL,
    collected_at TIMESTAMPTZ DEFAULT now(),
    processed   BOOLEAN DEFAULT false
);
```
Tabel buffer antara Collector dan Parser. Collector menulis baris log mentah di sini, Parser membacanya (di mana `processed = false`) lalu mengubah flag menjadi `true` setelah berhasil diproses.

#### `events` — Log yang Sudah Diparsing
```sql
CREATE TABLE events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp   TIMESTAMPTZ NOT NULL,
    level       TEXT NOT NULL,
    source      TEXT NOT NULL,
    message     TEXT NOT NULL,
    raw         TEXT,
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_events_timestamp ON events(timestamp DESC);
CREATE INDEX idx_events_level ON events(level);
```
Menyimpan log yang sudah diparse menjadi field terstruktur. Dua index memastikan query filter berdasarkan waktu dan severity tetap cepat meski data berjumlah jutaan baris.

#### `rules` — Aturan Deteksi Ancaman (SIEM)
```sql
CREATE TABLE rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    condition   JSONB NOT NULL,
    threshold   INT,
    window_secs INT,
    severity    TEXT NOT NULL CHECK (severity IN ('INFO','WARN','CRITICAL')),
    action      TEXT DEFAULT 'alert',
    active      BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);
```
`condition` disimpan sebagai `JSONB` — format fleksibel yang memungkinkan berbagai tipe kondisi (threshold, regex, kombinasi AND/OR) tanpa mengubah skema tabel.

#### `alerts` — Alert yang Terpicu
```sql
CREATE TABLE alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id     UUID REFERENCES rules(id),
    event_id    UUID REFERENCES events(id),
    severity    TEXT NOT NULL,
    status      TEXT DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved')),
    triggered_at TIMESTAMPTZ DEFAULT now(),
    resolved_at  TIMESTAMPTZ
);
```
Menyimpan setiap alert yang ditriggered rule. `status` merepresentasikan lifecycle alert dari `open` → `acknowledged` → `resolved`.

---

## 10. Nginx — Reverse Proxy & TLS

**Owner:** E3 Lucky Himawan Prasetya  
**File:** `nginx/nginx.conf`  
**Fungsi:** Menjadi gateway tunggal untuk semua traffic publik. Menangani HTTPS, routing, dan keamanan header.

### Routing Logic

```
Internet (HTTPS :443)
    │
    Nginx
    ├── /api/*   → backend:80    (REST API Go)
    ├── /health  → backend:80    (Health check)
    └── /*       → frontend:3000 (Next.js dashboard)

Internet (HTTP :80)
    │
    Nginx → redirect 301 ke HTTPS
```

### Konfigurasi Keamanan

Nginx menambahkan security headers ke setiap response:
- `X-Frame-Options: DENY` — mencegah clickjacking
- `X-Content-Type-Options: nosniff` — mencegah MIME type sniffing
- `Strict-Transport-Security` — memaksa HTTPS untuk semua request berikutnya

---

## 11. scripts/ — Helper Scripts

**Owner:** E3 Lucky Himawan Prasetya & E1 Mochammad Irfan Sandy
**Lokasi:** `scripts/`

| Script | Fungsi |
|---|---|
| `docker-model-run.sh` | Menjalankan Docker Model Runner dengan model yang ditentukan. Mendukung auto-fallback dari `Q6_K` ke `Q4_K_M` jika memori tidak cukup. Juga unload model sebelumnya untuk mencegah konflik memori. |
| `up-with-dmr.sh` | One-command bootstrap: menjalankan `docker-model-run.sh` lalu `docker compose up`. Memudahkan setup di VPS baru. |

---

## 12. Alur Data End-to-End

Berikut adalah alur lengkap dari log masuk hingga notifikasi dikirim:

```
File Log (/var/log/auth.log)
    │
    │  polling setiap COLLECTOR_RELOAD_INTERVAL detik
    ▼
┌─────────────────────────────────────────────────┐
│  Log Collector (Go)  — E1 Irfan                 │
│  • Baca baris baru dari file                    │
│  • INSERT ke tabel raw_logs (processed=false)   │
└──────────────────────────┬──────────────────────┘
                           │
                           │  poll setiap PARSER_POLL_INTERVAL_MS ms
                           ▼
┌─────────────────────────────────────────────────┐
│  Log Parser (Go)  — E1 Irfan                    │
│  • Pilih plugin: syslog / nginx / JSON          │
│  • Ekstrak: timestamp, level, source, message   │
│  • INSERT ke tabel events                       │
│  • UPDATE raw_logs SET processed=true           │
└──────────────────────────┬──────────────────────┘
                           │
                     ┌─────┴──────┐
                     │            │
                     ▼            ▼
         ┌───────────────┐  ┌─────────────────────────┐
         │  PostgreSQL   │  │  Rule Engine (Go) — E2   │
         │  events table │  │  Tuti                    │
         └───────────────┘  │  • Evaluasi threshold    │
                            │  • Pattern matching      │
                            │  • AND/OR kondisi        │
                            │  • Assign severity       │
                            └────────────┬────────────┘
                                         │
                                   Rule terpenuhi?
                                         │ YA
                                         ▼
                            ┌─────────────────────────┐
                            │  Alert Manager — E2 Tuti │
                            │  • Cek deduplication     │
                            │  • INSERT ke tabel alerts│
                            │  • Kirim email (SMTP)    │
                            │  • Kirim webhook (HTTP)  │
                            └────────────┬────────────┘
                                         │
                                         ▼
         ┌───────────────────────────────────────────┐
         │  WebSocket Server (Go)  — E1 Irfan        │
         │  poll setiap WS_POLL_INTERVAL_MS ms       │
         │  • Query events & alerts baru             │
         │  • Broadcast ke semua client terkoneksi   │
         └──────────────────────┬────────────────────┘
                                │
                                │ WebSocket push
                                ▼
         ┌───────────────────────────────────────────┐
         │  Dashboard UI (Next.js)  — E3 Lucky       │
         │  • EventTable: update real-time           │
         │  • SeverityBadge: warna sesuai level      │
         │  • AlertList: tampilkan alert baru        │
         │  • Sound/visual notification              │
         └───────────────────────────────────────────┘
```

---

*Laporan ini dibuat berdasarkan source code, konfigurasi, dan WBS project SIEM NCC Laboratory 2026.*  
*Kelompok 3 · E1 Mochammad Irfan Sandy · E2 Tuti Purwaningsih · E3 Lucky Himawan Prasetya*
