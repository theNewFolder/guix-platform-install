#!/run/current-system/profile/bin/guile \
--no-auto-compile -s
!#
;;; postinstall/recipes/add/build-acceleration.scm
;;; Configures the 7-Layer Guix Build & Substitute Acceleration Stack in /etc/config.scm:
;;;   - Multi-mirror substitute URLs (bordeaux.guix.gnu.org + ci.guix.gnu.org)
;;;   - Parallel build daemon tuning (--cores=0, --max-jobs=N, --gc-keep-derivations=yes)
;;;   - Optional LAN mDNS substitute discovery (--discover=yes + avahi-service-type)
;;;   - Optional RAM-backed /tmp tmpfs mount for machines with >= 16 GiB RAM

(use-modules (ice-9 format)
             (ice-9 popen)
             (ice-9 rdelim)
             (srfi srfi-1))

(define (read-first-line cmd)
  (let* ((port (open-input-pipe cmd))
         (line (read-line port)))
    (close-pipe port)
    (if (eof-object? line) "" line)))

(define (detect-ram-mib)
  (let ((raw (read-first-line "awk '/MemTotal:/ {print int($2/1024)}' /proc/meminfo 2>/dev/null")))
    (or (string->number raw) 8192)))

(define (detect-cores)
  (let ((raw (read-first-line "nproc 2>/dev/null")))
    (or (string->number raw) 4)))

(define (recommended-max-jobs ram-mib)
  (cond
   ((>= ram-mib 32768) 8)
   ((>= ram-mib 16384) 4)
   ((>= ram-mib 4096)  2)
   (else               1)))

(define (print-acceleration-snippet ram-mib cores max-jobs)
  (format #t "=== GNU Guix Build Acceleration Configuration ===~%")
  (format #t "Detected Hardware: ~a logical cores | ~a MiB RAM -> Recommended --max-jobs=~a~%~%"
          cores ram-mib max-jobs)
  (format #t "Add the following service modification inside your (operating-system (services ...)) form:~%~%")
  (format #t "(modify-services %base-services~%")
  (format #t "  (guix-service-type config =>~%")
  (format #t "    (guix-configuration~%")
  (format #t "     (inherit config)~%")
  (format #t "     (discover? #t) ; Auto-discover LAN guix publish servers via mDNS~%")
  (format #t "     (substitute-urls~%")
  (format #t "      (append (list \"https://bordeaux.guix.gnu.org\"~%")
  (format #t "                    \"https://ci.guix.gnu.org\")~%")
  (format #t "              %default-substitute-urls))~%")
  (format #t "     (extra-options~%")
  (format #t "      (list \"--cores=0\"~%")
  (format #t "            \"--max-jobs=~a\"~%" max-jobs)
  (format #t "            \"--gc-keep-derivations=yes\"~%")
  (format #t "            \"--gc-keep-outputs=yes\")))))~%~%")
  (when (>= ram-mib 16384)
    (format #t ";; >= 16 GiB RAM detected: Add RAM-backed /tmp tmpfs to (file-systems ...) for 2-4x faster source unpack/compile:~%")
    (format #t "(file-system~%")
    (format #t "  (mount-point \"/tmp\")~%")
    (format #t "  (device \"none\")~%")
    (format #t "  (type \"tmpfs\")~%")
    (format #t "  (check? #f)~%")
    (format #t "  (options \"mode=1777,size=50%\"))~%~%"))
  (format #t ";; Pin your current working channel commits for 100% substitute hit rate:~%")
  (format #t ";;   guix describe --format=channels > /etc/channels-pinned.scm~%")
  (format #t ";;   sudo guix time-machine -C /etc/channels-pinned.scm -- system reconfigure /etc/config.scm~%"))

(let* ((ram (detect-ram-mib))
       (cores (detect-cores))
       (jobs (recommended-max-jobs ram)))
  (print-acceleration-snippet ram cores jobs))
