**Quick answer (TL;DR)**
Ship a tiny, single‑binary, self‑hosted object‑storage daemon that speaks the core Amazon S3 API against a plain POSIX filesystem. Target “FTP‑style” workflows (simple auth, folder‑ish listings, streaming uploads/downloads). Build it in Go, MIT License, Docker/Helm‑ready. First milestone: read/write, bucket‑scoped keys, V4‑signing, TLS‑terminating reverse proxy. Later: multipart, versioning, per‑bucket quota, optional OpenID Connect, FUSE mount helper, and an FTP‑proxy shim for legacy clients.

---

### 1 — Why not just use MinIO/Ceph/etc.?

* **MinIO** is still great, but they recently yanked the free admin UI and are gating more enterprise features — the community backlash shows many devs want a leaner alternative.
* **Ceph RGW** and **Scality Ring** are powerful but overkill when you just need “FTP‑on‑HTTP”.
* Lightweight newcomers like **Garage**, **SeaweedFS**, **VersityGW** exist, yet each brings its own distributed‑storage assumptions—you may only need a \*nix path.
  Building a purpose‑built “S3 façade over local disk” keeps the surface small, avoids AGPL worries, and lets you hard‑focus on UX. ([Cloudian][1], [futuriom.com][2], [GitHub][3])

### 2 — Target use‑cases & scope

| Must‑have (v0.1‑v0.3)                                                                                                                                                                                       | Nice‑to‑have (v0.4‑v1.0)                                                                                                                                                                                          | Wild‑cards (v1.x+)                                                                                                                                                                                                      |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| • Single‑node & docker compose<br>• Buckets → directories<br>• PUT/GET/DELETE/ListObjectsV2<br>• AWS V4 signature auth (access/secret keys)<br>• HTTPS behind Caddy/Traefik<br>• Streaming, zero‑copy sends | • Multipart & resumable<br>• Object tags & per‑bucket policies<br>• Built‑in TLS + Let’s Encrypt<br>• OpenID Connect / JWT auth<br>• Server‑side encryption (age, libsodium)<br>• Quotas & soft‑delete versioning | • FUSE mount (read‑only) for legacy apps<br>• Web UI in Svelte (optional add‑on)<br>• FTP‑proxy (accepts PORT/PASV, stores in S3)<br>• rclone remote & restic backend files<br>• Cluster mode with Raft‑backed metadata |

### 3 — Architecture sketch

```
┌────────────┐    S3 REST/JSON   ┌─────────────┐
│  Client    │ ────────────────▶ │  YourDaemon │──┐
└────────────┘  (aws s3 cp,      └─────────────┘  │
                s3cmd, rclone)                    ▼
                                            ┌────────┐
                                            │ POSIX  │
                                            │  disk  │
                                            └────────┘
```

* **Language**: Go (std lib `net/http`, `io.Copy`, concurrency, static binary).
* **Router**: chi or gin; middlewares: auth, metrics, gzip, CORS.
* **Storage layer**: abstract interface; default `LocalFS`; later plug‑ins: NFS, SMB share, Wasabi, Backblaze, etc.
* **Metadata**: extended‑attributes + SQLite for multipart bookkeeping; swap for embedded Badger when clustering.
* **Config**: single `config.yaml`, env overrides; hot‑reload with SIGHUP.
* **Observability**: Prometheus, OpenTelemetry traces, optional pprof endpoint.

### 4 — Development roadmap

| Version | Goals                                         | Rough ETA |
| ------- | --------------------------------------------- | --------- |
| 0.1     | Read‑only GET + ListObjectsV2                 | 2 weeks   |
| 0.2     | PUT/DELETE, basic auth, Docker image          | +3 wks    |
| 0.3     | Multipart ≥5 GB, HTTP range requests, CI/CD   | +4 wks    |
| 0.4     | Web‑UI plug‑in, OIDC, per‑bucket ACL          | +6 wks    |
| 0.5     | FUSE helper + FTP‑proxy sidecar               | +8 wks    |
| 1.0     | Hardened API, semver freeze, docs, Helm chart | Q1 2026   |

### 5 — Project hygiene

* **Repo layout**: `/cmd/daemon`, `/internal/http`, `/pkg/storage`, `/docs`.
* **CI**: GitHub Actions → build, unit + integration tests (moto server for clients).
* **Releases**: multi‑arch binaries (linux/amd64, arm64).
* **License**: Apache‑2.0 to maximise downstream adoption.
* **Governance**: `CODEOWNERS`, GH Discussions, semantic‑commit bot, conventional changelog.
* **Security**: `slsa-generator` provenance, automated `gosec`, daily Dependabot.

### 6 — Names that aren’t taken

* **“Slipstream”** – light, fast data flow
* **“Porter”** – carries files across ports 80/443
* **“Shelve”** – stick it on the shelf
* **“Tropos”** – first layer above ground, i.e., above FTP

### 7 — Things you maybe didn’t consider

* **Mount‑able**: A read‑only FUSE view lets old software use POSIX paths without touching FTP.
* **Partial compatibility layer**: Ship a tiny FTP front‑end that *internally* speaks your S3 API; eases migration for scripts you can’t rewrite.
* **Object‑level delta sync**: Optional zstd‑diff block store so repeated uploads of large logfiles cost O(diff) not O(size).
* **Policy‑as‑code**: Bundle OPA‑based policy engine so ops teams can express per‑prefix restrictions declaratively.
* **Staggered deletion**: default to a recycle‑bin prefix with TTL to prevent “rm ‑rf /” incidents—mirrors the safety nets people expect from modern cloud storage.

---

#### Sources

• Overview of on‑prem S3‑compatible storage and why you may want a lightweight alternative ([Cloudian][1], [Medium][4])
• MinIO community‑edition feature removals and user backlash ([futuriom.com][2])
• List of FOSS S3 servers (Garage, SeaweedFS, Ceph, MinIO, etc.) ([GitHub][3])

[1]: https://cloudian.com/guides/s3-storage/best-s3-storage-options-top-5-on-prem-s3-compatible-storage-solutions-2025/?utm_source=chatgpt.com "Top 5 On-Prem S3-Compatible Storage Solutions [2025] - Cloudian"
[2]: https://www.futuriom.com/articles/news/minio-faces-fallout-for-stripping-features-from-web-gui/2025/06?utm_source=chatgpt.com "MinIO Faces Fallout for Stripping Functions from Open Source Version"
[3]: https://github.com/fffaraz/awesome-selfhosted-aws?utm_source=chatgpt.com "Awesome Self-hosted AWS - GitHub"
[4]: https://medium.com/cubbit/top-minio-and-ceph-s3-alternatives-in-2025-european-gems-inside-b99aa4c6abb6?utm_source=chatgpt.com "Top MinIO and Ceph S3 alternatives in 2025 (European gems inside)"
