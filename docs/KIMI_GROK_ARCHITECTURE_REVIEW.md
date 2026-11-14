# Kimi & Grok Consultation: General-Hardware Guix Installer, Speed Optimizations & Teaching Curriculum

- **Generated**: 2026-11-14T05:59:27Z
- **Kimi CLI**: `kimi` (249 lines)
- **Grok CLI**: `grok` (`grok-4.7-build`, 635 lines)
- **Symlink integrity check**: `mcp-sync --check-links` -> `links: current (73 managed links)`

---

# Part I: Kimi CLI Raw Output

• # 1. Re-Architecting `guix-platform-install` for General Hardware

  ## 1.1 From platform silos to a probe → plan → apply pipeline

  The core mistake of per-platform silos (`cloudzy`, `framework`, `framework-dual`) is conflating **four orthogonal axes** into one hardcoded script: firmware (UEFI/BIOS), storage topology (bus, count, encryption), CPU/microcode, and platform extras (bootloader quirks, kernel modules, firmware packages). Re-architect as a three-phase pipeline:

  ```
  probe  →  facts.scm (alists of probed hardware)
  plan   →  plan.scm  (partition layout, bootloader, microcode, channels) — pure function of facts
  apply  →  execute plan idempotently (wipefs/parted/mkfs/cryptsetup/guix system init)
  ```

  `plan` being a *pure* Guile function of `facts` is what gives you a free dry-run: `guix-platform-install plan` prints the s-expression plan and the exact shell commands it would run; `apply` runs them. This replaces per-platform scripts with per-axis *policy modules* that compose.

  ## 1.2 CPU & architecture matrix

  Probe via `/proc/cpuinfo`, `uname -m`, DMI (`/sys/class/dmi/id/`), and map to:

  | Probe | Consequence |
  |---|---|
  | x86_64, vendor GenuineIntel | `(service intel-microcode-service-type)` — note: in current Guix this lives in nonguix; on pure Guix with linux-libre, microcode loading is **blocked by the deblobbed kernel** — the kernel literally refuses microcode updates. State this honestly in the plan output. |
  | x86_64, AuthenticAMD | `amd-microcode` (same nonguix caveat). AMD also needs `amdgpu` early-load decisions; linux-libre ships amdgpu but without firmware blobs many newer GPUs get no display — probe PCI class `0x0300` and warn. |
  | aarch64, device-tree systems (RPi, Rockchip, most SBCs) | No EFI variables in the classical sense; U-Boot is the firmware. Use `u-boot-<board>-bootloader` targets, and the install target is the raw SD/eMMC offset, not a partition. Board detection via `/proc/device-tree/model` and `compatible`. |
  | aarch64, UEFI servers (Ampere Altra, Qualcomm, ACPI+UEFI) | These boot exactly like x86_64 UEFI: `grub-efi-bootloader`, ESP on `vfat`, standard path. Treat "UEFI-ness" as orthogonal to architecture — key insight. |
  | aarch64, Apple Silicon | Out of scope for vanilla Guix: requires Asahi kernel/m1n1 chainloading. Detect via `compatible` containing `apple,` and fail with an explicit unsupported message instead of a broken install. |

  Model this as a `(cpu-support)` record: `(arch . vendor . microcode-service . kernel-args)`, so `framework` becomes just "x86_64 + intel + UEFI + NVMe + wifi-firmware-needed" — data, not code.

  ## 1.3 Storage bus & naming abstraction

  The single most bug-prone area. Rules to bake in:

  **Partition separator.** Compute it from the device name, never hardcode: if the base name ends in a digit (`nvme0n1`, `mmcblk0`, `loop0`, `md0`), partitions are `${dev}p${n}`; otherwise `${dev}${n}` (`sda1`, `vda1`, `xvdz3`). In Scheme:

  ```scheme
  (define (partition-device disk n)
    (string-append disk (if (char-numeric? (string-ref disk (1- (string-length disk)))) "p" "")
                   (number->string n)))
  ```

  **Never persist kernel names.** `/dev/nvme0n1` enumeration order depends on PCIe probe timing and can flip across reboots (two identical NVMe drives swap). Anything written into the generated `operating-system` or crypttab must be an invariant:

  - **LUKS**: reference by `UUID=` of the *LUKS container* (from `blkid` on the raw partition), and inside, the mapped device `/dev/mapper/<name>`.
  - **Filesystems**: `UUID=` from `blkid` post-mkfs.
  - **ESP in `(bootloader-configuration targets)`**: `"/boot/efi"` mount path is fine, but the `file-systems` entry should be UUID.
  - Prefer `/dev/disk/by-id/` (contains model+serial, e.g. `nvme-Samsung_SSD_980_...`) for the *installer plan* because it's human-auditable, then convert to UUIDs for the *installed system*. `by-path` is stable against drive swapping but not against moving the drive to another port; `by-uuid` survives port moves but not reformatting — pick per context and document why.

  **Layout generation.** Parameterize `(layout uefi? luks? fs-type swap-policy)`:

  - UEFI+LUKS2+Btrfs: `p1` ESP 512MiB `ef00` vfat; `p2` LUKS2 (`--type luks2 --pbkdf argon2id` — note cryptsetup defaults are fine on modern versions, but GRUB's LUKS2 support historically choked on argon2id for `/boot` on LUKS; the clean solution is **unencrypted ESP + `/boot` on the ESP**, or encrypted `/boot` with `--pbkdf pbkdf2` only if `/boot` is inside LUKS and GRUB must unlock it — in Guix, the standard pattern is `/boot/efi` unencrypted and everything else under LUKS, GRUB unlocks via `luks-device-mapping` with the kernel re-unlocking; double-password-entry is avoided by embedding a keyfile in the initrd, which Guix supports via `(luks-device-mapping-with-options ...)` — but know the initrd lives on the ESP, so a keyfile there weakens the threat model; document the tradeoff rather than hiding it).
  - Btrfs subvolumes: `@` (root), `@home`, `@gnu` (separate so store scrubbing/snapshots are independent), `@swap` with `chattr +C` if using a swapfile (NOCOW is mandatory — a COW swapfile fails to activate).
  - Legacy BIOS on GPT: 1MiB BIOS Boot Partition `ef02`, **unformatted** — the classic bug is mkfs'ing it, which GRUB then can't embed into.
  - MBR: no boot partition needed, but GRUB post-MBR gap embedding; note ~2TiB ceiling.

  ## 1.4 Multi-disk safety (evolving `install-this-pc.sh`)

  MODEL matching is necessary but not sufficient (two identical models!). Upgrade the fingerprint to a tuple and require *all* present fields to match:

  ```scheme
  (disk-fingerprint
    (model  "Samsung SSD 980 PRO 2TB")
    (serial "S6B0NL0W123456X")        ; lsblk -o SERIAL — may be empty on some USB bridges
    (wwn    "eui.002538b321b59f2a")  ; lsblk -o WWN — most stable for NVMe/SATA
    (path   "pci-0000:0e:00.0-nvme-1")) ; optional, warns if drive moved ports
  ```

  Then apply **defense in depth** before any destructive op:

  1. **Fingerprint match** (all fields; empty serial = require explicit `--i-know-serial-is-missing`).
  2. **Refcheck / holder check**: refuse if `lsblk -o MOUNTPOINTS` shows anything mounted on the target *or its partitions*; refuse if `/sys/block/<dev>/holders/` is non-empty (means mdadm/LVM/dm holds it); refuse if the target appears in `/proc/swaps`.
  3. **Signature audit**: `blkid -p` and `wipefs -n` (dry print) on the target — if the "cold/work" disk check was supposed to be against a *blank* disk but finds ZFS (`zfs_member`), LVM (`LVM2_member`), or mdadm superblocks, treat as inhabited and abort. ZFS is special: an exported pool leaves no mounts and no holders — only the label betrays it. Also check `zpool status` importable pools and `pvs`/`vgs` for the device.
  4. **Inverse check**: verify the *protected* disks (your "cold"/"work" drives) are present and resolvable by their own fingerprints, and add them to an explicit **blast-radius lock**: a file listing devices the run may never touch; every destructive helper (`wipefs`, `sgdisk`, `mkfs.*`, `cryptsetup luksFormat`) goes through one wrapper in Go/Guile that resolves its argument to a canonical `/dev/disk/by-id` path and checks membership in the lock set. One chokepoint, not scattered checks.
  5. **Dry-run plan**: print the full plan — target device, partitions to create with sizes/types, signatures that will be destroyed (output of `wipefs -n`), and the protected list — and require typing the disk's serial (not "yes") to proceed. Typing the serial defeats reflexive confirmation better than any prompt wording.
  6. Refuse to run if the target contains the *currently booted root* (compare `findmnt -n -o SOURCE /` ancestry) unless `--install-over-running-system` is passed.

  ## 1.5 Bootloader & firmware matrix

  ```
  firmware := uefi | bios
  partition-table := gpt (default) | mbr (only if bios + <=2TiB + explicit)

  uefi + gpt → grub-efi-bootloader, targets ("/boot/efi"), ESP vfat 512MiB ef00
  bios + gpt → grub-bootloader, target "/dev/sdX", ef02 1MiB unformatted
  bios + mbr → grub-bootloader, msdos, bootable flag
  ```

  UEFI caveats that bite in practice:

  - **Detect UEFI by `/sys/firmware/efi` existing** — not by DMI vendor. Installing in BIOS-booted mode on a UEFI machine produces an unbootable UEFI system.
  - **EFI variables**: on some broken UEFI firmwares (many VPS, older AMI), `efibootmgr` writes fail or are ignored. `grub-install --no-nvram` + installing to the **removable media path** (`/boot/efi/EFI/BOOT/BOOTX64.EFI`) is the universal fallback. In Guix you get this by `(bootloader-configuration (bootloader grub-efi-removable-bootloader))` — offer it as a plan option "buggy UEFI / VPS mode". `cloudzy` and `oracle` are exactly this case; that entire silo collapses into one flag.
  - Some firmwares only boot the removable path; some only boot registered entries. Default to registering (efibootmgr via normal grub-efi) and fall back to removable on failure.
  - 32-bit UEFI with 64-bit CPU (old Bay Trail tablets): needs `grub-efi-bootloader` with i386-efi on the removable path — detect via `/sys/firmware/efi/fw_platform_size` containing `32`.

  **linux-libre vs nonguix separation.** Keep it structurally explicit in the generated config:

  ```scheme
  ;; base module — always pure
  (operating-system (kernel linux-libre) (firmware '()) ...)

  ;; nonguix module — only emitted when probe determines need AND user opted in
  (kernel linux)
  (firmware (list linux-firmware sof-firmware))
  (cons* (service intel-microcode-service-type)
         ;; nvidia-service-type only when PCI 10de: detected AND opted in
         ...)
  ```

  Probe-driven recommendation: if `lspci -nn` shows NVIDIA (`10de:`), recent Intel Wi-Fi (`8086:` with `iwlwifi` need), or AMD GPU newer than what linux-libre firmware covers, print "this hardware will not fully function on linux-libre; pass `--nonguix` to enable non-free firmware." With nonguix: pin the channel commit in `channels.scm` (both `guix` and `nonguix` channels pinned to a tested pair — un-pinned nonguix is the #1 source of "worked last month" breakage), and authorize its substitute key (`3E89...` nonguix key) via `--substitute-urls`/ACL or the plan writes `/etc/guix/acl` entries — do this *before* `guix system init` or the kernel build compiles locally for hours.

  ## 1.6 Hardware probing & config synthesis

  Don't generate Scheme by string interpolation from Bash — the current `guix shell go` → run-remote-steps approach should evolve to **Guile-native S-expression generation**, which is just printing quasiquoted data:

  ```scheme
  (define (facts->operating-system facts plan)
    `(operating-system
       (host-name ,(plan-host-name plan))
       (bootloader ,(bootloader-expr (plan-firmware plan)))
       (mapped-devices ,(luks-exprs plan))
       (file-systems ,(append-map fs-expr (plan-filesystems plan)))
       (services ,(append (base-services facts)
                          (microcode-services facts)
                          (platform-services facts)))))
  ```

  Write it with `(ice-9 pretty-print)` or `pretty-print-with-comments`, round-trip-parse it in tests (`(read ...)` must succeed), then `guix system init config.scm /mnt`. Better: run `guix system build config.scm` *before* init as a whole-config compile check — it catches unbound variables and type errors without touching disks. And `guix system vm config.scm` gives you a smoke-test VM of the exact generated config as a CI gate per hardware-profile fixture.

  Probe sources, in order of reliability: `lsblk -J -O` (JSON — parse in Go or with `(json)` in Guile), `lspci -nn -D`, `lsusb`, `/sys/class/dmi/id/{product_name,board_vendor}`, `/sys/firmware/efi`, `/proc/cpuinfo`. Cache the raw probe to `facts.txt` next to the generated config so a failed install is debuggable post-mortem.

  ---

  # 2. Top 10 Guix Build & Substitute Speed Optimizations (ranked by leverage)

  **1. Substitutes actually working and authenticated (the 100× item).** Nothing beats not building. Verify `guix weather ci.guix.gnu.org bordeaux.guix.gnu.org` for your system; authorize both keys; add nonguix substitutes if using that channel. Caveat: substitutes lag `master` by hours-to-days on big rebuilds — pulling an older commit (`guix pull --commit=`) often converts a 6-hour build into a 10-minute download. "Choose a commit with good coverage" is the highest-leverage operational habit in Guix.

  **2. Local builds in RAM: `TMPDIR` on tmpfs.** The daemon builds in `$TMPDIR` (`/tmp` by default, on-disk on many distros). Mount `/tmp` as tmpfs or point the daemon's environment: `TMPDIR=/dev/shm` in the `guix-daemon` systemd unit / Shepherd service. Caveats: size it — `ungoogled-chromium`, `llvm`, `rust` can need 30–60 GiB of build tree; a too-small tmpfs fails with ENOSPC *and* the tmpfs competes with page cache for RAM, so on a 16 GiB machine big builds OOM. Rule of thumb: tmpfs sized ≤ 50% RAM, and for the giant packages, per-build `TMPDIR` override on disk or just accept the substitute-only policy (`--fallback` off, `guix weather` first).

  **3. Parallelism: `--cores` and `--max-jobs`.** Defaults are conservative.

  ```scheme
  (guix-configuration
    (extra-options '("--cores=0"          ; 0 = all cores per job
                     "--max-jobs=4"
                     "--cache-failures"   ; don't retry known-failing builds
                     "--keep-derivations" ; cheap; lets you gcroots/inspect
                     "--keep-outputs")))  ; great for guix environment re-use
  ```

  Tuning: `--max-jobs=1 --cores=N` is optimal for one huge parallel build (chromium); `--max-jobs=4 --cores=N/4` is better for many small packages (typical system profile). `--cache-failures` is pure win — without it the daemon re-runs failing builds of several GB.

  **4. LAN caching with `guix publish`.** One beefy always-on machine publishes its store:

  ```sh
  guix publish --port=8080 --compression=zstd:3 --cache=/var/cache/guix/publish \
               --workers=4 --ttl=30d --advertise   # Avahi/mDNS discovery
  ```

  Clients get `--discover` (`guix-daemon --discover` or substitute-urls auto-discovery) or explicit `--substitute-urls=http://builder.lan`. `--cache` pre-compresses nars so publish serves instantly instead of compressing per-request; zstd:3–6 is the sweet spot (level 19 is barely smaller, much slower). On the publisher, authorize your own key; on clients, `guix archive --authorize < publisher-key`. Caveat: `--advertise` needs Avahi running on both ends; on locked-down LANs use explicit URLs.

  **5. Compression choice.** `--compression=zstd:3` daemon-side (and for `guix publish`) beats gzip:9 on both speed *and* usually size. Substitute servers already serve zstd narinfos; ensure your `guix` is recent enough to prefer them (it is, since 1.4-era) — check with `guix download --verbosity=2` or watch narinfo `Compression: zstd`. For `guix pack`/`guix archive` over the wire, zstd:3 default-ish; zstd:19 only for archival.

  **6. Distributed builds with `guix offload`.** `/etc/guix/machines.scm`:

  ```scheme
  (list (build-machine
          (name "bigbox.lan") (systems '("x86_64-linux" "i686-linux"))
          (speed 4.0) (parallel-builds 4) (user "guix-builder")
          (private-key "/etc/guix/signing-key.sec")))  ; ssh keypair
  ```

  Pro tips manuals gloss over: set up `~/.ssh/config` **ControlMaster/ControlPersist** for the offload user — each offloaded build otherwise pays a full SSH handshake; give each machine a truthful `speed` so the scheduler routes big jobs to big boxes; `systems` is how you get aarch64 builds off an Ampere box or x86 builds onto a Pi's armv7. Test with `guix offload test`. Debug with `GUIX_DAEMON_SOCKET=... guix offload status`-style logs in `/var/log/guix/daemon.log` — offload failures silently fall back to local builds, which masquerades as "slow" rather than "broken".

  **7. Channel & evaluation caching.** `guix pull` compiles thousands of Guile modules — the single most painful Guix operation. Mitigations: (a) pull *less often* and pin (`channels.scm` with commits + `guix time-machine -C channels.scm -- ...` for everything reproducible); (b) pull to commits with good substitute coverage so the compiled Guix itself is substituted, not built — the channel-build step substitutes when the commit is in CI's history; (c) don't fight Guile's compiler caches: run as the same user consistently, keep `~/.cache/guile` (compiled `.go` files) out of cleanup cron jobs — blowing it away recompiles every module on next invocation; (d) on the daemon side, grafts: `--no-grafts` makes downloads/builds faster *now* at the cost of ungrafted (potentially unpatched) packages — fine for throwaway dev shells, wrong for servers; conversely a system with thousands of grafts pays quadratic-ish derivation rewriting on every op — periodic `--no-grafts` rebuild then re-enable clears accumulated graft chains.

  **8. Microarchitecture tuning — use sparingly.** `--tune=native` (or `tune` property / `GUIX_TUNE`) compiles for your µarch (psABI levels like `x86-64-v3` are the safer middle ground). The trap: every tuned derivation has a different hash → **zero substitute hits** for the tuned package *and everything that depends on it*. So tune leaf-heavy, self-contained things (your kernel via nonguix, math libs in a HPC profile) and never base libraries (glibc, openssl) unless you enjoy rebuilding the world. Gain is real (~5–15% on vectorized code) but the default answer for a workstation is: don't.

  **9. Daemon hygiene: GC strategy and failure caches.** Speed over weeks is dominated by store management: `guix gc -F 50G` (free-space-driven) instead of aggressive full GCs that delete substitutes you'll re-download tomorrow; `--keep-outputs` + `--keep-derivations` makes `guix shell`/`guix environment` re-entry nearly instant. Add gcroots for active dev profiles (`guix shell --root=...`) so GC never evicts your working sets — re-downloading a 3 GiB toolchain because of an overzealous cron GC is a classic self-inflicted wound.

  **10. P2P substitutes: GIPS / GNUnet-file-sharing / IPFS — promising, still niche.** The architecture: nars are content-addressed already, so publishing to IPFS (`guix publish` has had IPFS support via a running `ipfs` daemon; the GIPS research line explored this systematically) or GNUnet FS works, with discovery via DHT or mDNS and integrity enforced by the existing narinfo signature chain — transport can't forge content because the signature is over the nar hash. Honest assessment for 2026: warm-cache LAN/WAN-mesh hits are excellent and it's the most *elegant* scaling answer; cold-cache DHT lookups add seconds of latency per narinfo and the publisher population is tiny, so treat it as a supplement to `guix publish` on your LAN and the official CI, not a replacement. Signing and authorization remain the existing `guix archive --authorize` ACL — no new trust model is needed, which is the design's real win. Web-of-Trust/fraud-proof extensions are research-stage; don't build curriculum promises on them.

  ---

  # 3. Teaching: Mental Models & Unmentioned Pro Tips

  ## 3.1 Guile Scheme for Guix users

  **The one mental model that unlocks Guix: two strata of execution.** Host-side Scheme runs *now*, on your machine, and its job is to *construct descriptions* (derivations, gexps) of what will run later, possibly in a build container, possibly on another architecture. G-expressions are staged quotation:

  ```scheme
  #~(mkdir #$output)          ; build-side code, #$output splices a store path
  #~(symlink #$coreutils ...) ; #$ = unquote across the stage boundary
  #$(list a b)                ; host-side list spliced in
  #$@pkgs                     ; splicing unquote, like ,@ but staged
  ```

  Teach it as: `` ` ``/`,` quote within one stage; `#~`/`#$` quote *across* stages. Once learners see `gexp->derivation` as "compile this staged code into a .drv", Guix stops being magic. Pro tip: `,lower` and `,build` in `guix repl` (and `guix build --derivations`) let you inspect exactly what host code produced.

  **Homoiconicity in practice**: code is data, so config generation (Section 1.6) needs no templating language — quasiquote *is* the template engine. The anti-pattern to warn about: string-append'ing Scheme source. Always build S-expressions, pretty-print them.

  **Macros**: teach `syntax-rules` first (pattern → template, hygiene for free), then `syntax-case` with `syntax->datum`/`datum->syntax` for when you need to break hygiene or compute at expansion time. Guix itself is the best macro museum: `operating-system`, `define-record-type*`, `modify-services` are all `syntax-case` territory.

  **`define-record-type*`** (Guix's, not SRFI-9): keyword constructors with defaults, `inherit` for functional update — `(operating-system (inherit base) (host-name "x"))` — and it plays with sanitizers/delayed fields. This one macro explains 40% of Guix source.

  **`(ice-9 match)`**: replace all `car/cadr/assoc-ref` chains on alists with pattern matching; show `match` against `lspci`-parsed alists in your installer code as the motivating example.

  **Debugging**: Geiser (`M-x run-guile`, `,m (gnu packages)` to switch modules, `C-c C-k` compile buffer, `,d` doc, `M-.` jump) or Arei/ares-rs for nREPL-style. `guix repl` gives you a Guile with Guix preloaded — it's the fastest way to explore the API (`,apropos operating-system`). When a config fails with a cryptic backtrace, `(use-modules (guix diagnostics))`-style formatted errors hide the real frame — re-run with `guix system build config.scm --verbosity=2` and read the *innermost* frame. PK (`(pk 'label value)` from `(guix build utils)`/Guile) is printf-debugging that returns its argument — teach it early, it works build-side too (visible in `guix build --keep-failed` logs).

  ## 3.2 Emacs

  **Mental model**: Emacs is a Lisp machine whose primary data structure is the *buffer* (a string + gap + properties + local state), not the file. Windows view buffers; frames hold windows; processes and even network connections are buffer-producing objects. Once "everything is a buffer, everything is a function, everything is inspectable" lands (`C-h f/v/k/b`, `M-x ielm`, `M-:`), self-documentation replaces tutorial-hunting — that *is* the Emacs superpower to teach first.

  **Init**: `early-init.el` runs before package/UI init — put `package-enable-at-startup nil` (when using Guix or elpaca/straight), frame/menu/tool-bar suppression, and GC tuning (`gc-cons-threshold` up during init, reset after via `emacs-startup-hook` or `gcmh`) there. `init.el` is for everything else. Pro tip: `emacs --init-directory=~/test-emacs` gives hermetic config experiments.

  **The minibuffer stack** (Vertico/Consult/Marginalia/Orderless/Corfu/Eglot): teach it as *layered, replaceable completion*: Vertico = UI list; Orderless = matching style (space-separated unordered components — the single biggest day-one QoL win); Marginalia = annotations; Consult = commands built on `completing-read` (`consult-line`, `consult-ripgrep`, `consult-buffer`); Embark (don't skip it!) = actions on the thing at point / in the minibuffer — `C-.` then `?` is discoverability incarnate; Corfu = in-buffer popup completion fed by capfs; Eglot = LSP client that just wires into capf/eldoc/xref/flymake — minimal config, works with Guile (via `guile-lsp-server`, rough but improving), C, Go, Rust.

  **Declarative Emacs via Guix Home**: `home-emacs-service-type` installs emacs packages from Guix (native-compiled, pinned, no ELPA network drift) and tangled/inline Lisp into the init. Pro tips: (a) prefer Guix-packaged elisp over `package.el` so `guix home reconfigure` rollbacks roll back Emacs too; (b) keep a thin handwritten `init.el` *outside* the Guix-generated one for experimentation, loaded from the generated one — otherwise every paren tweak costs a reconfigure; (c) `M-x emacs-init-time` and `esup` for startup profiling.

  **Surviving parens**: teach structural editing from day one — `paredit` (`C-(`, `C-)`, `M-r` raise, `M-s` splice) or `smartparens` (more forgiving, non-Lisp-aware modes) or the gentler `puni`. The rule to instill: *never type a bare paren*; use slurp/barf/raise. Also `check-parens` before commits, and `rainbow-delimiters` for orientation. Beginners fear parens; structural editing converts that fear into the reason they can never go back.

  ## 3.3 Org-mode literate system configuration

  **Mental model**: the `.org` file is the *source of truth*; tangled files are *build artifacts*. Never edit artifacts — commit only the org file (or commit both but treat tangled output as generated and regenerate in a pre-commit/CI check to detect drift).

  Battle-tested header stack:

  ```org
  #+PROPERTY: header-args:scheme :tangle-mode (identity #o644) :mkdirp yes
  #+PROPERTY: header-args:sh :tangle-mode (identity #o755) :mkdirp yes :shebang "#!/bin/sh"
  ```

  - `:mkdirp yes` — tangling into `modules/hardware.scm` fails confusingly without it.
  - `:tangle-mode (identity #o755)` — the `identity` trick is the famous one: `:tangle-mode` expects a *form that evaluates* to a mode; a bare `#o755` errors.
  - `:noweb yes` — compose blocks: define `<<base-services>>` once, reference in multiple tangled outputs (system config, VM variant, installer variant). This is how one org file drives `cloudzy`, `framework`, and a `guix system vm` test target without copy-paste — noweb refs are expanded at tangle time, so artifacts stay self-contained.
  - Property inheritance: set `:header-args:scheme :session none :eval never` at file level; per-headline `:tangle path/to/file.scm` via `org-set-property` or `#+PROPERTY:` in subtrees. Pro tip: a `:tangle no` default plus per-block opt-in prevents stray scratch blocks from polluting your system config.
  - Workflow: `org-babel-tangle` (`C-c C-v t`) → `guix system reconfigure` in one `C-c C-c` shell block or a Makefile target. Use `(org-babel-tangle-file ...)` in an Emacs batch script in CI, then `git diff --exit-code` on tangled output = drift detection.
  - Tangling *Guix Home* and *system* configs from one file: use noweb for shared service lists, and headline tags (`:cloudzy:`) with `org-babel-tangle` + `#+PROPERTY: header-args :tangle` per subtree. Caveat to teach: noweb + indented blocks can mangle whitespace-sensitive output; test with `org-babel-expand-src-block`.

  ## 3.4 Linux & Guix internals

  **The store**: `/gnu/store/<hash>-<name>` where hash covers the *build recipe* — inputs, builder, env — not the output bits (input-addressed). Change a dependency's source → new hash → new path → nothing overwrites anything → rollbacks and coexistence fall out for free. Contrast with **fixed-output derivations** (source tarballs): hash commits to the *output*, so `guix download` can fetch from any mirror and verify. This dichotomy is the single best explanatory frame for "why is Guix trustworthy and reproducible". Pro tip: `guix gc --references`/`--referrers` on a store path visualizes the graph; `guix graph --type=references` makes it literal.

  **Boot sequence** (the part nobody teaches): GRUB (generated menu entries point into the store, one per generation — rollback is a GRUB menu) → Linux-libre kernel → **initrd whose PID 1 is a Guile program** (`/gnu/store/...-initrd/init` is a compiled gexp): it mounts devtmpfs/proc/sysfs, runs `cryptsetup`/`luks-device-mapping`, loads modules from its own store closure, finds the root fs by UUID, then `switch-root` → the system's activation: the `...-boot` script runs activation (symlinks `/etc`, creates users/homes) → **GNU Shepherd as PID 1**. Teach Shepherd as "systemd, but services are Guile values and `herd status|start|restart nginx` is a REPL-adjacent experience" — `herd eval root '(...)'` even evaluates Guile in PID 1. Pro tips: `guix system describe`, `/var/guix/profiles/system-N-link` for generation archaeology; a broken generation is recovered by picking the previous GRUB entry — demonstrate this live, it converts fear of `reconfigure` into confidence.

  **Profiles**: a profile is a union-built tree (`manifest` → profile derivation) exposed as generationed symlinks: `~/.guix-profile` → `/var/guix/profiles/per-user/$USER/guix-profile` → `guix-profile-N-link` → store. Switching generations = flipping one symlink = atomic. `guix package --roll-back`, `--switch-generation`, `--delete-generations` are all symlink/gcroot bookkeeping. The "unmentioned" bit: profiles are *also* the mechanism behind `guix shell` (ephemeral profiles, `--root` pins them as gcroots) and Guix Home (`~/.guix-home`).

  **Synthesis trio for teaching**: `guix system container config.scm` (share kernel, isolated namespaces — great classroom sandbox, zero image size), `guix system vm config.scm` (QCOW2 full-virt of the exact config — the definitive "test your installer plan" tool; add `-nic user,model=virtio-net-pci` and shared 9p dirs for realism), `guix pack -f docker --entry-point=bin/myapp myapp` (OCI image from store, bit-reproducible, no Dockerfile). The teaching arc: *same* `operating-system`/manifest → three deployment targets → this is what "configuration as data" buys you. Final pro tip: `guix system image --image-type=raw-with-offset` (and `pine64`/`rock64` image types) is how the installer *should* eventually produce SBC images — prebuilt, flashable, no on-device init at all.

  The thread to weave through the whole curriculum: **everything in Guix is a value, every value is inspectable, every deployment is a pure function of the config** — once that lands, the tools stop being a pile of commands and become one idea with many front doors.


---

# Part II: Grok CLI (`grok-4.7-build`) Raw Output

This is an architecture and teaching note for evolving `guix-platform-install` off per-host silos, for making Guix installs and rebuilds fast, and for teaching the stack underneath. It assumes the current shape you described: a Go + Guile + Bash installer, platform recipes (`cloudzy`, `framework`, `framework-dual`, `oracle`, `raspberry-pi`), and `install-this-pc.sh` that pins one NVMe by `lsblk` `MODEL` and builds `run-remote-steps` with `guix shell go`.

The target architecture is one probe, one typed inventory, one declarative plan, one apply engine. Platforms become data, not forks.

---

## 1. General-hardware `guix-platform-install`

### 1.1 Split the installer into four strata

Per-platform scripts rot because they mix four different problems:

1. **Inventory.** What is this machine, as facts from sysfs, EFI, and blkid.
2. **Intent.** What the operator asked for (target disk, filesystem, libre vs nonguix, hostname, users).
3. **Plan.** A pure value: partitions, LUKS, filesystems, bootloader, and an `operating-system` S-expression. No side effects.
4. **Apply.** The only stratum allowed to write: `sgdisk`/`parted`, `cryptsetup`, `mkfs`, `mount`, `guix system init`, `guix system reconfigure`.

Go is a good UI and SSH/orchestrator. Guile should own the plan, because the artifact you emit is already Guile. Bash should shrink to a rescue shim that can format and mount when Guile is not yet on the target.

Suggested pipeline:

```
probe (Go or Guile) → inventory.scm (read-only S-exp)
intent (CLI/Org)    → intent.scm
plan  (Guile)       → plan.scm + config.scm + plan.txt
lock                → operator confirms blast radius
apply               → run only the steps listed in plan.scm
```

Serialize inventory and plan as S-expressions with a version field (`(installer-plan 3 ...)`). Pretty-print them into the install log. The same planner runs in dry-run on a laptop and on the metal.

Platforms become **quirks tables**, not codepaths:

| Quirk key | Examples |
|---|---|
| `firmware-interface` | `uefi`, `bios`, `u-boot-efi`, `raspberry-pi-boot` |
| `disk-bus` | `nvme`, `scsi`, `virtio`, `mmc` |
| `board` | `framework-13-amd`, `pi4`, `oracle-ampere`, `generic` |
| `initrd-extras` | `nvme`, `thunderbolt`, `usb-storage` |
| `firmware-packages` | empty, or nonguix `linux-firmware` / `sof-firmware` |

`framework` and `framework-dual` collapse into one board quirk plus a disk-role map. `cloudzy` and `oracle` collapse into virtio + UEFI. The Pi stays a board quirk because its boot ROM is not GRUB.

### 1.2 CPU and architecture matrix

Detect architecture from `uname -m` and refine with `/proc/cpuinfo` and DMI. The kernel and the Guix system triplet are separate decisions.

**x86_64 Intel.** DMI and CPUID vendor `GenuineIntel`. Default kernel is `linux-libre`. Microcode is a non-free blob: ship it only on the nonguix opt-in path, as a package in the `firmware` field (and, where the board needs it early, as an initrd module). Do not bake `intel-microcode` into the libre image. Early microcode load matters for laptop suspend and for speculative-execution mitigations; document that a libre install will boot without it and that CPU errata stay unpatched.

**x86_64 AMD.** Vendor `AuthenticAMD`. Same split: `amd-microcode` only behind nonguix. Framework AMD boards additionally need the nonguix kernel or `linux-firmware` for Wi-Fi/GPU, and often `sof-firmware` for the audio DSP. That is a board feature flag (`needs-sof?`), not a copy of the whole installer.

**i686 / x86_64-linux on BIOS-era machines.** Still `x86_64-linux` or `i686-linux` system types. Bootloader branch is BIOS (below), not a CPU branch.

**aarch64-linux, three families that must not share a recipe.**

- **SBCs with a vendor boot ROM (Pi 4/5, many Allwinner/Rockchip boards).** The ROM loads firmware from a FAT partition with a fixed layout (`config.txt`, `u-boot.bin`, board dtbs). Guix already has `operating-system` helpers and bootloader records for some of these. Treat them as `boot-method: vendor-boot-fat`, not as GRUB-EFI. The root filesystem can still be ext4 or Btrfs on a later partition. U-Boot is part of the image, not something you install with `grub-install` after the fact.
- **UEFI ARM servers (Ampere Altra, generic SBSA/SBBR machines, many Oracle ARM shapes).** These look like PCs: EFI system partition, `grub-efi-bootloader` or the removable variant, device tree or ACPI. Probe for `/sys/firmware/efi`. If it exists, you are in the UEFI planner, and the CPU being ARM only changes the system type to `aarch64-linux` and the EFI binary path (`BOOTAA64.EFI`, Grub's ARM64 target).
- **Apple Silicon.** Not a Guix System target you should pretend is generic. Asahi is a separate firmware and kernel fork. The installer should detect Apple via DMI and refuse with a pointer, rather than emit a config that cannot boot.

**Cross-install.** `guix system init` on an x86_64 installer can target `aarch64-linux` only if you pass `--system=aarch64-linux` and have either substitutes or a transparent emulation setup (`qemu-binfmt`). Do not enable binfmt as a surprise. Cloud images should be built where the substitutes exist (`guix system image --system=aarch64-linux` on a machine that can get them) and then flashed.

**What the config must parameterize.**

```scheme
(operating-system
  (kernel linux-libre)          ; or linux, from the feature flag
  (firmware (list ...))         ; '() on the libre path
  (initrd-modules
   (append
    (cond ((eq? bus 'nvme) '("nvme"))
          ((eq? bus 'mmc)  '("mmc_block" "sdhci"))
          ((eq? bus 'virtio) '("virtio_pci" "virtio_scsi" "virtio_blk"))
          (else '()))
    %base-initrd-modules)))
```

Missing `nvme` or `virtio_blk` in the initrd is the classic "GRUB loads, then Guile initrd cannot find `/gnu/store`" failure. The planner derives initrd modules from the bus of the **target root disk**, not from the live USB's bus.

### 1.3 Storage bus and naming

Kernel names are not identifiers. They are enumeration order.

| Bus | Whole disk | Partition | Separator |
|---|---|---|---|
| NVMe | `/dev/nvme0n1` | `/dev/nvme0n1p1` | `p` (the disk name already ends in a digit) |
| eMMC / SD | `/dev/mmcblk0` | `/dev/mmcblk0p1` | `p` |
| SATA/SCSI sd | `/dev/sda` | `/dev/sda1` | none |
| VirtIO block | `/dev/vda` | `/dev/vda1` | none |
| Xen | `/dev/xvda` | `/dev/xvda1` | none |
| NVMe multipath | `/dev/nvme0n1` may be a path, not the only name | partitions still `pN` | easy to wipe the wrong path |
| device-mapper | `/dev/dm-0`, `/dev/mapper/cryptroot` | n/a | never a partition target |

The rule that produces the partition suffix: if the whole-disk basename ends in a digit, insert `p`. That covers `nvme0n1`, `mmcblk0`, `nvme0c0n1`, and loop devices. It fails closed on names you do not recognize: the planner refuses instead of concatenating `sda` + `p` + `1`.

**Never persist a `/dev/...` name into `file-system` or GRUB search.** Persist:

- `PARTUUID` for the EFI system partition and for "find the partition before a filesystem UUID exists". GPT partition unique GUIDs survive `mkfs`.
- Filesystem `UUID` (`lsblk -no UUID`) inside `file-system-device` once the filesystem is created.
- LUKS `UUID` (`cryptsetup luksUUID`) as the `mapped-device` source.
- `/dev/disk/by-id/` (model+serial, or `nvme-eui.…`, or `wwn-…`) as the **install-time** handle for partitioning.

`by-path` is a PCI address (`pci-0000:02:00.0-nvme-1`). It is stable across reboots on a laptop and **unstable** when you move a USB drive or when a hypervisor renumbers virtio slots. Use it as a fingerprint ingredient, not as the only key.

`by-id` for NVMe has two flavors: the volatile `nvme-Samsung_SSD_…` string and the EUI/NGUID form. Prefer WWN/EUI when present.

**Layout generator.** One pure function:

```text
layout (disk roles) →
  ((esp  size: 1GiB  type: ef00  fs: fat32)
   (boot optional)
   (root type: 8300 or luks)
   (swap optional))
```

Roles, not platform names:

- `system` — the only disk that may be partitioned.
- `cold` / `work` — listed in the plan as **untouchable**. Apply checks them and aborts if a step names them.
- `raid-member`, `zfs-member` — refused by the laptop installer unless a dedicated, explicit mode is on. Guix System does not manage ZFS as a root of trust; do not half-support it.

**LUKS2 + Btrfs, the layout worth standardizing on for laptops.**

1. GPT.
2. ESP, FAT32, 1 GiB, type `EF00`, mounted at `/boot/efi`, flag `boot`.
3. One large partition, type `8300` (or a dedicated LUKS type if you prefer; GRUB just needs to find the LUKS container).
4. `cryptsetup luksFormat --type luks2` with argon2id. Record the PBKDF parameters in the plan so a re-run does not silently change them. GRUB's LUKS2 support is limited: it reads the token to unlock `/gnu/store` only if the keyslot is one GRUB understands (pbkdf2 is the conservative choice when GRUB itself must unlock; argon2id works if an unencrypted `/boot` holds the kernel and initrd). Pick one and encode it as a plan enum:
   - `grub-decrypts-root`: LUKS1 or LUKS2+pbkdf2, no separate `/boot`.
   - `initrd-decrypts-root`: argon2id, ESP (and optionally ext4 `/boot`) outside LUKS, `mapped-devices` in the config.
5. Inside the mapper: Btrfs with subvolumes `@` (root), `@home`, `@gnu-store` only if you really want the store separate (usually you do **not**; the store is a tree of hardlinks and lives well on `@`), `@log`, `@snapshots`.
6. `file-system` entries use `(device (uuid "…" 'btrfs))` and `(options "subvol=@,compress=zstd,space_cache=v2")`. The subvolume that holds `/gnu/store` needs `(needed-for-boot? #t)`.

**ext4 layout** for servers and first installs: ESP + one ext4 root, optional swap partition. Fewer moving parts when you are teaching. Offer it as `(filesystem-type 'ext4)`.

Swap on a LUKS laptop should be a swap **file** on the encrypted volume, or a second LUKS mapping, never a cleartext partition. Resume-from-disk needs the unlock to happen in the initrd; Guix supports encrypted swap via `mapped-devices` plus a `swap-device` on the mapper. Hibernation is a feature flag, default off, because it forces swap size ≥ RAM and complicates the initrd.

**Partition-separator bugs to test explicitly.**

- `nvme0n1` + naive `disk + "1"` → `nvme0n11`, which can be a real other namespace. The planner must reject any generated path that does not match `^<disk>p[0-9]+$` for digit-terminated disks.
- `mmcblk0boot0` / `mmcblk0rpmb` are not install targets. Exclude `name` matching `boot[0-9]` and `rpmb`.
- A USB installer appearing as `sda` while the internal NVMe is `nvme0n1`. The system disk is chosen by identity, never by "the only non-removable disk" without a confirmation, because some NVMe enclosures show `RM=0`.

### 1.4 Multi-disk safety (the evolution of `install-this-pc.sh`)

`MODEL` alone is not an identity. Two Framework SSDs, two Samsung 980s, or a replaced drive with the same model string will collide. Identity is a tuple, strongest available first:

1. `ID_WWN` / `ID_WWN_WITH_EXTENSION` from `lsblk -o NAME,MODEL,SERIAL,WWN,HCTL,TRAN,PATH` or `/run/udev/data/b…`.
2. `ID_SERIAL_SHORT` (NVMe serial).
3. PCI/`by-path` as a **location** hint, stored separately from identity.
4. `MODEL` as a human check, never as the only key.

**Holder and signature refcheck, before any `wipefs` without `-n`.**

For every block device the plan might touch, collect:

- `lsblk -J -o NAME,TYPE,FSTYPE,UUID,PARTUUID,MOUNTPOINTS,MODEL,SERIAL,WWN,SIZE,TRAN,ROTA,RM,PKNAME`
- `/sys/block/<disk>/holders/` — if non-empty, the disk backs a `dm-` device (LUKS, LVM). Abort unless that holder is one this installer created in this run.
- `/sys/block/<disk>/<part>/holders/` — same for partitions.
- `wipefs -n` and `blkid` — look for `zfs_member`, `LVM2_member`, `linux_raid_member`, `swap`, `crypto_LUKS`, `btrfs`, `ext4`, `ntfs`, `apfs`. Any of these on a disk not named as `system` is a hard stop.
- Mountpoints from `/proc/mounts` and findmnt, including mounts hidden in other mount namespaces if you can see them. A mounted "cold" disk is a hard stop even if the operator picked it.
- Open swap (`/proc/swaps`).
- `mdadm --detail --scan` and `/proc/mdstat` if present.

**Blast-radius lock.** The plan file lists:

```text
WILL WRITE:
  /dev/disk/by-id/nvme-eui.0025385a31b0e1c4   MODEL=…  SERIAL=…  WWN=…
WILL NOT TOUCH:
  /dev/disk/by-id/ata-CT1000…                 MODEL=…  reason=role:work
  /dev/sda                                    reason=usb-installer
```

Apply refuses to run unless:

- `plan.scm` contains the full identity tuple.
- A fresh probe, taken immediately before write, matches that tuple byte for byte (serial and WWN).
- The operator passed `--commit <plan-sha256>` where the hash is of the canonical S-expression. A dry-run prints the hash and writes nothing.
- The write set is exactly one whole disk, or an explicit allow-list of partitions on that disk.

`install-this-pc.sh` today pins MODEL at script-generation time. Move that pin into the plan hash, and re-probe at apply time. Generating the script on a different boot, after a USB stick has shifted `sda`→`sdb`, must not be able to reuse an old plan.

Also refuse:

- Disks smaller than a floor (for example 32 GiB) unless `--allow-small`.
- The disk that contains the running root, unless the mode is `reconfigure` rather than `init`.
- Devices with `ro=1`.
- `/dev/fd0`, `/dev/zram*`, `/dev/loop*` unless the target is explicitly a disk image (the image mode uses a file, `losetup --show --find --partscan`, and the same planner).

Dry-run output should be the exact `sgdisk` invocation and the exact `operating-system` form, so a student can diff two plans.

### 1.5 Bootloader and firmware matrix

Probe `/sys/firmware/efi`. Present means UEFI. Absent means BIOS or a ROM that booted you some other way. Also read `efibootmgr -v` when `efivarfs` is mounted. If the firmware interface is UEFI but NVRAM writes fail (common in some VPS and locked-down firmwares), you already know to use the removable path.

**UEFI.**

- ESP, FAT32, GPT type `EF00`, mounted `/boot/efi`.
- Default: `(bootloader-configuration (bootloader grub-efi-bootloader) (targets '("/boot/efi")))`. This registers a `Boot####` variable via efibootmgr.
- Fallback when NVRAM is immutable, when installing for a disk that will move machines, or when `efibootmgr` fails in the plan's preflight: `grub-efi-removable-bootloader`, which writes `EFI/BOOT/BOOTX64.EFI` (or `BOOTAA64.EFI`). Firmware finds it without variables. Many VPS images should just default to removable.
- Do not mount the ESP at `/boot`. Guix wants the kernel and initrd on a filesystem GRUB can read, and an unencrypted ESP is the usual place only for the bootloader binary. If root is LUKS with argon2id, put kernels on an unencrypted ext4 `/boot` **or** use a GRUB-readable LUKS and let `/boot` live on root. Encode one. Mixing them produces a config whose `linux` line points at a path GRUB cannot see.

**Legacy BIOS.**

- GPT disks still need a BIOS Boot Partition, type `EF02`, about 1–2 MiB, no filesystem, for GRUB's `core.img`. `(bootloader grub-bootloader)` with `(targets '("/dev/disk/by-id/…"))` — the **disk**, not a partition.
- MBR (msdos label): no `EF02` partition; GRUB embeds in the post-MBR gap. Prefer GPT+`EF02` for anything new. The planner should refuse MBR unless `--label=msdos` is set, because students otherwise end up with an unbootable GPT disk and no BIOS boot partition.
- Never install BIOS GRUB onto a disk that failed the identity check. `grub-install` to the wrong disk overwrites another drive's MBR.

**Libre vs nonguix, as a channel boundary.**

Keep three composable modules:

1. `base.scm` — users, sudo, ssh, locales, `linux-libre`, `firmware '()`, substitute URL `https://ci.guix.gnu.org` and `https://bordeaux.guix.gnu.org`, only the default Guix signing key.
2. `hardware/<board>.scm` — `initrd-modules`, `kernel-arguments` (`quiet`, `nvme_core.default_ps_max_latency_us`, `mem_sleep_default`), udev rules, `swap-devices`.
3. `profiles/nonguix.scm` — **opt-in**. Adds the nonguix channel, the nonguix signing key, `(kernel linux)`, `(firmware (list linux-firmware sof-firmware))`, and any `kernel-loadable-modules` for NVIDIA or similar.

Channel file the installer may write only in opt-in mode:

```scheme
(list (channel
        (name 'guix)
        (url "https://git.savannah.gnu.org/git/guix.git")
        (branch "master")
        (commit "PINNED-COMMIT"))
      (channel
        (name 'nonguix)
        (url "https://gitlab.com/nonguix/nonguix")
        (branch "master")
        (commit "PINNED-COMMIT")
        (introduction
         (make-channel-introduction
          "897c1a470da759236cc11798f4e0a5f7d4d59fbc"
          (openpgp-fingerprint
           "2A39 3FFF 68F4 EF7A 3D29  12AF 6F51 20A0 22FB B2D5")))))
```

Pin commits. Floating `master` in an installer makes yesterday's machine and today's machine different systems. Authorize the nonguix substitute key as a separate explicit step (`guix archive --authorize`), never by appending it to the libre image's default keys. Libre installs must still function with `firmware '()` and `linux-libre`: Wi-Fi and GPU acceleration may be missing, and the installer should say so in the plan banner before the operator commits.

NVIDIA and Broadcom are per-board packages inside the nonguix module. The base module does not import `(nongnu packages …)`. That import is the line that makes a config unloadable on a pure Guix host; keep it in the opt-in file so `guix system build base.scm` stays a valid libre test.

### 1.6 Probing and synthesizing `operating-system`

Probe into an S-expression the planner can `match` on. Whether Go or Guile collects it matters less than the schema:

```scheme
(inventory
  (version 3)
  (arch "x86_64")
  (firmware-interface uefi)
  (dmi (vendor "Framework") (product "Laptop 13 (AMD Ryzen 7040Series)") (bios-version "…"))
  (cpu (vendor "AuthenticAMD") (family 25) (model 116))
  (pci (("c1" "00" "03") (class "0300") (vendor "1002") (device "15bf"))) ; lspci -nn
  (usb (("0bda" "8852")))
  (block
   ((name "nvme0n1") (size 1024…) (model "…") (serial "…") (wwn "eui.…")
    (tran "nvme") (rm? #f)
    (parts ()))))
```

Sources, in order of trust for scripting:

- `/sys/class/dmi/id/{sys_vendor,product_name,product_version,bios_version}` — empty on some VMs; that emptiness is itself a fact.
- `/sys/class/block/*/device` and `/sys/block/*/queue/rotational`.
- `lsblk -J` for topology.
- `lspci -nn` for class/vendor/device. Match on the four-digit IDs, not on the text name. `0x8086` / `0x1002` / `0x10de` decide firmware hints.
- `/sys/firmware/devicetree/base/model` on ARM.
- `guix repl` can do this without Go: `(ice-9 popen)`, `(ice-9 rdelim)`, `(json)`. Go is justified for the CLI, the disk-safety lock, and parallel SSH to a VPS. The **synthesis** should be Guile so you do not maintain two config languages.

**Composable records, not one giant file.**

```scheme
(use-modules (gnu)
             (gnu system)
             ((hardware framework-13-amd) #:prefix hw:)
             ((profile laptop) #:prefix profile:))

(operating-system
  (inherit %base)
  (host-name "frame")
  (bootloader hw:bootloader)
  (mapped-devices hw:mapped-devices)
  (file-systems (append hw:file-systems %base-file-systems))
  (kernel profile:kernel)
  (firmware profile:firmware)
  (initrd-modules hw:initrd-modules)
  (packages (append profile:packages %base-packages))
  (services (append profile:services %base-services)))
```

`inherit` works because `operating-system` is a `define-record-type*`. Hardware modules export **pieces**, not a full OS, so a teaching profile and a server profile can share `hw:file-systems`.

`guix system init config.scm /mnt` is the apply step for a fresh root. `guix system reconfigure` is the apply step when the store is already the running system. The installer should not shell out to ad-hoc `cp` of config files into the new system; put `/etc/config.scm` in place via the `operating-system` itself or a documented copy from `/mnt/etc/config.scm` after init, and keep that path identical to the plan's `config.scm` hash.

Image mode (`guix system image`) is the cloud path: the same modules, a `disk-image` or `docker-image` type, no `install-this-pc` disk probe. VPS providers then receive an image. In-place `guix system init` over SSH stays available for a provider that only gives you a rescue shell.

---

## 2. Top 10 build and substitute speed optimizations

Ranked by leverage on a machine that already has a recent Guix. Exact fields drift by release; the daemon options below are the stable CLI, passed through `extra-options` when the record has no dedicated field.

### 1. Hit substitutes before you build

The largest win is not compiling. Confirm with `guix weather` and `guix build --dry-run -v1`.

```scheme
(service guix-service-type
  (guix-configuration
    (substitute-urls
     (list "https://ci.guix.gnu.org"
           "https://bordeaux.guix.gnu.org"))
    (authorized-keys
     (list %default-authorized-guix-keys))
    (discover? #f)))
```

Add a nonguix mirror and its key **only** in the nonguix profile. Order is preference order. If ci is sick, bordeaux should still answer; `guix weather` tells you which is actually publishing the derivations you need. A URL that times out on every narinfo dominates wall time, so drop dead mirrors instead of stacking them.

Caveat: substitute URLs inside `guix-configuration` are baked into the system. A laptop offline at boot simply falls through to local builds. `guix-daemon` does not require the network to start.

The client is Guile HTTP. Do not plan on HTTP/2 multiplexing saving you inside `guix substitute`. Front a local cache with nginx if you want HTTP/2 or a single TCP connection from a fleet; the daemon still sees one fast origin.

### 2. A LAN `guix publish` cache in front of both build farms

One machine on the network runs:

```bash
guix publish \
  --port=8080 \
  --advertise \
  --ttl=2592000 \
  --workers=4 \
  --cache=/var/cache/guix/publish \
  --cache-bypass-threshold=10MiB \
  --compression=zstd:7
```

Clients:

```scheme
(substitute-urls
 (list "http://guix-cache.local:8080"
       "https://ci.guix.gnu.org"
       "https://bordeaux.guix.gnu.org"))
(discover? #t)   ; Avahi finds --advertise publishers
```

`--compression=zstd:7` is the knee of the curve. `zstd:19` burns CPU on the publisher and stalls a small VM. `lzip` is smaller and much slower to compress; use it when the link is the bottleneck and the cache is warm. `--workers` should match the publisher's cores, not the client's. `--cache` stores already-compressed nars so the second laptop does not recompress. narinfo TTL around 30 days is fine on a trusted LAN; shorter TTLs cause needless revalidation storms after `guix pull`.

Authorize the publisher's signing key on every client (`guix archive --authorize`). An unsigned publish is rejected. `--advertise` uses Avahi; it works only on a link-local multicast domain, not across routed VPNs unless you run a reflector.

Caveat: `guix publish` serves **your** store. It is not a transparent mirror of ci unless you have built or substituted those items locally first. Seed it by building the image you care about on the cache host.

### 3. Size the daemon for the machine

```bash
guix-daemon --build-users-group=guixbuild \
            --max-jobs=4 \
            --cores=4 \
            --max-silent-time=3600 \
            --timeout=86400
```

Or:

```scheme
(guix-configuration
  (extra-options
   (list "--max-jobs=4"
         "--cores=4"
         "--gc-keep-derivations=yes"
         "--gc-keep-outputs=yes")))
```

`--cores` is cores **per job**. `--max-jobs` is parallel jobs. Total threads is the product. On an 8-core laptop, `--max-jobs=2 --cores=4` usually beats `--max-jobs=8 --cores=1` because Guile, gcc, and rustc all parallelize internally, and eight linkers will thrash disk. Set `build-accounts` in `guix-configuration` at least as high as `--max-jobs` (the default 10 is enough until you raise jobs).

`--cache-failures` is a footgun on a teaching machine: a transient test failure or a full disk gets remembered, and later builds say "permanently failed" until `guix gc` or a daemon without that flag. Leave it off unless you are bisecting and want failed derivations skipped.

`--gc-keep-derivations=yes` and `--gc-keep-outputs=yes` spend disk to keep grafts and rebuilds cheap. Turn keep-outputs off on small VPS disks. Derivations are small; outputs are not.

### 4. Offload heavy builds

`/etc/guix/machines.scm` (or the `build-machines` field, which generates it):

```scheme
(list (build-machine
        (name "builder.local")
        (user "offload")
        (system "x86_64-linux")
        (host-key "ssh-ed25519 AAAA…")
        (private-key "/etc/guix/offload-key")
        (speed 4)
        (parallel-builds 4)
        (compression "zstd")
        (overload-threshold 0.6)))
```

`speed` is relative. A machine at speed 4 is preferred over the local speed-1 daemon for jobs it can do. `system` routes `aarch64-linux` jobs to an ARM box so you do not emulate. SSH multiplexing (`ControlMaster` in the offload user's `ssh_config`, or Guix's own reuse) matters because every derivation is a connection. The offload user must have a login shell that can run `guix repl` and the store must be visible; a locked-down `command=` forced command breaks the protocol.

Caveat: offload still **evaluates** the derivation locally. A slow laptop spends time in `guix system reconfigure` Guile before any core on the builder spins. Evaluation speed is optimization 8. Also, offloaded failures are harder to read; keep one local core available so small packages do not pay SSH latency (`--max-jobs=1` locally plus offload).

### 5. Do not `--tune` if you want substitutes

`guix build --tune=native` (and the package transformation `tuned-package`) rewrites compiler flags toward this CPU (`-march=native` or a named psABI level). The resulting derivation hash differs from what ci and bordeaux published, so you build the whole graph.

Use `--tune` only for a leaf you measured (a numerical library, a video encoder). Leave the system, Guile, and the toolchain untuned so they substitute. If you need a faster baseline for everything, a custom channel that changes the default `-march` is a distribution fork: own the build farm, or accept cold caches.

psABI levels (`x86-64-v2`, `v3`, `v4`) are the right vocabulary when you do tune, because `native` on a Zen 4 box produces binaries that SIGILL on the VPS. Pin the level in the plan.

### 6. tmpfs for `TMPDIR` only when the package fits

```bash
mount -t tmpfs -o size=24G,mode=0755 tmpfs /gnu/tmp-build
# daemon --tmpdir=/gnu/tmp-build
# or: (guix-configuration (tmpdir "/gnu/tmp-build"))
```

The daemon build directory defaults under `/tmp`, which on many Guix Systems is already tmpfs and **half of RAM**. That is perfect for small packages and fatal for `llvm`, `rust`, `webkit`, `ungoogled-chromium`, `libreoffice`. Those need tens of gigabytes. A silent OOM killer looks like a flaky build.

Policy: default `TMPDIR` on disk (`/var/tmp/guix-build`). For a CI machine with 64 GiB RAM and small packages, a sized tmpfs is reasonable. Watch `/tmp` usage during the first chromium build before you switch a laptop. `tpmfs` does not make downloads faster; those are fixed-output derivations written into the store.

### 7. Trust and key setup, once

Every failed signature is a stall and a retry. On install:

- Official ci and bordeaux keys are in `%default-authorized-guix-keys` when `authorize-key?` is true. Do not hand-edit `/etc/guix/acl` and also declare keys in the config; one source.
- Nonguix key via `guix archive --authorize` from the published key, after you have read the fingerprint out of band.
- Clock skew makes TLS and narinfo look "corrupt". `guix` before NTP has finished is a common first-boot failure. The installer should start a time sync before the first substitute.

`guix-daemon --disable-chroot` is faster to set up and **not** an optimization. It leaks host paths into builds and breaks reproducibility. Keep the chroot. Add only the `/dev` nodes you must via `chroot-directories`.

### 8. Make `guix pull` and `time-machine` cheap

`guix pull` compiles a large Guile tree into a new profile of `.go` files. The first pull on a slow VPS is the thing students think is "Guix being slow".

- Pull on a fast machine, publish, substitute the pull's guix profile, or distribute a pinned `channels.scm` and a pre-pulled profile.
- `guix time-machine -C channels.scm -- …` caches inferiors under `~/.cache/guix/inferiors`. Reuse the same channel file. Changing the commit by one character is a new inferior and a new compile.
- `GUILE_LOAD_COMPILED_PATH` must point at that profile's `lib/guile/3.0/site-ccache`. If you run a `guix` binary with `GUILE_LOAD_PATH` into a dirty git checkout and an empty compiled path, Guile autocompiles into `~/.cache/guile` on every invocation. The symptom is a multi-second pause before `guix describe` prints. Fix the path; do not sprinkle `guild compile` at random.
- For development of the installer itself, `guix shell -D guix --pure` plus `./pre-inst-env` is the supported way to run a checkout. Teach that before anyone edits `~/.config/guix/current`.

Grafts: security replacements rewrite references in dependents. They are cheaper than a rebuild and still add a derivation layer. After a big graft (glibc, git), dry-runs show many "grafted from" lines and substitute hits depend on the build farm having built the grafts. If you pin an old Guix, you may graft locally for a long time. Pinning recent commits avoids that better than any daemon flag.

### 9. GIPS / content-addressed nar sharing, as transport, not as trust

Official substitutes are: a `.narinfo` signed by the build-farm key, plus a `.nar` (possibly compressed). The store file name is still **input-addressed** (hash of the derivation's inputs), except fixed-output derivations (downloads), whose hash is of the content.

GIPS-style designs (IPFS, GNUnet FS, or a libp2p DHT) replace the HTTP fetch of the nar body with a content-addressed fetch. The CID/URN is inside the signed narinfo. Trust stays the OpenPGP signature and your ACL. The DHT does not decide authenticity. A wrong-but-popular block fails the signature check, or fails the nar hash, and must be rejected. That is the whole security story; do not teach "IPFS means reproducible."

Practical latency:

- **Warm cache / mDNS / LAN peer:** can beat ci for a multi-gigabyte nar, because you avoid the WAN and you already have the bytes next door. This overlaps `guix publish --advertise` and is the version worth deploying.
- **Cold DHT:** often slower than bordeaux. Discovery, provider records, and block-by-block fetch lose to one HTTPS connection from a CDN-like CI. GNUnet GNS names and IPFS gateways add more moving parts (daemon, identity, NAT). Treat them as an experiment with a fallback URL list, never as the only substitute source.
- Fraud proofs and a web of trust among publishers are research on top of the existing key authorization. Until that is in your Guix, the operational control is still `authorized-keys`.

Use GIPS when you have many machines and a fat nar that CI is slow to serve. Measure a cold fetch and a warm fetch separately before you recommend it in the curriculum.

### 10. Avoid doing the work

- `guix gc -d` and `--delete-generations` reclaim more time-to-next-build than any compression flag, because a full disk turns every build into an error.
- `guix shell -m manifest.scm` with a committed manifest substitutes one environment instead of ad-hoc `guix shell a b c` which still hits the cache but is harder to share.
- `guix build -P 1 <pkg>` rebuilds dependents; do not use it as a habit.
- For the installer, prebuild `guix system image` in CI and hand the VPS an image. On-machine `guix system init` from source is a teaching exercise, not the fastest path to a host.
- Log compression (`log-compression 'zstd` or `gzip` in `guix-configuration`) saves disk on `/var/log/guix/drvs`. It does not speed builds. zstd logs are nicer to keep.

**A sane laptop daemon, summarized:**

```scheme
(guix-configuration
  (substitute-urls
   (list "http://10.0.0.2:8080"
         "https://ci.guix.gnu.org"
         "https://bordeaux.guix.gnu.org"))
  (discover? #t)
  (max-silent-time 3600)
  (timeout (* 24 3600))
  (tmpdir "/var/tmp/guix-build")
  (extra-options
   (list "--max-jobs=2"
         "--cores=4"
         "--gc-keep-derivations=yes"
         "--gc-keep-outputs=yes")))
```

Verify with `guix weather -m manifest.scm`, then `guix build --dry-run`, then one real build while watching `guix processes` (or the daemon log). If dry-run says "would be built" for `guix` itself, you are off the substitute path and no core-count tweak will feel fast.

---

## 3. Mental models and the things manuals skip

### 3.1 Guile Scheme

**Homoiconicity means the macro sees your source as data, and the evaluator sees the macro's output as source.** It does not mean "code and data are the same at runtime." By the time a function runs, macros have already expanded, and records are already records.

Three times, and students need all three named:

| Time | What runs | Example |
|---|---|---|
| Expand | Macros, at compile/load | `syntax-rules`, `define-record-type*` |
| Host / run | The Guile that is evaluating your OS config or `guix repl` | looking up `linux-libre`, building a gexp |
| Build | A derivation's builder, often another Guile, in the chroot | the `#~` script |

**Quote stops evaluation. Quasiquote stops it, then unquote punches holes.**

```scheme
'(a ,b)    ; ⇒ (a (unquote b))     the comma is literal data
`(a ,b)    ; ⇒ (a 42)              if b is 42
`#(a ,b)   ; vector, same rule
```

G-expressions are a second quasiquote that does not splice host values into the build script. It splices **store references**.

```scheme
#~(begin
    (use-modules (guix build utils))
    (copy-file #$(file-append coreutils "/bin/ls") "/tmp/ls"))
```

- `#~` is "build a gexp". The `lambda`-like body is not run now.
- `#$` is "insert the store item of this host value". `coreutils` the package object becomes `/gnu/store/…-coreutils-…/bin/ls` as a string **on the build side**.
- `#$@` is the list version: splice a list of packages as consecutive references.
- `ungexp` and `ungexp-splice` are the long names of `#$` and `#$@`.

If you `#$` something that is not a package, origin, file-like, or gexp, you get a confusing type error at **host** time. If the script fails, that is **build** time, and the log is in `/var/log/guix/drvs/…`. Teach students to say which time they are in before they debug.

A gexp is turned into a derivation when something lowers it (`guix build`, `gexp->derivation`, `operating-system-derivation`). Reading `#~` does not talk to the daemon. That delay is why an OS config can `define` packages and only hit the network when you `reconfigure`.

**Macros.** `syntax-rules` is a pattern rewriter. Reach for it first. `syntax-case` is for when you must break hygiene or inspect identifiers (`datum->syntax` is the escape hatch, and it is how you make bugs that only appear at the call site). `define-record-type*` (Guix, not RNRS) gives you field defaults, thunked fields, and `inherit`. `operating-system`, `file-system`, and `package` are all this. Thunked fields (`packages`, `services`) delay evaluation so cyclic module graphs can load. If a student forces the field at the top level during module load, they reintroduce the cycle.

**`(ice-9 match)`.** Use it on inventories and on gexp inputs. It is the control structure that replaces long `cond` chains over lists. Match `('nvme …)` and `('scsi …)` explicitly; a catch-all clause should raise, not guess a partition suffix.

**Debugging.**

- `guix repl` is a Guile with Guix modules on the load path. `,use (gnu system)` works there. A naked `guile` often does not.
- Geiser or Arei: the important commands are "eval defun", "show expansion" (`macroexpand`), and the backtrace. `,bt` in the REPL. Expansion errors point at the macro; runtime errors point at the expanded code, which is why you expand first.
- `#:use-module` versus `#:autoload` matters when teaching configs. A top-level use of `(nongnu …)` on a libre machine fails at **load**, before any `if`. Guard the import by splitting modules, not with `if`.
- The stack is not the Bash stack. `pk` (from `(guix diagnostics)` or plain `pk`) prints and returns its argument. Sprinkle `pk` in the planner. Remove it before the plan hash exists, because debug output must not be in the hashed value.

### 3.2 Emacs

**Buffers hold text. Windows display buffers. Frames are OS windows. Processes are asynchronous and deliver output into buffers or filters.** A beginner who thinks "the file is open" will mis-debug. The file is visited by a buffer; the buffer may be unsaved; a process may be appending to a different buffer (`*Guix REPL*`, `*compilation*`).

**`early-init.el` runs before the GUI and before packages.** Set `gc-cons-threshold` tuning only if you measure, inhibit the menu bar, and set `package-enable-at-startup` to `nil` when Guix Home owns the load path. **`init.el` runs after.** Putting `load-theme` work in `early-init.el` is a common source of "Emacs flashes wrong, then fixes itself."

**The narrow completion stack, and the job of each piece.**

- Vertico: the completion **UI** in the minibuffer.
- Orderless: how a query splits into matchers (space-separated, out of order).
- Marginalia: annotations (file modes, docstrings) beside candidates.
- Consult: **commands** that feed candidates (lines in buffer, ripgrep, imenu).
- Corfu: in-buffer completion popup, the minibuffer's sibling.
- Eglot: LSP client. It feeds Corfu; it is not a completion UI.

If all six are installed and "completion is broken," disable them from the top: Eglot off, then Corfu, then Vertico. One of them is fighting `completion-styles`.

**Guix Home versus `package.el`.** `home-emacs-service-type` builds an Emacs with packages as store items and a generated `init.el`. That Emacs is reproducible and slightly annoying to iterate on: a one-line elisp change is a `guix home reconfigure`. The workable split is:

- Packages and `load-path` from Guix Home (no MELPA in class; network failures during `package-install` waste the day).
- Personal keybindings in a small file under `~/` that `init.el` loads, if you accept that this file is unreproducible state.
- Or everything in the Home config, and students learn to reconfigure. Prefer this for the course machines so nobody has a private `~/.emacs.d` that the instructor cannot see.

Do not mix `use-package` with MELPA and Guix Emacs in the same init without a rule. Two package managers installing `magit` is the usual broken state.

**Parentheses.** Paredit is strict and the right teacher for a month: you cannot unbalance the buffer, so you learn slurp, barf, and raise. Smartparens is gentler and easier to fight. Puni is a smaller modern set of the same structural commands. Pick one. The pro tip manuals assume: **`C-M-u` / `C-M-d` (up/down list) and `C-M-f` / `C-M-b` (forward/back sexp) matter more than any visual rainbow.** Rainbow delimiters help reading and do not help editing. Also turn on `show-paren-mode` and learn `check-parens` when something will not eval. Geiser's "I was evaluating the wrong sexp" problem is almost always point placement: eval the enclosing `defun`, not the inner `let` you half-selected.

### 3.3 Org-mode literate programming

One `.org` file tangles to `base.scm`, `hardware/framework.scm`, `channels.scm`, and a shell installer. The drift failure is editing the tangled file and forgetting. Make the tangled files **generated**: the apply script runs `emacs --batch -l org --eval '(org-babel-tangle-file …)'` or, better, a Makefile whose only source of truth is the Org file. Commit the Org file. Either commit the tangled results and CI-fail if they differ, or do not commit them and always tangle in the build. Committing both without a check is how they diverge.

Header arguments that matter:

```org
#+PROPERTY: header-args:scheme :mkdirp yes :comments no :noweb yes
#+PROPERTY: header-args:shell  :mkdirp yes :tangle-mode (identity #o755)
```

- `:mkdirp yes` creates `hardware/` on tangle. Without it, the first tangle on a clean checkout fails halfway and leaves a partial tree.
- `:tangle-mode (identity #o755)` is an Emacs Lisp form, not a string. Quoted `"#o755"` will not do what you want. This is the usual "why isn't my script executable" bug.
- `:noweb yes` expands `<<name>>` references at tangle time. Those names are **not** Guile variables. Noweb runs before Guile. If you put a `<<host-name>>` inside a gexp and also expect Guile to see it, you have two substitution phases. Name the noweb refs in a way that looks nothing like Scheme (`<<HW-FILE-SYSTEMS>>`).
- Property inheritance: `#+PROPERTY: header-args:scheme` applies to scheme blocks. A block-level `:tangle foo.scm` overrides it. `:header-args:` (no language) applies to all languages and will accidentally pass `:noweb yes` to shell blocks that contain `<<`. Set language-specific properties.
- `:comments no` keeps the tangled Scheme loadable and diffable. Org's default comment links are useful once and then they make `guix system reconfigure` errors point at the tangled file with extra noise. Keep `:comments link` only while teaching tangle, then turn it off.
- `org-babel-tangle` does not evaluate the Scheme. Students think tangle "checks" the config. It does not. The check is `guix system build -n config.scm` (or `guix home build -n`). Put that in the same Makefile.
- Drawer `:PROPERTIES:` on a heading override file-level properties for blocks under that heading. A hardware section can `:tangle hardware/pi.scm` while the rest tangles to `base.scm`. This is the right structure for one document, many boards.
- `noweb` and indentation: tangled output keeps the Org source's indentation unless you set `:noweb-prefix no`. Gexps inside a heading at column 0 are easier to tangle than gexps nested under a list item.

Source of truth rule for the course: if it is not in the Org file, it is not configuration. The dry-run plan from section 1 can be an Org result block, but the committed plan hash should be produced by the Guile planner, not by hand-edited tables.

### 3.4 Linux and Guix internals

**The store path is a claim about inputs, not about bytes.**

`/gnu/store/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-hello-2.12.1`

Those 32 characters are a base32 hash of the **derivation inputs**: sources, compiler, flags, environment, and the build script. Two different recipes must not land on the same path. The same recipe must. This is why a timestamp in the build script costs you a world rebuild, and why `--tune` misses the cache.

Exceptions you should make students point at:

- **Fixed-output derivations.** `origin` downloads. The hash in the recipe **is** the file hash (`sha256` in the `origin`). The builder may use the network. If the bytes do not match, the build fails. Mirrors are allowed because the hash, not the URL, is the identity.
- **Grafts.** A new output path that is a reference-rewritten copy of an old build, used so a security fix in OpenSSL does not rebuild every package from source. `guix build --no-grafts` shows the ungrafted dependency. When a student says "I changed one line and 400 packages rebuilt," they are often looking at grafts being applied, not at full compiles. `guix build -v1` distinguishes them.
- **Content-addressed** work (interned files via `guix hash`, the daemon's `--check`) is real but it is not what names ordinary packages today. Say "input-addressed store" so nobody "corrects" you with the Nix slogan without the footnote.

The daemon is the only writer of `/gnu/store`. Root cannot `make install` into it and have a coherent system. Builders run as `guixbuilderNN` in a chroot, with a private `/tmp`, no network (unless fixed-output), and a store mounted read-only except for the output path.

**Boot, in the order a student can observe.**

1. Firmware (UEFI or BIOS) loads GRUB from the ESP or the BIOS boot partition.
2. GRUB reads a `grub.cfg` that lives in the store, reached via a small config on the ESP. Entries are **generations**. Each points at a concrete kernel and initrd path in `/gnu/store`.
3. The initrd's PID 1 is **Guile**, not systemd and not yet Shepherd. It runs the `boot-system` Guile program: find the root (decrypt LUKS here if configured), mount `/gnu/store` and the root filesystem, `pivot_root` / switch into the new root.
4. That Guile execs the system script, which starts **GNU Shepherd** as PID 1. `herd status` talks to it. Services are Scheme objects, not unit files.
5. `/run/current-system` is a symlink to the current generation's profile, which is a tree of symlinks into the store (`bin`, `etc`, the Shepherd services). `/var/guix/profiles/system-N-link` are the previous generations. `guix system roll-back` moves the symlink. It does not restore a disk image. Garbage-collecting old generations deletes the store items they alone kept alive. A generation you can still boot in GRUB is one whose store items still exist.

If GRUB appears and then drops you in a Guile backtrace, you are in step 3: wrong `initrd-modules`, wrong UUID, or LUKS parameters GRUB/initrd cannot open. If Shepherd starts and a service fails, you are in step 4: `herd start ssh-daemon`, logs under `/var/log`. Those are different layers; do not reinstall GRUB for a Shepherd failure.

**Profiles are atomic symlink trees.** A profile generation is one store item containing symlinks. Updating a profile is `rename` of a symlink, which is atomic on the same filesystem. Users' `~/.guix-profile` works the same way as the system profile. This is why a partial upgrade is not a thing inside one transaction, and why two transactions can still be mixed if you install some packages in the user profile and some in the system.

**Containers, VMs, packs.**

- `guix system container config.scm` builds the OS closure and starts it in Linux namespaces, sharing the host kernel. Fast loop for "does this service start?" No UEFI, no disk image. `herd` works inside if you enter the container.
- `guix system vm config.scm` builds a script that boots the system under QEMU, with its own kernel from the config. This is how you test bootloader and initrd changes without wiping a laptop. It is slower, and it is the right exam environment.
- `guix pack -f docker` (or `singularity`, `tar`) exports a closure as someone else's image format. It is not Guix System: there is no Shepherd generation management inside a typical pack. Use it to hand a program to a machine that will not run Guix. Use `guix system image` when you want a whole system.
- `guix shell -C` is a container around a profile, not a system. Good for class exercises that must not touch the host store's notion of "installed," and still too easy to confuse with `system container`. Draw the three boxes: shell container (packages), system container (PID 1 is Shepherd, host kernel), VM (guest kernel).

**One teaching sequence that matches the installer.**

1. `match` a hand-written inventory S-expression and emit a partition plan. No disk access. This is week-one Guile.
2. Tangle that planner from Org. Add a dry-run.
3. `guix system vm` a tiny `operating-system` with `linux-libre` and an ext4 root.
4. Add LUKS in the VM only.
5. Add a second virtual disk and show the identity lock refusing it.
6. Only then talk about nonguix, substitutes, and `guix publish`.

That order forces the two-stage model (plan is data, apply is a derivation-shaped side effect) before anyone has a custom kernel to hide in.

---

**Operational risks worth stating once.** The removable EFI path and the "GRUB decrypts root" path interact: pick them in the plan enum and test both in `guix system vm`, because a laptop firmware that ignores NVRAM will boot only the removable loader. `--cache-failures` and `--tune=native` are the two flags that look like speed and produce the longest debugging sessions. And `MODEL`-only disk pinning will eventually select the wrong identical SSD; the commit hash over WWN+serial is the actual safety property.
