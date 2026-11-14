# Part 2: GNU Emacs, Org-Mode, Literate Systems Engineering & Production Mastery

> **Audience & Scope**: Modules 4, 5, and 6 of the *Guile, Guix, Emacs, Org & Linux Systems Masterclass*. Every configuration snippet, Scheme module, Emacs Lisp form, and shell pipeline in this text is complete, self-contained, and adheres strictly to Free Software (FSDG-compliant GNU Guix) and suckless declarative principles.

---

# Module 4: GNU Emacs for Lisp & Guix Engineers (From First Principles to REPL Nirvana)

## 4.1 The Emacs Mental Model: A Live Lisp Machine, Not a Text Editor

**[Concept]** Most programmers approach GNU Emacs expecting an editor like Vim or VS Code—a compiled C/Rust/Electron binary that loads text files into memory and optionally runs plugins on the side. That mental model will fight you at every step. **GNU Emacs is a bytecode- and native-compiled Lisp virtual machine whose user interface happens to be text buffers.** Only the lowest-level primitives (garbage collection, terminal/window display, subprocess spawning, and basic string/buffer storage) are written in C; the remaining ~85% of Emacs—including every keystroke handler, window splitter, file manager, Git porcelain, and Scheme IDE—is live Emacs Lisp (`elisp`) executing inside a single shared address space.

Because there is no wall between "the application" and "your configuration," every keystroke invokes a Lisp function (`self-insert-command` when you type the letter `a`). You can inspect, redefine, advise, or trace any function in the running editor without restarting it.

```
+-----------------------------------------------------------------------------------+
|                        FRAME (OS Window / Terminal Screen)                        |
|  +--------------------------------------+--------------------------------------+  |
|  | WINDOW 1 (Viewport onto Buffer A)    | WINDOW 2 (Viewport onto Buffer B)    |  |
|  |                                      |                                      |  |
|  | (define-module (my-system)           | GNU Guile 3.0.9                      |  |
|  |   #:use-module (gnu)                 | scheme@(guile-user)> (+ 2 3)         |  |
|  |   #:use-module (guix gexp))          | $1 = 5                               |  |
|  |                                      | scheme@(guile-user)> |               |  |
|  | (operating-system                    |                                      |  |
|  |   (host-name |"tuf")                 |                                      |  |
|  |   ...)                               |                                      |  |
|  |                                      |                                      |  |
|  | [Mode Line: config.scm (Scheme/G)]   | [Mode Line: *Guile REPL* (Geiser)]   |  |
|  +--------------------------------------+--------------------------------------+  |
|  | ECHO AREA / MINIBUFFER: C-M-x => $2 = #<operating-system tuf>               |  |
+-----------------------------------------------------------------------------------+
                                       ^
                                       | Views into
+--------------------------------------+--------------------------------------------+
|                        EMACS LISP VM HEAP (Buffers in RAM)                        |
|   [ Buffer: config.scm ]   [ Buffer: *Guile REPL* ]   [ Buffer: *Messages* ]      |
|   [ Buffer: *scratch*  ]   [ Buffer: init.el      ]   [ Buffer: *Guix Packages* ] |
+-----------------------------------------------------------------------------------+
```

### The Seven Core Primitives of the Emacs UI and Runtime

| Primitive | What It Actually Is | Common Newcomer Misconception | Essential Keys |
| :--- | :--- | :--- | :--- |
| **Buffer** | An in-memory Lisp object holding text, local variables, a syntax table, a keymap, and a mark ring. A buffer *may* be visiting a file on disk (`config.scm`), or it may be purely ephemeral (`*scratch*`, `*Messages*`, `*Guile REPL*`). | Thinking "closing a tab closes the file." Killing a window only hides the viewport; the buffer remains alive in memory until `kill-buffer` (`C-x k`). | `C-x b` (`switch-to-buffer`), `C-x C-b` (`ibuffer`), `C-x k` (`kill-buffer`), `C-x C-s` (`save-buffer`) |
| **Window** | A tiled rectangular viewport displaying one buffer at a specific scroll offset and cursor position. Multiple windows can view different regions of the *exact same buffer* simultaneously. | Calling an OS desktop window a "window." What macOS/X11/Wayland calls a window, Emacs calls a **Frame**. | `C-x 2` (split below), `C-x 3` (split right), `C-x 1` (keep only this window), `C-x 0` (delete this window), `C-x o` (`other-window`) |
| **Frame** | An X11, Wayland, macOS Cocoa, or TTY top-level window containing one or more Emacs Windows plus ashared Minibuffer at the bottom. An Emacs daemon (`emacs --daemon`) can serve graphical and TTY frames simultaneously from the same Lisp image. | Starting a fresh `emacs` process for every file instead of connecting via `emacsclient -c` or `emacsclient -t`. | `C-x 5 2` (`make-frame-command`), `C-x 5 0` (`delete-frame`) |
| **Minibuffer & Echo Area** | The single bottom line of a frame. When idle, it acts as the **Echo Area** (printing return values and messages). When a command prompts for input (`M-x`, `C-x C-f`), it activates as the **Minibuffer**—a full Emacs buffer where you can use history (`M-p`/`M-n`) and editing keys. | Getting trapped in the minibuffer when a prompt is open. Always press `C-g` (`keyboard-quit`) to cancel an active minibuffer prompt. | `M-x` (`execute-extended-command`), `C-g` (`keyboard-quit`), `M-p` / `M-n` (previous/next history item) |
| **Point & Mark (The Region)** | **Point** is the exact integer character offset (1-indexed!) where the cursor sits—conceptually *between* two characters. **Mark** is a saved integer position in the buffer. The contiguous span between Point and Mark is the **Region**. | Expecting Shift-arrow selection to be the only way to select text. In Emacs, you drop a Mark with `C-SPC`, move Point with any navigation command (including search or sexp motion!), and operate on the Region. | `C-SPC` (`set-mark-command`), `C-x C-x` (`exchange-point-and-mark`), `M-w` (copy region), `C-w` (kill/cut region), `C-y` (yank/paste) |
| **Major Mode** | Every buffer has **exactly one** Major Mode (`scheme-mode`, `emacs-lisp-mode`, `org-mode`, `dired-mode`, `magit-status-mode`). It defines the buffer's specialized keymap, syntax table, indentation rules, and font-lock (syntax highlighting). | Expecting global keys to do everything. Major modes bind domain-specific commands under the `C-c` prefix. | `C-h m` (`describe-mode` — lists the active major and minor modes and their keybindings) |
| **Minor Mode** | Zero or more optional, composable feature layers toggled per-buffer or globally (`geiser-mode`, `paredit-mode`, `flycheck-mode`, `rainbow-delimiters-mode`, `visual-line-mode`). | Not realizing why a keybinding changed: minor mode keymaps take precedence over the major mode keymap! | `M-x <mode-name>` toggles any minor mode on or off. |

### Key Notation Survival Table

Before writing a line of code, lock in Emacs key chord notation:
- `C-<chr>`: Hold **Control** and press `<chr>` (e.g., `C-x` is Control+x).
- `M-<chr>`: Hold **Meta** (**Alt** or **Option** on modern keyboards, or press and release `ESC` once) and press `<chr>` (e.g., `M-x` is Alt+x).
- `C-M-<chr>`: Hold **both Control and Meta** and press `<chr>` (e.g., `C-M-x` evaluates the top-level Lisp definition around point).
- `S-<chr>`: Hold **Shift** (e.g., `S-TAB` cycles global Org outline visibility).
- **Prefix Keymaps**: Emacs groups commands into trees:
  - `C-x ...` — Global core commands (files, buffers, windows, frames).
  - `C-c ...` — Mode-specific commands and user bindings (`C-c C-<letter>` belongs to the major/minor mode; `C-c <letter>` is reserved for *your* personal bindings!).
  - `C-h ...` — The self-documenting Help system (`C-h f` describe function, `C-h v` describe variable, `C-h k` describe key, `C-h m` describe mode, `C-h i` Info manuals).

---

## 4.2 Emacs Lisp (Elisp) vs. Guile Scheme: The 5 Critical Differences

**[Concept]** As a Guix engineer, you write system configurations and packages in **Guile Scheme** (an R6RS/R7RS Scheme dialect) while configuring your editor and Org-Babel workflows in **Emacs Lisp** (a cousin of Maclisp and Common Lisp). Their surface syntax looks identical—both use parenthesized prefix S-expressions—which lures newcomers into writing Scheme idioms in `init.el` or Elisp idioms in `config.scm`. Memorize these five structural differences on Day 1 and you will never be bitten by cross-dialect confusion.

| Dimension | Emacs Lisp (Elisp) | GNU Guile Scheme | The Classic Newcomer Trap |
| :--- | :--- | :--- | :--- |
| **1. Truthiness, Falsiness & the Empty List** | `nil` and `'()` are **identical** (`(eq nil '())` $\rightarrow$ `t`). `nil` is the **only** false value; everything else (including `0` and `""`) is truthy (`t`). | `#f` and `'()` are **completely distinct** (`(eq? #f '())` $\rightarrow$ `#f`). **Only `#f` is false**; `'()` (the empty list), `0`, and `""` are all **truthy** (`#t`)! | Writing `(if my-list ...)` in Guile Scheme thinking an empty list `'()` will take the false branch. In Scheme, `'()` is truthy! You **must** use `(if (null? my-list) ...)` or `(pair? my-list)`. |
| **2. Namespace Architecture (Lisp-2 vs. Lisp-1)** | **Lisp-2**: Every symbol has a *separate* value cell and function cell. A variable named `list` does not shadow the function `list`. Passing a function as a value requires `#'fn` (sharp-quote), and calling a function stored in a variable `f` requires `(funcall f arg)` or `(apply f args)`. | **Lisp-1**: Every symbol has a *single* unified binding in lexical scope. Functions are ordinary first-class values. Passing a function is just `fn`, and calling a function stored in variable `f` is simply `(f arg)`. | Naming a local variable `list` in Guile Scheme (`(let ((list '(1 2 3))) (list 4 5))`) — this shadows the `list` procedure and crashes with `Wrong type to apply: (1 2 3)`! Conversely, writing `(map (lambda (x) ...) xs)` in Elisp instead of `(mapcar (lambda (x) ...) xs)`. |
| **3. Variable Scoping (Dynamic vs. Lexical)** | **Historically Dynamic by default!** Unless the file's **very first line** contains `;;; -*- lexical-binding: t; -*-`, local variables bound by `let` or `defun` are dynamically scoped across the call stack. | **Lexically scoped by default** since the birth of Scheme (1975). Dynamic parameters require explicit `make-parameter` and `parameterize`. | Omitting `;;; -*- lexical-binding: t; -*-` on line 1 of an `.el` file, causing closures (`lambda`) returned from functions or passed to hooks/timers to fail with `Symbol's value as variable is void`. |
| **4. Definition Syntax (`defun` vs. `define`)** | Separate macros for functions vs. variables: `(defun foo (x y) "Doc." (+ x y))`, `(defvar *bar* 42 "Doc.")`, `(defconst +baz+ 99)`, and `(setq x 10)` for assignment. | Unified `define` for both values and procedures: `(define bar 42)` and `(define (foo x y) "Doc." (+ x y))` (shorthand for `(define foo (lambda (x y) ...))`). Mutation uses `(set! x 10)`. | Writing `(define (my-func x) ...)` in an Emacs `init.el` (which throws `(void-function define)`), or writing `(setq x 1)` in Guile Scheme instead of `(set! x 1)` (or better, lexical `let`). |
| **5. Tail-Call Optimization (TCO) & Iteration** | **No guaranteed general TCO** (only explicit self-tail-recursion via `named-let` in Emacs 28+). Deep recursion overflows `max-lisp-eval-depth`. Idiomatic Elisp uses imperative loops (`dolist`, `dotimes`, `while`, `cl-loop`) or `mapcar`/`seq-filter`. | **Mandatory proper Tail-Call Optimization** guaranteed by the language standard. Tail-recursive calls compile to direct jumps (`goto`) consuming $O(1)$ stack frames. Idiomatic Scheme uses named `let`, `fold`, `map`, and `filter`. | Translating a 50,000-iteration recursive Scheme loop directly into Elisp without `dolist`/`while`, hitting `Lisp nesting exceeds 'max-lisp-eval-depth'`. |

**[Script/Config]** Side-by-side Rosetta Stone demonstrating all five differences in runnable code:

```elisp
;;; elisp-vs-scheme-demo.el --- Emacs Lisp side -*- lexical-binding: t; -*-
;; 1. First line MUST enable lexical-binding so closures capture lexical env!

(defun demo-elisp-filter-map (pred-fn transform-fn items)
  "Filter ITEMS with PRED-FN and apply TRANSFORM-FN (Lisp-2: needs `funcall')."
  ;; In Elisp, `nil' IS the empty list `()'. An empty list is falsy!
  (if (null items) ; or simply `(if (not items) ...)'
      nil
    (let ((result nil))
      ;; Idiomatic Elisp uses `dolist' instead of deep recursion:
      (dolist (item items (nreverse result))
        ;; Lisp-2: `pred-fn' is in the value cell, so we MUST use `funcall'!
        (when (funcall pred-fn item)
          (push (funcall transform-fn item) result))))))

;; Calling in Elisp: sharp-quote `#'evenp' looks up the function cell
(demo-elisp-filter-map #'cl-evenp (lambda (x) (* x 10)) '(1 2 3 4 5 6))
;; => (20 40 60)
```

```scheme
;;; scheme-side-demo.scm --- GNU Guile Scheme side
(use-modules (srfi srfi-1))

(define (demo-scheme-filter-map pred-fn transform-fn items)
  "Filter ITEMS with PRED-FN and apply TRANSFORM-FN (Lisp-1: direct call)."
  ;; Named `let' compiles to a zero-allocation tail-recursive loop:
  (let loop ((rest items)
             (acc '()))
    ;; CRITICAL: In Scheme, `'()' is TRUTHY! Always test with `(null? rest)'.
    (if (null? rest)
        (reverse acc)
        (let ((head (car rest)))
          ;; Lisp-1: `pred-fn' is called directly in operator position!
          (if (pred-fn head)
              (loop (cdr rest) (cons (transform-fn head) acc))
              (loop (cdr rest) acc))))))

;; Calling in Guile Scheme: functions are ordinary values, no `#'` needed
(demo-scheme-filter-map even? (lambda (x) (* x 10)) '(1 2 3 4 5 6))
;; => (20 40 60)
```

---

## 4.3 Structural Editing: The Secret to Loving Parentheses

**[Concept]** Beginners hate Lisp parentheses because they edit Lisp as a sequence of raw characters—manually typing `(`, counting closing `)))` at the end of a form, and accidentally deleting a delimiter with Backspace. **Experienced Lisp engineers never edit characters; they edit the Abstract Syntax Tree (AST) directly.** With structural editing (`paredit-mode`, `smartparens-strict-mode`, or `evil-cleverparens-mode` for Evil/Vim users), parentheses are always inserted in balanced pairs, Backspace refuses to delete an unmatched paren, and you reshape code by expanding, shrinking, splicing, or raising entire S-expressions (`sexps`).

### The Complete Visual Guide to Structural Editing

In every diagram below, the vertical bar `|` represents **Point** (your cursor position) before and after the keystroke.

#### 1. Balanced Sexp Navigation (`C-M-f`, `C-M-b`, `C-M-d`, `C-M-u`)

Stop holding down arrow keys. Move across whole AST nodes in a single keystroke:

```
BEFORE:  |(service openssh-service-type (openssh-configuration (port-number 2222)))
PRESS:   C-M-f  (forward-sexp: jump across the entire balanced form!)
AFTER:   (service openssh-service-type (openssh-configuration (port-number 2222)))|

BEFORE:  |(service openssh-service-type (openssh-configuration (port-number 2222)))
PRESS:   C-M-d  (down-list: step INSIDE the opening parenthesis)
AFTER:   (|service openssh-service-type (openssh-configuration (port-number 2222)))

BEFORE:  (service openssh-service-type (openssh-configuration |(port-number 2222)))
PRESS:   C-M-u  (backward-up-list: climb OUT to the enclosing parent list)
AFTER:   (service openssh-service-type |(openssh-configuration (port-number 2222)))
```

#### 2. Forward Slurp (`C-)` or `C-<right>` in Paredit/Smartparens, `>` in `evil-cleverparens`)

**What it does**: Pulls the next sibling S-expression to the right *inside* the current enclosing list. Use this when you wrote `(cons* a b) c` and want `c` inside the `cons*` call.

```
BEFORE:  (cons* btrfs-progs |lvm2) git %base-packages
PRESS:   C-)   (paredit-forward-slurp-sexp / sp-forward-slurp-sexp)
AFTER:   (cons* btrfs-progs |lvm2 git) %base-packages
PRESS:   C-)   (again!)
AFTER:   (cons* btrfs-progs |lvm2 git %base-packages)
```

#### 3. Forward Barf (`C-}` or `C-<left>` in Paredit/Smartparens, `<` in `evil-cleverparens`)

**What it does**: Pushes the last element of the current list *out* to the right (the exact inverse of Forward Slurp).

```
BEFORE:  (cons* btrfs-progs |lvm2 git %base-packages)
PRESS:   C-}   (paredit-forward-barf-sexp / sp-forward-barf-sexp)
AFTER:   (cons* btrfs-progs |lvm2 git) %base-packages
```

#### 4. Wrap Round (`M-(`) & Splice (`M-s`)

**What it does**: `M-(` wraps the S-expression at point in a fresh pair of parentheses and places point at the head so you can type the function name. `M-s` dissolves the enclosing parentheses around point, leaving all inner elements in place.

```
--- WRAP (M-() ---
BEFORE:  (packages |(cons* git vim %base-packages))
PRESS:   M-(   (paredit-wrap-round / sp-wrap-round)
AFTER:   (packages (|(cons* git vim %base-packages)))

--- SPLICE (M-s) ---
BEFORE:  (let ((x 1)) |(begin (display x) (newline)))
PRESS:   M-s   (paredit-splice-sexp / sp-splice-sexp: dissolves `(begin ...)'!)
AFTER:   (let ((x 1)) |(display x) (newline))
```

#### 5. Raise S-Expression (`M-r`) — The Ultimate Refactoring Move

**What it does**: Replaces the **entire enclosing parent form** with the single S-expression at point, deleting the parent and all siblings! Suppose you wrapped a package in a temporary transformation or conditional `(if debug? |(list strace gdb) '())` and now just want `(list strace gdb)` to replace the whole `if` form:

```
BEFORE:  (append |(list btrfs-progs lvm2) (if use-debug? (list strace) '()))
         ;; Wait: let's say point is on `btrfs-progs' inside `(if test |btrfs-progs vim)':
BEFORE:  (cons* (if (host-is-laptop?) |tlp-package powertop) %base-packages)
PRESS:   M-r   (paredit-raise-sexp / sp-raise-sexp)
AFTER:   (cons* |tlp-package %base-packages)
```

#### 6. Kill Balanced Sexp (`C-M-k` or `C-k` in Paredit)

**What it does**: `C-M-k` (`kill-sexp`) cuts the entire balanced S-expression after point—whether it is a single symbol or a 60-line `(operating-system ...)` record—onto the kill ring without ever leaving a dangling parenthesis.

```
BEFORE:  (services (cons* |(service openssh-service-type
                                    (openssh-configuration
                                     (permit-root-login 'prohibit-password)))
                          %base-services))
PRESS:   C-M-k (kill-sexp)
AFTER:   (services (cons* |
                          %base-services))
```

---

## 4.4 Geiser, Interactive Guile REPL & Live Guix System Inspection

**[Concept]** Static edit-compile-run cycles belong to C and Go. When engineering Guix packages, G-expressions, or system services in Emacs, your `.scm` buffer is wired over a socket or pipe to a live Guile process via **Geiser** (`geiser` + `geiser-guile`). You evaluate definitions in place (`C-M-x`), inspect record types, expand hygienic macros (`#%`, `#~`, `modify-services`) inline, and even attach Emacs to `guix repl` so the full `(gnu ...)` and `(guix ...)` package graph is live in memory.

### Essential Geiser Workflow & Keybindings

| Key Chord | Geiser Command | What Happens Under the Hood |
| :--- | :--- | :--- |
| `M-x run-guile` (or `C-c C-z`) | `run-guile` / `geiser-mode-switch-to-repl` | Starts a background Guile subprocess (if not already running) and switches between your `.scm` buffer and `*Guile REPL*`. |
| `C-M-x` | `geiser-eval-definition` | Sends the top-level `(define ...)` or `(define-public ...)` enclosing point to the Guile REPL, compiles it into the current module, and flashes the result in the echo area. |
| `C-x C-e` | `geiser-eval-last-sexp` | Evaluates the S-expression immediately preceding point and prints `$N = <value>` in the echo area. |
| `C-c C-b` | `geiser-eval-buffer` | Evaluates and loads the entire buffer into the running Guile image. |
| `C-c C-d C-d` | `geiser-doc-symbol-at-point` | Opens a dedicated help buffer displaying the procedure signature, docstring, and module of the symbol at point. |
| `C-c C-d C-i` | `geiser-doc-look-up-manual` | Jumps directly to the official GNU Guile or GNU Guix Info manual node for the symbol at point! |
| `M-.` / `M-,` | `xref-find-definitions` / `xref-go-back` | Jumps directly to the source file and line where the Guix package, service, or Guile procedure at point is defined, then pops back. |
| `C-c C-m C-e` | `geiser-expand-last-sexp` | Runs `(macroexpand ...)` in Guile on the form before point—indispensable for debugging `syntax-rules`, `modify-services`, or record constructors. |

### Connecting Geiser to `guix repl` (The Superpower Workflow)

A plain `guile` process does not have the Guix channel modules (`(gnu packages ...)`, `(gnu services ...)`, `(guix gexp)`) on its `%load-path` unless started through `guix repl`. There are two ways to run Geiser with full Guix awareness:

1. **Method A: Local Unix Domain Socket or TCP Listener (Recommended for System/Package Dev)**
   In a terminal (or Emacs `M-x shell` / `eat`), launch `guix repl` listening on a local socket or localhost TCP port:
   ```bash
   # Option 1: Listen on localhost TCP port 37146
   guix repl --listen=tcp:37146

   # Option 2: Listen on a Unix domain socket (safer on multi-user hosts)
   guix repl --listen=unix:/tmp/guix-repl.sock
   ```
   Then inside Emacs:
   - For TCP: Run `M-x geiser-connect`, select `guile`, host `localhost`, port `37146`.
   - For Unix socket: Run `M-x geiser-connect-local`, select `guile`, and enter `/tmp/guix-repl.sock`.

2. **Method B: Inspecting Live Packages and Derivations in the Connected REPL**
   Once connected to `guix repl`, open any `.scm` file and evaluate forms with `C-x C-e`:
   ```scheme
   ;; Evaluate each form below with C-x C-e inside your Geiser-connected buffer:
   ,use (gnu packages base)
   ,use (guix packages)
   ,use (guix store)
   ,use (guix derivations)

   ;; 1. Inspect a package record at runtime:
   (package-Full-name glibc) ; or:
   (list (package-name glibc)
         (package-version glibc)
         (package-home-page glibc))
   ;; => ("glibc" "2.39" "https://www.gnu.org/software/libc/")

   ;; 2. Compute the exact /gnu/store derivation path of a package without building it:
   (with-store store
     (derivation-file-name (package-derivation store hello)))
   ;; => "/gnu/store/8z9...-hello-2.12.1.drv"
   ```

> **Pro Trap — The `(ice-9 readline)` Geiser Hang**: Never load `(use-modules (ice-9 readline))` unconditionally inside `~/.guile`! Geiser communicates with Guile over an asynchronous pipe/socket prompt protocol; activating GNU Readline inside a non-TTY subprocess scrambles prompt detection and hangs Emacs when opening `.scm` files. Always set `(setq geiser-guile-load-init-file nil)` in your Emacs configuration, or guard `~/.guile` with `(when (isatty? (current-input-port)) ...)`.

---

## 4.5 `guix.el` (`emacs-guix`), `guix-devel-mode` & Magit

**[Concept]** Because GNU Guix is written in Scheme and Emacs is a Lisp machine, the **`emacs-guix`** package (`guix.el`) turns Emacs into a graphical control plane for the `/gnu/store` daemon, system generations, and package definitions, while **Magit** manages your channel repositories and declarative config commits.

### High-Leverage `emacs-guix` Commands

| Command | What It Gives You Inside Emacs | Interactive Keys in the Buffer |
| :--- | :--- | :--- |
| `M-x guix` | Opens the master `emacs-guix` popup dashboard (Transient menu) for packages, profiles, generations, services, and store inspection. | `p` (packages), `g` (generations), `s` (services), `S` (store items) |
| `M-x guix-packages-by-name` | Interactive table of Guix packages matching your query. Press `RET` on any row to view its inputs, outputs, license, location in the channel tree, and store path. | `i` (mark install), `d` (mark delete), `u` (mark upgrade), `x` (execute marks), `e` (jump directly to the `.scm` package definition!) |
| `M-x guix-generations` | Lists all profile generations with timestamps and package counts. Compare any two generations with `D` (`guix-generations-diff`) or switch/delete generations. | `RET` (list packages in generation), `D` (diff two generations), `s` (switch to generation), `d` (mark for deletion) |
| `M-x guix-system-generations` | Same as `guix-generations`, but inspects `/var/guix/profiles/system-*-link` OS generations—showing kernel, bootloader, and Shepherd service diffs! | `RET` (inspect OS generation details), `D` (diff two OS generations) |
| `M-x guix-store-live-items` | Queries `guix-daemon` for live `/gnu/store` items, showing referrers (what keeps an item alive), requisites (its closure), and derivers (`.drv` files). | `r` (show referrers), `R` (show requisites), `d` (show derivers) |
| `M-x guix-devel-mode` | Minor mode for buffers visiting Guix channel `.scm` files. Automatically sets the Geiser REPL module to match the file's `(define-module ...)` header! | `C-c . b` (`guix-devel-build-package-definition` — builds the package at point!), `C-c . l` (`guix-devel-lint-package`), `C-c . u` (`guix-devel-download-package-source` — fetches tarball and inserts new `base32` hash!) |

### Complete, Production-Ready Emacs Configuration for Guile, Guix & Structural Editing

**[Script/Config]** Save or tangle this self-contained configuration to `~/.config/emacs/lisp/init-guix-ide.el` (or load it directly from `~/.config/emacs/init.el`). Every line is real, byte-compilable Emacs Lisp with `lexical-binding: t`.

```elisp
;;; init-guix-ide.el --- Complete Guile, Guix & Structural Lisp IDE -*- lexical-binding: t; -*-
;;; Commentary:
;; Provides:
;;   1. Balanced structural editing via `paredit' + `rainbow-delimiters'
;;   2. Interactive Guile Scheme evaluation via `geiser' + `geiser-guile'
;;   3. Inline macro expansion via `macrostep-geiser'
;;   4. On-the-fly bytecode compiler warnings via `flycheck-guile' (`guild compile')
;;   5. Native Guix package/generation/store management via `guix' (`emacs-guix')

;;; Code:

(require 'use-package)

;;;; 1. Locate Guile and Guix binaries reliably --------------------------------
(defconst my/guile-binary
  (or (executable-find "guile")
      "/run/current-system/profile/bin/guile"
      "/var/guix/profiles/per-user/root/current-guix/bin/guile"
      "guile")
  "Resolved path to the GNU Guile executable.")

;;;; 2. Scheme Major Mode ------------------------------------------------------
(use-package scheme
  :ensure nil
  :mode (("\\.scm\\'"   . scheme-mode)
         ("\\.sld\\'"   . scheme-mode)
         ("\\.guile\\'" . scheme-mode)
         ("\\.drv\\'"   . scheme-mode)))

;;;; 3. Structural Editing: Paredit + Rainbow Delimiters -----------------------
(use-package paredit
  :hook ((emacs-lisp-mode
          eval-expression-minibuffer-setup
          ielm-mode
          lisp-mode
          lisp-interaction-mode
          scheme-mode
          geiser-repl-mode) . enable-paredit-mode)
  :bind (:map paredit-mode-map
              ;; Keep M-s for search prefix if desired, or bind splice to M-S:
              ("C-)" . paredit-forward-slurp-sexp)
              ("C-}" . paredit-forward-barf-sexp)
              ("C-(" . paredit-backward-slurp-sexp)
              ("C-{" . paredit-backward-barf-sexp)
              ("M-r" . paredit-raise-sexp)
              ("M-(" . paredit-wrap-round)))

(use-package rainbow-delimiters
  :hook ((prog-mode geiser-repl-mode) . rainbow-delimiters-mode))

;; Highlight matching parentheses immediately without delay:
(use-package paren
  :ensure nil
  :custom
  (show-paren-delay 0.0)
  (show-paren-style 'parenthesis)
  (show-paren-context-when-offscreen 'overlay)
  :config
  (show-paren-mode 1))

;;;; 4. Geiser + Geiser-Guile (Live REPL) --------------------------------------
(use-package geiser
  :custom
  (geiser-default-implementation 'guile)
  (geiser-active-implementations '(guile))
  (geiser-mode-start-repl-p t)           ; auto-start REPL when opening .scm
  (geiser-repl-query-on-kill-p nil)      ; don't nag on exit
  (geiser-repl-use-other-window t)
  (geiser-repl-history-size 5000)
  (geiser-mode-autodoc-p t)              ; live procedure arglists in echo area
  (geiser-repl-add-project-paths t))

(use-package geiser-guile
  :after geiser
  :custom
  (geiser-guile-binary my/guile-binary)
  ;; CRITICAL: Do not load ~/.guile in Geiser subprocesses!
  ;; If ~/.guile activates (ice-9 readline), Geiser's prompt parser hangs.
  (geiser-guile-load-init-file nil)
  (geiser-guile-manual-lookup-other-window t)
  :config
  ;; Automatically add ~/.config/guix/current to Geiser's load path if present
  ;; so (gnu ...) and (guix ...) modules resolve even in plain `run-guile':
  (let ((guix-Load-dir (expand-file-name "~/.config/guix/current/share/guile/site/3.0"))
        (guix-go-dir   (expand-file-name "~/.config/guix/current/lib/guile/3.0/site-ccache")))
    (when (file-directory-p guix-Load-dir)
      (add-to-list 'geiser-guile-load-path guix-Load-dir))
    (when (file-directory-p guix-go-dir)
      (setenv "GUILE_LOAD_COMPILED_PATH"
              (concat guix-go-dir ":" (or (getenv "GUILE_LOAD_COMPILED_PATH") ""))))))

(use-package macrostep-geiser
  :after geiser
  :hook ((geiser-mode geiser-repl-mode) . macrostep-geiser-setup)
  :bind (:map geiser-mode-map
              ("C-c C-m x" . macrostep-expand)))

;;;; 5. On-the-fly Static Analysis with `guild compile' ------------------------
(use-package flycheck
  :hook (scheme-mode . flycheck-mode)
  :custom
  (flycheck-display-errors-delay 0.25))

(use-package flycheck-guile
  :after (flycheck geiser-guile)
  :demand t)

;;;; 6. Native GNU Guix Integration (`emacs-guix') -----------------------------
(use-package guix
  :commands (guix
             guix-packages-by-name
             guix-generations
             guix-system-generations
             guix-store-live-items
             guix-devel-mode)
  :hook (scheme-mode . guix-devel-mode)
  :bind (("C-c g g" . guix)
         ("C-c g p" . guix-packages-by-name)
         ("C-c g s" . guix-system-generations)
         ("C-c g l" . guix-store-live-items)))

;;;; 7. Magit: Git Porcelain for Channels & Config Repos -----------------------
(use-package magit
  :bind (("C-x g" . magit-status)
         ("C-x M-g" . magit-dispatch))
  :custom
  (magit-diff-refine-hunk 'all)          ; word-level diff highlighting
  (magit-save-repository-buffers 'dontask))

(provide 'init-guix-ide)
;;; init-guix-ide.el ends here
```

**[Commands/Exercise]** Hands-On Lab 4: Structural Refactoring & Live `guix repl` Inspection
1. Start a local Guix REPL server in your terminal:
   `guix repl --listen=unix:/tmp/guix-lab.sock`
2. Open `/tmp/lab4.scm` in Emacs (`C-x C-f /tmp/lab4.scm RET`) and connect Geiser with `M-x geiser-connect-local RET guile RET /tmp/guix-lab.sock RET`.
3. Insert the following broken S-expression:
   ```scheme
   (use-modules (gnu) (gnu packages base) (guix packages))
   (define my-pkgs (list hello) coreutils grep)
   ```
4. Place your cursor on `hello` inside `(list hello)`, press `C-)` (`paredit-forward-slurp-sexp`) **twice** so `coreutils` and `grep` are pulled inside `(list hello coreutils grep)`.
5. Press `C-M-x` (`geiser-eval-definition`) to compile `my-pkgs` into the live REPL, then type `(map package-full-name my-pkgs)` and press `C-x C-e` after the closing paren to verify the evaluated output `("hello-2.12.1" "coreutils-9.1" "grep-3.11")` in the echo area.


---

# Module 5: Org-Mode & Literate Systems Engineering (From Zero to Tangled OS)

## 5.1 Org-Mode from Scratch: Outlines, Structural Motion, Lists, Links & Live Tables

**[Concept]** Plain Markdown is passive presentation syntax: you can render it to HTML, but it has no computable AST, no spreadsheet engine, no queryable task database, and no native code-extraction (tangling) pipeline. **GNU Emacs Org-mode (`.org`) is a structured plain-text AST** (`org-element`) that combines hierarchical outlining, GTD project management, a live spreadsheet (`org-table`), an atomic Zettelkasten graph (`org-roam`), and an executable polyglot notebook and system configuration generator (`org-babel`).

### 1. Outlines, Visibility Cycling & Structural Tree Editing

An Org document is a tree of headings prefixed by one or more asterisks (`*` at column 0):

```org
#+title: Workstation Engineering Notebook
#+author: Omran
#+startup: content

* Level 1 Heading: Storage Architecture
** Level 2 Heading: LUKS2 & Btrfs Subvolumes
*** Level 3 Heading: Mount Options & Compression
```

Never copy-paste headings manually to reorganize a document. Org treats every heading and all text/children beneath it as a **Subtree** that moves and folds as a single atomic unit:

```
+-------------------------------------------------------------------------+
| VISIBILITY CYCLING STATE MACHINE                                        |
|                                                                         |
| 1. Local Subtree Cycling (Press `TAB` on a heading):                    |
|    [FOLDED: * Heading...] ---> [CHILDREN: ** Child 1] ---> [SUBTREE]    |
|            ^                                                   |        |
|            +---------------------------------------------------+        |
|                                                                         |
| 2. Global Buffer Cycling (Press `S-TAB` anywhere):                      |
|    [OVERVIEW: Top-level * only] -> [CONTENTS: All headings] -> [SHOW ALL]|
+-------------------------------------------------------------------------+
```

| Key Chord | Org Command | Tree Operation Performed |
| :--- | :--- | :--- |
| `TAB` | `org-cycle` | Cycles visibility of the **current subtree** (`FOLDED` $\rightarrow$ `CHILDREN` $\rightarrow$ `SUBTREE`). |
| `S-TAB` | `org-shifttab` | Cycles visibility of the **entire buffer** (`OVERVIEW` $\rightarrow$ `CONTENTS` $\rightarrow$ `SHOW ALL`). |
| `M-RET` | `org-meta-return` | Inserts a new heading (or plain list item) at the **same level** as the current line. |
| `C-RET` | `org-insert-heading-respect-content` | Inserts a new same-level heading at the **very end** of the current subtree without splitting its body text. |
| `M-UP` / `M-DOWN` | `org-metaup` / `org-metadown` | Swaps the **entire current subtree** (heading + body + children) with the sibling subtree above or below it! |
| `M-LEFT` / `M-RIGHT` | `org-metaleft` / `org-metaright` | Promotes (`**` $\rightarrow$ `*`) or demotes (`*` $\rightarrow$ `**`) the **single heading** at point. |
| `M-S-LEFT` / `M-S-RIGHT` | `org-shiftmetaleft` / `org-shiftmetaright` | Promotes or demotes the **entire subtree** (heading + all descendant headings). |
| `C-x n s` / `C-x n w` | `org-narrow-to-subtree` / `widen` | Narrows the Emacs buffer view exclusively to the current subtree so you can focus with zero distraction; `C-x n w` restores the full buffer. |

### 2. Plain Lists, Checkboxes & Progress Cookies (`[/]` and `[%]`)

Unordered lists start with `-` or `+`; ordered lists start with `1.` or `1)`. Adding `[ ]` right after the bullet turns the item into a three-state checkbox (`[ ]` unchecked, `[-]` partial/in-progress, `[X]` complete). Placing `[/]` or `[%]` in the parent heading or parent list item creates a live progress cookie that updates automatically every time you press `C-c C-c` on a child checkbox!

```org
* Pre-Reconfigure Verification Checklist [2/3] [66%]
- [X] Run =guix time-machine -C channels.scm -- system build config.scm=
- [X] Verify EFI System Partition is mounted at =/boot/efi= (not =/boot=)
- [-] Inspect Shepherd service graph [1/2]
  - [X] Check =guix system shepherd-graph config.scm=
  - [ ] Verify =sshd= and =elogind= dependencies
```
- **Keys**: `M-S-RET` (`org-insert-todo-heading`) inserts a new `- [ ]` checkbox item; `C-c C-c` toggles `[ ]` $\leftrightarrow$ `[X]` and recomputes `[/]` and `[%]` upward through the tree; `C-u C-c C-c` sets the intermediate `[-]` state.

### 3. Hyperlinks (`[[target][description]]`)

Org links unify web URLs, local files with exact line numbers or search strings, Info manual nodes, and immutable UUIDs:
- **Syntax**: `[[TARGET][DESCRIPTION]]` (or `[[TARGET]]` with no description).
- **Essential Link Types for Guix Engineers**:
  - `[[https://guix.gnu.org/manual/en/html_node/index.html][GNU Guix Reference Manual]]`
  - `[[file:/etc/config.scm::42][ Jump to line 42 of /etc/config.scm ]]`
  - `[[file:~/.config/guix/home.scm::(define home-packages][Jump to home-packages definition]]`
  - `[[info:guix#Operating System Reference][Open Guix Info Manual directly inside Emacs]]`
  - `[[id:6f8e4a12-9b30-4d1c-811e-77a910c3e201][Atomic Org-Roam Note Link]]`
- **Keys**:
  - `C-c l` (`org-store-link`): Remembers a link to the exact file/line/heading/commit where your cursor currently sits.
  - `C-c C-l` (`org-insert-link`): Prompts for (or selects from stored links) a target and description, or edits the existing link at point.
  - `C-c C-o` (`org-open-at-point`, or simply `RET` when `org-return-follows-link` is `t`): Opens the link in Emacs or your browser.

### 4. Tables & The Live Spreadsheet Engine (`#+TBLFM:`)

Type `| Component | RAM (MiB) | Notes |` and press `TAB`—Org immediately aligns the columns. Press `| - TAB` on a new row to expand a horizontal separator rule (`|---+---+---|`). Better still, every Org table is backed by Emacs Calc and Emacs Lisp via `#+TBLFM:` (Table Formulas):

```org
#+NAME: tbl-partition-budget
| Subvolume / Mount | Quota (GiB) | Quota (MiB) | Purpose                           |
|-------------------+-------------+-------------+-----------------------------------|
| /gnu/store        |          80 |       81920 | Immutable content-addressed store |
| /var/guix         |           5 |        5120 | Daemon SQLite DB & GC roots       |
| /home             |         120 |      122880 | User profiles & project worktrees |
| /tmp              |           8 |        8192 | In-RAM tmpfs build scratchpad     |
|-------------------+-------------+-------------+-----------------------------------|
| TOTAL             |         213 |      218112 | Verified allocation               |
#+TBLFM: $3=$2*1024::@6$2=vsum(@I..@II)::@6$3=vsum(@I..@II)
```

**Understanding `#+TBLFM:` Coordinates**:
- `$1`, `$2`, `$3`: Column 1, Column 2, Column 3 of the current row.
- `@2$3`: Row 2, Column 3 (counting data rows from the top; horizontal separator lines `|---+---|` are skipped in `@N` numbering!).
- `@I` and `@II`: The **first** and **second** horizontal separator lines (`|---+---|`). Thus `@I..@II` means *"all data rows between the header rule and the totals rule"*—so you can insert new rows in the middle without breaking your sum formula!
- `::` separates multiple formulas on a single `#+TBLFM:` line.
- **Emacs Lisp in Table Formulas**: Prefix a formula with `'(...)` to evaluate arbitrary Elisp:
  `#+TBLFM: $3='(* (string-to-number $2) 1024)`
- **Keys**: Press `C-c *` (`org-table-recalculate`) on a row, or `C-u C-c *` (or `C-c C-c` directly on the `#+TBLFM:` line) to recompute the entire spreadsheet!

---

## 5.2 Task & Project Management: States, Schedules, Properties & `org-capture`

**[Concept]** Systems engineering requires tracking multi-stage migrations, channel upgrades, and hardware diagnostics right alongside your documentation. Org-mode turns any heading into a state-machine task item indexed across your entire `org-agenda-files` list.

### 1. The GTD State Machine (`TODO` $\rightarrow$ `NEXT` $\rightarrow$ `WAIT` $\rightarrow$ `DONE`)

In our `init-org.el`, we configure two parallel state sequences:
```elisp
(setq org-todo-keywords
      '((sequence "TODO(t)" "NEXT(n)" "WAIT(w@/!)" "|" "DONE(d!)" "CANCELLED(c@)")
        (sequence "PROJ(p)" "|" "COMPLETED(C!)"))
      org-log-done 'time
      org-log-into-drawer t)
```
- The vertical bar `"|"` separates **incomplete** states (which show up as active work in your Agenda) from **terminal/completed** states.
- `(w@/!)` means: when entering `WAIT` (`w`), prompt for a note explaining *who/what* you are waiting on (`@`), and when leaving `WAIT`, log a timestamp (`!`) inside the `:LOGBOOK:` drawer.
- **Key**: Press `C-c C-t` (`org-todo`) on any heading followed by `t`, `n`, `w`, `d`, or `c` to transition state.

### 2. `SCHEDULED:`, `DEADLINE:`, Tags & Properties Drawers

```org
* PROJ Migrate Workstation to Pure GNU Guix System              :@computer:guix:
:PROPERTIES:
:ID:         8a31f422-1c4b-4e9a-b820-9f12c304e550
:CREATED:    [2026-11-14 Sat 09:00]
:COOKIE_DATA: todo recursive
:END:
** NEXT Pin channels.scm and verify guix system build
SCHEDULED: <2026-11-14 Sat> DEADLINE: <2026-11-16 Mon>
:PROPERTIES:
:EFFORT:   1:30
:END:
:LOGBOOK:
- State "NEXT"       from "TODO"       [2026-11-14 Sat 09:15]
:END:
```
- **`SCHEDULED:` (`C-c C-s`)**: The date you plan to **start working** on the task.
- **`DEADLINE:` (`C-c C-d`)**: The date the task **must be finished** (warns you in the agenda `org-deadline-warning-days` in advance).
- **Active `<...>` vs. Inactive `[...]` Timestamps**:
  - `<2026-11-14 Sat>` (**Active**, inserted with `C-c .`): Appears on your daily/weekly `M-x org-agenda` calendar.
  - `[2026-11-14 Sat]` (**Inactive**, inserted with `C-c !`): Records when a note or state change occurred *without* cluttering your agenda calendar.
- **Tags (`C-c C-q`)**: Context labels (`:@computer:`, `:guix:`) aligned on the right of the heading. Inherited down the subtree!
- **Properties Drawer (`:PROPERTIES:` ... `:END:`, set via `C-c C-x p`)**: Key-value metadata attached to a subtree (`:ID:` for `org-roam`, `:EFFORT:` for time estimates, `:header-args:` for Org-Babel execution/tangle defaults).

### 3. Frictionless Capture (`C-c c` / `org-capture`) & Refile (`C-c C-w` / `org-refile`)

When you are deep inside a Scheme debugging session and spot a latent bug in a Shepherd service, never break flow by hunting for a notes file. Press `C-c c` (`org-capture`), hit `l` ("Todo with link to here"), type a one-line description, and press `C-c C-c`. Org automatically records a task in `~/org/inbox.org` containing a clickable `[[file:...]]` link back to the exact line of `.scm` code you were editing, then restores your window layout! Later, in `inbox.org`, press `C-c C-w` (`org-refile`) to move the subtree into the appropriate project in `projects.org`.

---

## 5.3 Org-Babel (`ob-scheme`, `ob-shell`) & Literate Systems Programming

**[Concept]** Donald Knuth coined *Literate Programming* in 1984: instead of writing source code with occasional comments for humans, you write an explanatory engineering narrative for humans that contains embedded source code blocks. From that single literate `.org` document, Org-Babel performs two orthogonal operations:
1. **Weaving / Interactive Execution (`C-c C-c`)**: Running code blocks live in the document, passing outputs from one block (e.g., `sh`) as inputs to another block (e.g., `scheme` or `python`), and rendering tables or results inline.
2. **Tangling (`C-c C-v t` / `org-babel-tangle`)**: Extracting and assembling code blocks (resolving `<<noweb>>` macro references) into their target system paths (`/etc/config.scm`, `~/.config/guix/channels.scm`, `~/.config/guix/home.scm`).

### 1. `:results value` vs. `:results output` & Persistent `:session`

Every newcomer to Org-Babel trips over `:results value` vs. `:results output`. Memorize the distinction:

| Header Argument | What Org-Babel Captures | When to Use It |
| :--- | :--- | :--- |
| `:results value` *(Default!)* | Wraps your block in a function, executes it, and captures the **return value of the very last expression** in the block. Anything printed to `stdout` (`display`, `format #t`, `echo`) is ignored! | Pure functional calculations in Guile Scheme, returning lists/alists that Org automatically formats into an Org Table! |
| `:results output` | Captures everything written to **standard output (`stdout`)** during block execution and ignores the final return value. | Shell scripts (`sh`/`bash`), CLI tools (`guix describe`), or Scheme blocks that use `(display ...)` / `(format #t ...)`. |
| `:session *guile*` | Runs the block inside a persistent named Emacs REPL buffer (`*guile*`) instead of spawning a fresh subprocess that dies after the block finishes. | Defining helper procedures or importing `(guix ...)` modules in one block and reusing them in later blocks! |

```org
#+begin_src scheme :session *guix-calc* :results value table :colnames '("Package" "Outputs" "Synopsis")
(use-modules (gnu packages base)
             (gnu packages version-control)
             (gnu packages admin)
             (guix packages))

(map (lambda (pkg)
       (list (package-full-name pkg)
             (string-join (package-outputs pkg) ",")
             (package-synopsis pkg)))
     (list glibc coreutils git shepherd))
#+end_src

#+RESULTS:
| Package        | Outputs          | Synopsis                                     |
|----------------+------------------+----------------------------------------------|
| glibc-2.39     | debug,out,static | The GNU C Library                            |
| coreutils-9.1  | debug,out        | Core GNU utilities (file, text, shell)       |
| git-2.45.2     | gui,out,send-email,svn | Distributed version control system     |
| shepherd-0.10.4| out              | System service manager                       |
```

### 2. Polyglot Data Pipelines with `:var` (Bash $\rightarrow$ Python $\rightarrow$ Guile Scheme)

Org-Babel can pass the output of any block (or any `#+NAME:` Org table!) into a block written in a **completely different language** via `:var name=source-block-name`. Org automatically serializes 2D tables into native lists-of-lists in Python (`[[...], [...]]`) and Guile Scheme (`'(("..." ...) ...)`):

```org
#+NAME: sh-disk-usage
#+begin_src sh :results value table
df -Pm / /tmp | awk 'NR>1 {print $6, $2, $3, $4}'
#+end_src

#+RESULTS: sh-disk-usage
| /    | 102400 | 41200 | 61200 |
| /tmp |   8192 |   128 |  8064 |

#+NAME: py-normalize-mounts
#+begin_src python :var rows=sh-disk-usage :results value
# Python receives `rows` from the Bash block as a native list of lists!
return [
    [mount, int(total_mb), round((int(used_mb) / int(total_mb)) * 100.0, 1)]
    for mount, total_mb, used_mb, _free_mb in rows
]
#+end_src

#+RESULTS: py-normalize-mounts
| /    | 102400 | 40.2 |
| /tmp |   8192 |  1.6 |

#+begin_src scheme :var metrics=py-normalize-mounts :results output
;; Guile Scheme receives `metrics` from Python as a quoted list of lists!
(use-modules (ice-9 format))
(for-each
 (lambda (row)
   (let ((mount (car row))
         (total (cadr row))
         (pct   (caddr row)))
     (format #t "Mount ~a (~d MiB): ~,1f% used -> Status: ~a~%"
             mount total pct (if (> pct 85.0) "CRITICAL" "HEALTHY"))))
 metrics)
#+end_src

#+RESULTS:
: Mount / (102400 MiB): 40.2% used -> Status: HEALTHY
: Mount /tmp (8192 MiB): 1.6% used -> Status: HEALTHY
```

---

## 5.4 Tangling (`C-c C-v t`) & Complete Working `guix-workstation.org`

**[Concept]** Instead of scattering system state across `/etc/config.scm`, `~/.config/guix/channels.scm`, `~/.config/guix/home.scm`, and dozens of dotfiles, you maintain **one declarative Org document** (`guix-workstation.org`). Using `:tangle <path>`, `:mkdirp yes` (automatically create parent directories), and `:noweb yes` (modular composition via `<<block-name>>`), pressing `C-c C-v t` (`org-babel-tangle`) generates the entire operating system and home environment.

```
                      +-------------------------------+
                      |     guix-workstation.org      |
                      |   (Single Source of Truth)    |
                      +---------------+---------------+
                                      |
                        C-c C-v t  OR  make tangle
                                      |
        +-----------------------------+-----------------------------+
        |                             |                             |
        v                             v                             v
+-----------------------+   +-----------------------+   +-----------------------+
| ~/.config/guix/       |   | /etc/config.scm       |   | ~/.config/guix/       |
|   channels.scm        |   | (Noweb-assembled OS)  |   |   home.scm            |
+-----------+-----------+   +-----------+-----------+   +-----------+-----------+
            |                           |                           |
            v                           v                           v
      guix pull /               sudo guix system            guix home
   guix time-machine               reconfigure             reconfigure
```

### Complete, Self-Contained `guix-workstation.org` (Pure FSDG GNU Guix)

**[Script/Config]** Save the following complete literate document to `~/guix-workstation.org` (or `/tmp/guix-course-assets/guix-workstation.org`) and press `C-c C-v t` inside Emacs (or run `emacs --batch -Q --eval "(require 'ob-tangle)" --eval '(org-babel-tangle-file "guix-workstation.org")'`). Notice how `:noweb yes` assembles `/etc/config.scm` and `~/.config/guix/home.scm` cleanly from modular, documented sub-blocks!

````org
#+title: Declarative GNU Guix Workstation (System, Channels & Home)
#+author: Omran
#+startup: overview
#+property: header-args :mkdirp yes :comments no

* 1. Channel Pinning (~/.config/guix/channels.scm)
To guarantee bit-for-bit reproducibility across machines, we pin the official
FSDG-compliant =guix= channel to an exact Git commit and OpenPGP introduction.

#+begin_src scheme :tangle ~/.config/guix/channels.scm :tangle-mode (identity #o644)
;; Generated from guix-workstation.org — do not edit by hand.
(list (channel
        (name 'guix)
        (url "https://git.guix.gnu.org/guix.git")
        (branch "master")
        (commit
          "230aa373f315f247852ee07dff34146e9b480aec")
        (introduction
          (make-channel-introduction
            "9edb3f66fd807b096b48283debdcddccfea34bad"
            (openpgp-fingerprint
              "BBB0 2DDF 2CEA F6A8 0D1D  E643 A2A0 6DF2 A33A 54FA")))))
#+end_src

* 2. Operating System Components (Noweb Building Blocks)
Instead of writing a monolithic 300-line =/etc/config.scm=, we define each
subsystem in its own named block (=#+NAME:=) and compose them via =:noweb yes=.

** 2.1 Storage: LUKS2 + Btrfs Subvolumes + EFI at /boot/efi
Note: Always mount the EFI System Partition at =/boot/efi= (not =/boot=) on
LVM/LUKS setups so GRUB automatically includes its storage modules (Guix bug#44877).

#+NAME: os-storage-spec
#+begin_src scheme :tangle no
(define %crypt-root
  (mapped-device
    (source (uuid "11111111-2222-3333-4444-555555555555"))
    (target "cryptroot")
    (type luks-device-mapping)))

(define (btrfs-subvol subvol-name mount-target)
  (file-system
    (device "/dev/mapper/cryptroot")
    (mount-point mount-target)
    (type "btrfs")
    (flags '(no-atime))
    (options (string-append "compress=zstd:3,ssd,subvol=" subvol-name))
    (needed-for-boot? (member mount-target '("/" "/gnu/store")))
    (dependencies (list %crypt-root))))

(define %workstation-file-systems
  (cons* (btrfs-subvol "@root"  "/")
         (btrfs-subvol "@store" "/gnu/store")
         (btrfs-subvol "@home"  "/home")
         (file-system
           (device (file-system-label "ESP"))
           (mount-point "/boot/efi")
           (type "vfat")
           (options "umask=0077"))
         (file-system
           (device "none")
           (mount-point "/tmp")
           (type "tmpfs")
           (flags '(no-suid no-dev))
           (options "size=8G,mode=1777")
           (check? #f))
         %base-file-systems))
#+end_src

** 2.2 System Services: SSH, Networking, Zram & Periodic Btrfs Scrub
We use a G-expression (=#~=) to schedule a weekly Btrfs scrub via =mcron=,
referencing the exact =btrfs-progs= store path via =#$(file-append ...)=.

#+NAME: os-services-spec
#+begin_src scheme :tangle no
(define %weekly-btrfs-scrub
  #~(job "0 3 * * 1"
         (string-append #$(file-append btrfs-progs "/bin/btrfs")
                        " scrub start -B /")))

(define %workstation-services
  (cons* (service openssh-service-type
                  (openssh-configuration
                    (permit-root-login 'prohibit-password)
                    (password-authentication? #f)))
         (service zram-device-service-type
                  (zram-device-configuration
                    (size "8G")
                    (compression-algorithm 'zstd)
                    (priority 10)))
         (service fstrim-service-type)
         (simple-service 'workstation-btrfs-scrub
                         mcron-service-type
                         (list %weekly-btrfs-scrub))
         (simple-service 'workstation-sysctl
                         sysctl-service-type
                         '(("vm.swappiness" . "100")
                           ("vm.vfs_cache_pressure" . "50")))
         %desktop-services))
#+end_src

** 2.3 Assembled System Configuration (/etc/config.scm)
This block pulls in =<<os-storage-spec>>= and =<<os-services-spec>>= at tangle time!

#+begin_src scheme :noweb yes :tangle /etc/config.scm :tangle-mode (identity #o644)
;; /etc/config.scm — Tangled from guix-workstation.org (Pure GNU Guix / linux-libre)
(use-modules (gnu)
             (gnu system)
             (gnu system file-systems)
             (gnu system mapped-devices)
             (gnu services base)
             (gnu services desktop)
             (gnu services linux)
             (gnu services mcron)
             (gnu services networking)
             (gnu services ssh)
             (gnu services sysctl)
             (gnu packages admin)
             (gnu packages certs)
             (gnu packages emacs)
             (gnu packages file-systems)
             (gnu packages linux)
             (gnu packages shells)
             (gnu packages suckless)
             (gnu packages version-control)
             (guix gexp))

<<os-storage-spec>>

<<os-services-spec>>

(operating-system
  (host-name "guix-workstation")
  (timezone "Asia/Dubai")
  (locale "en_US.utf8")
  (keyboard-layout (keyboard-layout "us" #:options '("ctrl:nocaps")))

  ;; Pure FSDG-compliant GNU Linux-libre kernel (default in operating-system):
  (kernel linux-libre)
  (kernel-arguments
    (cons* "nowatchdog" "nmi_watchdog=0" %default-kernel-arguments))

  (bootloader
    (bootloader-configuration
      (bootloader grub-efi-bootloader)
      (targets '("/boot/efi"))
      (keyboard-layout keyboard-layout)
      (timeout 3)))

  (mapped-devices (list %crypt-root))
  (file-systems %workstation-file-systems)

  (users (cons (user-account
                 (name "om")
                 (comment "Omran")
                 (group "users")
                 (home-directory "/home/om")
                 (shell (file-append zsh "/bin/zsh"))
                 (supplementary-groups '("wheel" "netdev" "audio" "video" "kvm")))
               %base-user-accounts))

  (packages (cons* btrfs-progs
                   cryptsetup
                   emacs-pgtk
                   git
                   htop
                   st
                   dwm
                   dmenu
                   slock
                   %base-packages))

  (services %workstation-services))
#+end_src

* 3. Declarative User Environment (~/.config/guix/home.scm)
Guix Home manages user packages, XDG dotfiles, environment variables, and Shepherd
user daemons (like =emacs --fg-daemon=) with the same atomic rollback guarantees as
=guix system=.

** 3.1 Emacs & Guile IDE Packages for Guix Home
#+NAME: home-packages-spec
#+begin_src scheme :tangle no
(define %user-packages
  (map specification->package
       '("guile"
         "guile-readline"
         "guile-colorized"
         "emacs-pgtk"
         "emacs-geiser"
         "emacs-geiser-guile"
         "emacs-paredit"
         "emacs-rainbow-delimiters"
         "emacs-macrostep-geiser"
         "emacs-flycheck"
         "emacs-flycheck-guile"
         "emacs-guix"
         "emacs-magit"
         "emacs-org-roam"
         "emacs-vertico"
         "emacs-consult"
         "emacs-orderless"
         "emacs-marginalia"
         "emacs-corfu"
         "ripgrep"
         "fd"
         "git"
         "gnupg"
         "openssh")))
#+end_src

** 3.2 Assembled Guix Home Configuration (~/.config/guix/home.scm)
#+begin_src scheme :noweb yes :tangle ~/.config/guix/home.scm :tangle-mode (identity #o644)
;; ~/.config/guix/home.scm — Tangled from guix-workstation.org
(use-modules (gnu home)
             (gnu home services)
             (gnu home services shells)
             (gnu home services shepherd)
             (gnu packages)
             (gnu packages emacs)
             (gnu services)
             (guix gexp))

<<home-packages-spec>>

(define %emacs-daemon-service
  (shepherd-service
    (provision '(emacs-daemon))
    (documentation "Run GNU Emacs as a persistent background Lisp daemon.")
    (start #~(make-forkexec-constructor
              (list #$(file-append emacs-pgtk "/bin/emacs") "--fg-daemon")
              #:log-file (string-append (getenv "HOME") "/.local/state/log/emacs-daemon.log")))
    (stop  #~(make-kill-destructor))))

(home-environment
  (packages %user-packages)
  (services
    (list
      (service home-zsh-service-type
               (home-zsh-configuration
                 (xdg-flavor? #t)
                 (environment-variables
                   '(("EDITOR" . "emacsclient -t")
                     ("VISUAL" . "emacsclient -c -a emacs")
                     ("GUILE_AUTO_COMPILE" . "0")))
                 (aliases
                   '(("e"   . "emacsclient -t")
                     ("ec"  . "emacsclient -c -n")
                     ("gsr" . "sudo guix system reconfigure /etc/config.scm")
                     ("ghr" . "guix home reconfigure ~/.config/guix/home.scm")))))
      (service home-shepherd-service-type
               (home-shepherd-configuration
                 (services (list %emacs-daemon-service))))
      (simple-service 'guile-repl-init
                      home-files-service-type
                      `((".guile"
                         ,(plain-file "guile-init.scm"
                                      ;; Guard readline behind isatty? so Geiser never hangs!
                                      "(when (isatty? (current-input-port))\n  (use-modules (ice-9 readline) (ice-9 colorized))\n  (activate-readline)\n  (activate-colorized))\n")))))))
#+end_src
````

### Production Pro Lesson: Why `org-babel-tangle` Corrupts Patches & How to Write a Byte-Exact Batch Tangler (`tangle.el`)

**[Concept]** Built-in `org-babel-tangle` trims leading/trailing blank lines and strips trailing whitespace inside source blocks. That sounds harmless until you store a unified `diff -u` patch block inside an Org file (where an empty context line *must* contain a single space character `" "` or a trailing blank line) or when adeleted block in the Org file leaves a stale orphaned file behind in the build directory.

Here is the production `tangle.el` pattern (used in real-world declarative Org-OS trees) that wipes `build/` clean first so deleted blocks vanish, parses the Org AST with `org-element-parse-buffer`, and writes every `:tangle` block **byte-for-byte verbatim**:

```elisp
;;; tangle.el --- Byte-exact batch tangler for Org-declared OS trees -*- lexical-binding: t; -*-
;; Usage: emacs --batch -Q -l ./tangle.el system.org home.org
(require 'org)
(require 'org-element)
(require 'ob-core)

(defun my/tangle-mode (spec)
  "Parse octal file mode SPEC like \"o755\" or \"0644\"; default to #o644."
  (if (and spec (string-match "\\`o?\\([0-7]\\{3,4\\}\\)\\'" (format "%s" spec)))
      (string-to-number (match-string 1 (format "%s" spec)) 8)
    #o644))

(defun my/tangle-org-file (org-file out-root)
  "Write every :tangle block in ORG-FILE below OUT-ROOT byte-for-byte verbatim."
  (let ((count 0)
        (seen (make-hash-table :test #'equal)))
    (with-temp-buffer
      (insert-file-contents org-file)
      (delay-mode-hooks (org-mode))
      (org-element-map (org-element-parse-buffer) 'src-block
        (lambda (block)
          (let* ((params (org-babel-parse-header-arguments
                          (or (org-element-property :parameters block) "")))
                 (dest   (alist-get :tangle params)))
            (when (and dest (not (member dest '("no" "nil"))))
              (let ((target (expand-file-name dest out-root))
                    (body   (org-element-property :value block)))
                (when (gethash target seen)
                  (error "Duplicate :tangle target %s in %s" dest org-file))
                (puthash target t seen)
                (make-directory (file-name-directory target) t)
                (let ((coding-system-for-write 'utf-8-unix))
                  (write-region body nil target nil 'silent))
                (set-file-modes target (my/tangle-mode (alist-get :tangle-mode params)))
                (setq count (1+ count))))))))
    count))

(let* ((root  default-directory)
       (files (or command-line-args-left '("guix-workstation.org")))
       (total 0))
  (setq command-line-args-left nil)
  (dolist (f files)
    (setq total (+ total (my/tangle-org-file (expand-file-name f root) root))))
  (message "[OK] Tangled %d files byte-exact from %S" total files))
```

---

## 5.5 Exporting Org (`C-c C-e`) & Atomic Knowledge Graphs with `org-roam`

**[Concept]** The same `.org` file that tangles your OS can be exported to publication-grade PDF via LaTeX, HTML5, or Markdown, and cross-linked into an atomic Zettelkasten knowledge graph using **`org-roam`**.

### 1. Exporting Documents (`C-c C-e` / `org-export-dispatch`)

Press `C-c C-e` inside any Org buffer to open the interactive export dispatcher:
- `l p` — Export to LaTeX and compile to **PDF** (`ox-latex`, invoking `latexmk -pdf`).
- `l l` — Export to a standalone `.tex` file.
- `h o` — Export to **HTML5** (`ox-html`) and open in browser.
- `m m` — Export to **Markdown** (`ox-md`, once `(require 'ox-md)` is loaded).

Document-level export headers go at the top of your `.org` file:
```org
#+title: GNU Guix Production Runbook
#+author: Omran
#+options: toc:2 num:t ^:nil
#+latex_class: article
#+latex_header: \usepackage[margin=2.2cm]{geometry}
```
*(Pro Tip: Always include `#+options: ^:nil` in technical Org files so underscores in variable and file names like `xorg_server` are not misinterpreted as LaTeX subscripts!)*

### 2. `org-roam`: Atomic Knowledge Graphs with `[[id:...]]` Links

Hierarchical folder trees fail for engineering notes because a single note on *"Guix Shepherd Socket Activation"* belongs simultaneously under `Guix/`, `Linux-Init/`, and `Networking/`. **Org-roam** solves this by giving every note node (a file or any heading with an `:ID:` property) a globally unique UUID stored inside a `:PROPERTIES:` drawer, indexing all nodes and links into a local SQLite database (`~/.config/emacs/var/org-roam.db` via `org-roam-db-autosync-mode`), and displaying a live **Backlinks Buffer** showing every note that links *to* the current concept.

```org
:PROPERTIES:
:ID:       f47ac10b-58cc-4372-a567-0e02b2c3d479
:END:
#+title: Guix G-Expressions (Gexps)
#+filetags: :guix:scheme:store:

* Core Concept
A G-expression (=#~=) is an S-expression paired with its =/gnu/store= build
dependencies, staged for execution in another stratum (e.g., =shepherd= or
a derivation builder). See [[id:9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d][Guix Derivations and the Store]].
```

| Key Chord | `org-roam` Command | Purpose |
| :--- | :--- | :--- |
| `C-c n f` | `org-roam-node-find` | Fuzzy-search all existing note titles and tags; opens the note if it exists, or creates a new UUID-stamped note if it doesn't. |
| `C-c n i` | `org-roam-node-insert` | Searches for a node (or creates one on the fly) and inserts an immutable `[[id:UUID][Node Title]]` link at point—renaming the target file never breaks the link! |
| `C-c n l` | `org-roam-buffer-toggle` | Opens the side-window **Backlinks Panel** showing every note in your vault that references the current node, complete with surrounding context. |
| `C-c n d` | `org-roam-dailies-capture-today` | Appends a timestamped engineering log entry to today's `daily/YYYY-MM-DD.org` file. |


---

# Module 6: Unmentioned Pro Tips, Pitfalls, Cheat Sheets & 7-Day Capstone Curriculum

## 6.1 The "Seven Gates That Passed By Not Looking" (Verification Engineering from `guix-platform-install`)

**[Concept]** In systems and cloud-image engineering—where building a Guix image, uploading it, importing it, and booting a remote instance takes 30 to 60 minutes—a faulty assumption is not discovered in two seconds; it is discovered half an hour later on a headless machine you cannot log into.

During the engineering of `guix-platform-install`, **seven separate pre-commit and CI verification gates were discovered over two days that reported `[PASS]` while inspecting almost nothing**. Not one bug repeated seven times, but seven distinct mechanisms of false-green verification—three of them living inside the repository's primary `lib/validate-before-deploy.sh` gate. Every single one remained invisible for months *precisely because it was green*: a red check gets fixed immediately, whereas a green check that inspects nothing is trusted indefinitely.

### Anatomy of the Seven False-Green Gates

| Gate # | The Gate | What It Actually Did (Why It Passed By Not Looking) | The Engineering Fix |
| :--- | :--- | :--- | :--- |
| **Gate 1** | **The Shebang Rule** | **Nothing enforced it.** Eight scripts carried `#!/bin/bash`, which cannot execute on a Guix System because `/bin` contains only `/bin/sh` (Bash lives in `/gnu/store/.../bin/bash` or `/run/current-system/profile/bin/bash`). Worse, `run-tests.sh` *itself* had `#!/bin/bash`, making the mandated pre-commit command unrunnable directly on Guix! Nobody noticed because developers typed `bash run-tests.sh`, bypassing the kernel's `execve` shebang loader. | Scan every executable file's first two bytes (`#!`) and fail if any Guix-targeted script uses `#!/bin/bash` instead of `#!/run/current-system/profile/bin/bash` or `#!/usr/bin/env bash`. Execute test scripts directly (`./run-tests.sh`), never via `bash run-tests.sh`. |
| **Gate 2** | **The Manifest Hash Check** | **Hashed 1 file out of 54.** The loop checked a single entry and allowed the remaining 53 tracked files to drift silently forever. Even worse, on mismatch it printed `[WARN]` and exited `0`, so it never actually gated deployment. | Verify all $N$ manifest entries, assert the count (`54/54 files verified`), and exit non-zero (`exit 1`) on any mismatch or missing file. |
| **Gate 3** | **The ASCII / Unicode Check** | **Scanned 1 file and crashed silently on macOS.** The script invoked GNU-specific `grep -P '[^\x00-\x7F]'` and redirected stderr to `/dev/null` (`2>/dev/null`). On BSD/macOS `grep`, `-P` is an invalid flag that immediately exits with status `2`. Because the script treated "no output from grep" as "zero non-ASCII characters found," a crashed `grep` reported a clean `[PASS]` without reading a single byte! | Never suppress `stderr` across a pipeline whose exit code you invert! Distinguish exit code `0` (match found), `1` (no match found), and `>=2` (tool error = immediate **FAIL**), and print the exact count of scanned files. |
| **Gate 4** | **The Unit-Test Gate** | **Piped `go test ./...` into `grep -q "ok"`.** `grep -q "ok"` succeeds as soon as *one* package passes—or if any file path contains the substring `ok`—even when five other packages fail! A partially failing test suite reported a green `[PASS]`. | **Gate on exit status, never on reassuring text.** Run `if output=$(go test ./... 2>&1); then ...` so the gate succeeds if and only if the test runner's exit code is `0`. |
| **Gate 5** | **The Test Runner (`run-tests.sh`)** | **`set -e` aborted on the first deliberately tolerated failure.** Three lines above a loop explicitly annotated `# Don't fail the entire test suite on legacy converted scripts`, an active `set -e` caused the shell to terminate immediately on the first non-zero return, silently skipping all subsequent test suites! | Understand `set -e` interaction with subshells and conditionals: capture non-gating command status explicitly (`rc=0; run_optional_check || rc=$?`) while keeping mandatory gates strict. |
| **Gate 6** | **Device-Detection Tests** | **Asserted the container environment rather than the contract.** The test asserted "no block devices found," which passed inside an empty Docker container and failed on every real Linux workstation. Worse, one boolean condition was inverted so the success case was reported as failure. | Test deterministic parser/contract behavior against injected fixture inputs (`lsblk` mock trees), never the host machine's accidental hardware state. |
| **Gate 7** | **Evaluation vs. Build Tests** | **Confused Scheme evaluation with derivation building.** Both code paths were reported as "verified" because `guix system` evaluated the `.scm` file without error. However, a `#f` value inside `authorized-keys` (and an unbound `read-line` inside a Shepherd G-expression) only executes inside the **derivation builder at build/boot time**, crashing a real 40-minute cloud image build. | **Name the phase your evidence comes from:** *Evaluated* $\neq$ *Built* $\neq$ *Booted* $\neq$ *Logged In*. Gate G-expressions by actually building the derivation (`guix system build` / `guix system vm`) and exercising the runtime Shepherd service. |

### Crossing Unverified Boundaries: Why Smart Engineers Get Bitten

Every expensive failure in `guix-platform-install` came from a confident mental model of *another layer's* behavior that had not been tested across the boundary:

| What Was Assumed | The Boundary That Was Not Crossed | How to Verify Across the Boundary in < 60 Seconds |
| :--- | :--- | :--- |
| *"The `.scm` config evaluates without error $\Rightarrow$ the system builds."* | **Evaluation Time $\rightarrow$ Build Time** (G-expression builders only run when `guix-daemon` realizes the `.drv`). | Run `guix system build config.scm` (or build the specific `computed-file` / service derivation in `guix repl`). |
| *"`read-line` is in Guile core $\Rightarrow$ my Shepherd service G-exp can call `(read-line port)`."* | **Guile Interactive Env $\rightarrow$ Shepherd G-exp Module Scope** (`read-line` requires `(ice-9 rdelim)`!). | Always declare `(use-modules (ice-9 rdelim))` explicitly inside the `#~(begin ...)` G-expression body and compile-check it. |
| *"A flag accepted by `oci compute image import` is accepted by `oci compute instance launch`."* | **API Verb A $\rightarrow$ API Verb B** | Run `--help` or `--generate-full-command-json-input` on the exact target subcommand before scripting it. |
| *"My test image is pure if it has no baked SSH key."* | **Purity $\rightarrow$ Diagnosability** (When metadata injection failed, no baked key meant no login $\rightarrow$ no logs $\rightarrow$ failure reason stranded inside the dead VM). | **Design the test so failure teaches:** keep a fallback diagnostic console or emergency key during staging so a failure tells you `Unbound variable: read-line` on the first run. |

### The Three Iron Laws of Verification Gates

1. **Law 1: A gate that cannot say what it looked at is not a gate.** Every check must print an explicit count (`54/54 manifest hashes verified`, `21 scripts scanned for shebang compliance`). If a glob matches zero files (`*.scm` in the wrong directory), `0 files scanned` must trigger an immediate **FAIL**.
2. **Law 2: Gate on exit status, never on matching text.** Never pipe a test runner into `grep -q "ok"` or `grep -v "ERROR"`. Capture the command's exit status directly.
3. **Law 3: A check that cannot run must FAIL, never PASS.** If a required tool or flag is missing, or a file is unreadable, exit non-zero immediately. And whenever you write or modify a gate, **break the target on purpose first**, watch the gate turn red, and only then restore it.

**[Script/Config]** Complete, runnable pre-deploy verification script (`verify-guix-repo.sh`) implementing all three Iron Laws:

```bash
#!/usr/bin/env bash
# verify-guix-repo.sh --- Fail-closed verification gate for Guix + Org repositories
# Implements the Three Iron Laws from `guix-platform-install`:
#   1. Reports exact counts of inspected files (0 files = FAIL).
#   2. Gates strictly on command exit status, never `grep -q "ok"`.
#   3. Fails closed if any tool or portable flag fails.
set -euo pipefail

REPO_ROOT="${1:-$(pwd)}"
cd "$REPO_ROOT"

FAILURES=0
pass() { printf '[OK]   %s\n' "$1"; }
fail() { printf '[FAIL] %s\n' "$1" >&2; FAILURES=$((FAILURES + 1)); }

# --- Gate 1: Shebang Compliance (No #!/bin/bash on Guix targets) -------------
sh_count=0
bad_shebangs=0
while IFS= read -r -d '' script; do
  sh_count=$((sh_count + 1))
  first_line=$(head -n 1 "$script" || true)
  if [[ "$first_line" == "#!/bin/bash"* ]]; then
    fail "Forbidden shebang '#!/bin/bash' in $script (use #!/usr/bin/env bash or /run/current-system/profile/bin/bash)"
    bad_shebangs=$((bad_shebangs + 1))
  fi
done < <(find . -maxdepth 3 -type f \( -name "*.sh" -o -perm -u+x \) -not -path "./.git/*" -print0)

if (( sh_count == 0 )); then
  fail "Shebang gate inspected 0 files — refusing false-green pass!"
elif (( bad_shebangs == 0 )); then
  pass "Shebang gate: $sh_count executable/shell files verified clean."
fi

# --- Gate 2: Portable ASCII Check (No silent grep -P crash!) -----------------
scm_count=0
non_ascii=0
while IFS= read -r -d '' file; do
  scm_count=$((scm_count + 1))
  # Portable POSIX/Perl-free check using LC_ALL=C and tr:
  if [[ -n $(LC_ALL=C tr -d '[:print:][:space:]' < "$file") ]]; then
    fail "Non-ASCII bytes detected in $file"
    non_ascii=$((non_ascii + 1))
  fi
done < <(find . -maxdepth 3 -type f \( -name "*.scm" -o -name "*.sh" \) -not -path "./.git/*" -print0)

if (( scm_count == 0 )); then
  fail "ASCII gate inspected 0 .scm/.sh files — refusing false-green pass!"
elif (( non_ascii == 0 )); then
  pass "ASCII gate: $scm_count files scanned byte-by-byte."
fi

# --- Gate 3: Guile Syntax & Unbound-Variable Compilation Check ---------------
if command -v guild >/dev/null 2>&1; then
  compiled_count=0
  while IFS= read -r -d '' scm; do
    compiled_count=$((compiled_count + 1))
    if ! guild compile -Wunbound-variable -Warity-mismatch -o /dev/null "$scm"; then
      fail "guild compile failed on $scm"
    fi
  done < <(find . -maxdepth 2 -type f -name "*.scm" -not -path "./.git/*" -print0)
  pass "Guile compilation gate: $compiled_count Scheme files verified with guild compile."
else
  fail "Required tool 'guild' not found on PATH (Law 3: missing tool = FAIL)."
fi

if (( FAILURES > 0 )); then
  printf '\n[ERROR] Verification failed with %d broken gate(s).\n' "$FAILURES" >&2
  exit 1
fi
printf '\n[OK] All verification gates passed with explicit evidence counts.\n'
```

---

## 6.2 20 "I Wish Someone Told Me on Day 1" Tips (Lisp, Guile, Guix, Emacs & Org)

Every single item below is a real-world trap that costs newcomers hours of debugging.

### Guile Scheme Traps & Pro Tips (1–6)

1. **Guile Strings Have No Octal Escape (`"\033"` is a Trap!)**
   - *The Trap*: Writing `"\033[32mOK\033[0m"` in Guile Scheme for ANSI terminal colors.
   - *Why It Fails*: Guile interprets `\0` as the NUL byte (`\x00`) followed by two literal characters `'3'` and `'3'`!
   - *The Fix*: Always use hexadecimal escapes in Guile strings: `"\x1b[32mOK\x1b[0m"`.
2. **The Executable Guile Script Meta-Switch (`\`) Shebang**
   - *The Trap*: Writing `#!/usr/bin/env guile -e main -s` on line 1 of a script.
   - *Why It Fails*: The OS kernel passes everything after the first space in a `#!` line as a **single combined argument**, so `env` searches for an executable literally named `"guile -e main -s"`.
   - *The Fix*: Use Guile's multi-line meta-switch (`\` at the end of line 1, flags on line 2, `!#` on line 3):
     ```scheme
     #!/usr/bin/env -S guile \
     -e main -s
     !#
     (define (main args) (display "Hello from Guile!\n"))
     ```
3. **Never Mutate a Quoted Literal List (`'(1 2 3)`)**
   - *The Trap*: Writing `(let ((xs '(a b c))) (set-car! xs 'z) xs)`.
   - *Why It Fails*: Quoted literals (`'(...)` and `"..."`) live in read-only constants memory compiled into the `.go` bytecode file; mutating them either crashes with a segmentation fault/read-only error or silently corrupts future calls to the same function!
   - *The Fix*: Construct fresh mutable cons cells at runtime with `(list 'a 'b 'c)` or `(cons ...)` whenever mutation is intended.
4. **`read-line` Is Not in the Default Guile Root Module**
   - *The Trap*: Calling `(read-line port)` inside a Guix script or Shepherd G-expression and getting `Unbound variable: read-line`.
   - *The Fix*: Always import `(use-modules (ice-9 rdelim))` for `read-line` and `(use-modules (ice-9 textual-ports))` for `get-string-all`.
5. **Reading Interactive Prompts When Piped (`curl ... | guile`)**
   - *The Trap*: Calling `(read-line (current-input-port))` in an installer script that a user invokes via `curl -sSL https://... | sh`.
   - *Why It Fails*: `stdin` is the script pipe itself, so `read-line` consumes the next line of your script as the user's answer!
   - *The Fix*: Open `/dev/tty` explicitly for interactive human prompts: `(call-with-input-file "/dev/tty" read-line)`.
6. **Silence Auto-Compilation Noise in CLI Tools (`GUILE_AUTO_COMPILE=0`)**
   - *The Trap*: Running a helper `.scm` script and having `;;; note: auto-compilation is enabled, compiling ...` pollute `stdout`/`stderr` or fill `~/.cache/guile/ccache` with stale bytecode when switching branches.
   - *The Fix*: Pass `--no-auto-compile` on the CLI or export `GUILE_AUTO_COMPILE=0` during development.

### GNU Guix Traps & Pro Tips (7–13)

7. **Git-Tracked Files Only in Local Channels & `time-machine`**
   - *The Trap*: Creating a new module `my-packages/foo.scm` in a local Git repository and running `guix time-machine` or `guix pull` against it, only to get `no code for module (my-packages foo)`.
   - *Why It Fails*: Just like Nix flakes, Guix's Git fetcher only copies tracked files into the `/gnu/store` checkout!
   - *The Fix*: Run `git add -N my-packages/foo.scm` (or commit it) before evaluating channel builds.
8. **Mount the EFI System Partition at `/boot/efi`, Not `/boot`, on LVM/LUKS**
   - *The Trap*: Mounting the FAT32 ESP directly at `/boot` when `/` resides on an LVM logical volume or LUKS container.
   - *Why It Fails*: Due to Guix bug `#44877`, when `/boot` is a separate plain VFAT partition, `grub-install` does not detect that the kernel lives on LVM/LUKS and omits `lvm.mod` / `cryptodisk.mod`, dropping you into `grub rescue>` on reboot!
   - *The Fix*: Keep `/boot` inside your encrypted/LVM root filesystem and mount the FAT32 ESP at `/boot/efi` with `(targets '("/boot/efi"))`.
9. **File-System Flags (`no-suid`, `no-dev`, `no-atime`) Belong in `(flags ...)`, Not `(options ...)`**
   - *The Trap*: Writing `(options "size=8G,nosuid,nodev")` on a `tmpfs` or `btrfs` `file-system` declaration.
   - *Why It Fails*: Guix passes `(options ...)` as filesystem-specific `mount(2)` data string (`-o`), and the Linux kernel rejects generic VFS flags (`MS_NOSUID`, `MS_NODEV`, `MS_NOATIME`) inside filesystem-specific data for several filesystems during initrd boot!
   - *The Fix*: Always put VFS mount flags in the `(flags '(no-suid no-dev no-atime))` field and keep filesystem-specific parameters (`"size=8G,mode=1777"` or `"compress=zstd,subvol=@root"`) in `(options ...)`.
10. **Preserve the Executable Bit with `(local-file ... #:recursive? #t)`**
    - *The Trap*: Staging a local helper script into `/gnu/store` with `(local-file "./bin/my-script")` and finding its `chmod +x` executable bit stripped inside the store!
    - *Why It Fails*: By default, non-recursive `local-file` interns a plain data blob with `0444` permissions.
    - *The Fix*: Pass `#:recursive? #t`: `(local-file "./bin/my-script" "my-script" #:recursive? #t)`—Guix serializes it as a NAR archive, preserving the executable permission bit (`0555` in the store).
11. **SSH Rejects Symlinks into `/gnu/store` (`StrictModes`)**
    - *The Trap*: Managing `~/.ssh/config` or `~/.ssh/authorized_keys` as normal `home-files-service-type` symlinks into `/gnu/store`.
    - *Why It Fails*: Files in `/gnu/store` have `mtime = 1` (1970-01-01) and are owned by `root:root` (or `guixbuild`). OpenSSH's `StrictModes yes` checks the target of `~/.ssh/*` symlinks, sees that your user (`uid 1000`) does not own the target file, and refuses to read `~/.ssh/config` or rejects public-key logins!
    - *The Fix*: Have a Guix Home activation G-expression (`home-activation-service-type`) *copy* `~/.ssh/*` files into place with `(chmod dst #o600)` Owned by your UID.
12. **`with-imported-modules` Inside G-Expressions (`#~`)**
    - *The Trap*: Calling `mkdir-p`, `invoke`, or `find-files` from `(guix build utils)` inside a `#~(begin ...)` G-expression without wrapping it in `with-imported-modules`.
    - *Why It Fails*: The builder VM or Shepherd daemon executes in a minimal Guile environment that does not have `(guix build utils)` on its `%load-path` unless Guix explicitly copies those modules into the derivation closure.
    - *The Fix*: Wrap the G-expression in `(with-imported-modules '((guix build utils)) #~(begin (use-modules (guix build utils)) ...))`.
13. **Never Pass `#f` Inside Service Extension Lists (e.g., `authorized-keys`)**
    - *The Trap*: Writing `(authorized-keys (list (and use-laptop-key? (local-file "id.pub"))))`.
    - *Why It Fails*: When `use-laptop-key?` is `#f`, the list becomes `'(#f)`. That evaluates cleanly at `guix system` read time, and only crashes 20 minutes later when the `openssh` derivation builder runs!
    - *The Fix*: Filter with `(filter identity (list ...))` or splice conditionally with `,@(if use-laptop-key? (list (local-file "id.pub")) '())`.

### GNU Emacs & Org-Mode Traps & Pro Tips (14–20)

14. **`~/.guile` With `(ice-9 readline)` Freezes Emacs Geiser**
    - *The Trap*: Adding `(use-modules (ice-9 readline)) (activate-readline)` unconditionally to `~/.guile` and watching Emacs freeze whenever you open a `.scm` file.
    - *The Fix*: Wrap `activate-readline` in `(when (isatty? (current-input-port)) ...)` in `~/.guile` **and** set `(setq geiser-guile-load-init-file nil)` in Emacs.
15. **`emacs --batch` Does Not Activate Installed Packages Automatically**
    - *The Trap*: Running an Elisp batch script with `emacs --batch -l my-script.el` and getting `Cannot open load file: No such file or directory, dash` even though `dash` is installed.
    - *Why It Fails*: Batch mode (`--batch`) skips normal post-`early-init.el` package activation.
    - *The Fix*: Call `(package-initialize)` or `(package-activate-all)` at the top of batch scripts when third-party ELPA packages are needed.
16. **Stop Org-Babel From Stripping Indentation (`org-src-preserve-indentation t`)**
    - *The Trap*: Editing a `Makefile` (which requires hard `TAB` characters) or Python/YAML block inside Org with `C-c '` (`org-edit-special`) and having Org replace tabs with spaces or shift indentation by 2 spaces on exit!
    - *The Fix*: Set `(setq org-src-preserve-indentation t org-edit-src-content-indentation 0)` in `init.el`.
17. **Disabling LaTeX Subscript Interpretation on Underscores (`#+options: ^:nil`)**
    - *The Trap*: Exporting an Org technical specification containing `x86_64_linux` or `file_system` to PDF/HTML and seeing `64` and `system` rendered as tiny subscripts ($x86_{64_{linux}}$).
    - *The Fix*: Add `#+options: ^:nil` at the top of your `.org` file (or `(setq org-export-with-sub-superscripts '{})` globally so only `x_{sub}` with explicit braces becomes a subscript).
18. **`org-babel-tangle` Trims Blank Lines (Corrupting Diff Patches)**
    - *The Trap*: Storing a `.patch` file inside an Org `#+begin_src diff :tangle foo.patch` block and having `patch -p1` reject the tangled file with `malformed patch at line 42`.
    - *Why It Fails*: `org-babel-tangle` strips trailing whitespace and blank lines, destroying empty diff context lines.
    - *The Fix*: Use the `org-element-map` byte-exact tangler (`tangle.el` in Section 5.4) whenever tangling patches or whitespace-sensitive files.
19. **Editing Code Blocks in a Real Major-Mode Buffer (`C-c '`)**
    - *The Trap*: Fighting indentation, Flycheck, or Paredit directly inside a giant `#+begin_src scheme` block in an `.org` file.
    - *The Fix*: Press `C-c '` (`org-edit-special`) with point inside any `#+begin_src` block! Emacs opens a dedicated indirect buffer in full `scheme-mode` (with Paredit, Geiser, Flycheck, and `C-M-x` active). Press `C-c '` again to commit changes back into the Org block.
20. **Recovering Instantly When Emacs Freezes (`C-g` and `pkill -USR2 emacs`)**
    - *The Trap*: An infinite loop in Elisp or a hung network subprocess locks up the Emacs UI, and a newcomer kills the entire Emacs process (losing unsaved buffers).
    - *The Fix*: Press `C-g` (`keyboard-quit`) three times rapidly. If the main thread is still blocked in Lisp evaluation, run `pkill -SIGUSR2 emacs` from any terminal—`SIGUSR2` forces the Emacs C runtime to interrupt Lisp evaluation immediately and pop open a `*Backtrace*` buffer without losing a single character of your unsaved work!

---

## 6.3 Complete 1-Page Reference Cheat Sheets

### Cheat Sheet 1: GNU Guile Scheme Core, Modules & Records

```scheme
;; 1. MODULE DECLARATION & IMPORTS
(define-module (my-org system-utils)
  #:use-module (srfi srfi-1)                 ; list library (fold, filter, partition)
  #:use-module (srfi srfi-9)                 ; define-record-type
  #:use-module (srfi srfi-26)                ; cut / cute partial application
  #:use-module (ice-9 match)                 ; structural pattern matching
  #:use-module (ice-9 format)                ; (format #t "~a ~d~%" str num)
  #:use-module ((gnu packages base) #:prefix base:)
  #:export (make-host host? host-name))

;; 2. BINDINGS, PROCEDURES & KEYWORD ARGUMENTS
(use-modules (ice-9 optargs))
(define* (deploy-node host #:key (port 22) (dry-run? #f))
  (let* ((uri (format #f "ssh://~a:~d" host port))
         (cmd (list "guix" "deploy" uri)))
    (if dry-run? (cons 'dry-run cmd) cmd)))

;; 3. PATTERN MATCHING (`(ice-9 match)')
(define (describe-mount spec)
  (match spec
    (((? string? dev) (? string? mnt) 'btrfs subvol)
     (format #f "Btrfs ~a -> ~a (subvol=~a)" dev mnt subvol))
    ((dev mnt type)
     (format #f "Standard ~a (~a) -> ~a" dev type mnt))
    (_ (error "Invalid mount spec" spec))))

;; 4. IMMUTABLE GUIX RECORDS (`(guix records)')
(use-modules (guix records))
(define-record-type* <node-config>
  node-config make-node-config node-config?
  (hostname node-config-hostname (default "localhost"))
  (cores    node-config-cores    (default 8)))

(define prod-node
  (node-config (inherit (node-config)) (hostname "tuf") (cores 16)))
```

### Cheat Sheet 2: GNU Guix CLI, G-Expressions & Services

```bash
# DAILY PACKAGE, ENVIRONMENT & STORE COMMANDS
guix search 'regex'                        # Search package synopses & descriptions
guix shell -D guix -- make -j$(nproc)      # Ephemeral build environment for package
guix shell --pure --container --network \  # Hermetic namespace sandbox
  coreutils curl nss-certs -- sh
guix time-machine -C channels.scm -- \     # Run exact pinned Guix revision
  system build /etc/config.scm
guix describe -f channels > channels.scm   # Export current commit pin
sudo guix system reconfigure /etc/config.scm
sudo guix system roll-back                 # Atomic rollback to previous OS generation
guix home reconfigure ~/.config/guix/home.scm
guix gc -d 30d && guix gc -F 20G           # Delete >30d generations & free 20 GiB
guix size emacs-pgtk                       # Inspect transitive store closure size
guix system shepherd-graph /etc/config.scm | dot -Tpdf > services.pdf
```

```scheme
;; G-EXPRESSION (`#~') SYNTAX SUMMARY
;;   #~(...)   or  (gexp ...)         Quote build-side code staging store dependencies
;;   #$pkg     or  (ungexp pkg)       Splice host value / expand package to /gnu/store/... path
;;   #$output  or  (ungexp output)    Reference builder output path inside build-side gexp
;;   #$@lst    or  (ungexp-splicing)  Splice a host list of items into build-side list
;;   (file-append pkg "/bin/foo")     Construct "/gnu/store/...-pkg/bin/foo" cleanly
;;   (local-file "./f" #:recursive? #t) Intern local file into store preserving +x bit
;;   (plain-file "name.conf" "text")  Create store file directly from a Scheme string
```

### Cheat Sheet 3: GNU Emacs Structural Editing, Geiser & `guix.el`

| Category | Key Chord | Command | Effect |
| :--- | :--- | :--- | :--- |
| **Sexp Motion** | `C-M-f` / `C-M-b` | `forward-sexp` / `backward-sexp` | Jump forward / backward across one balanced `(...)`. |
| **List Climbing** | `C-M-d` / `C-M-u` | `down-list` / `backward-up-list` | Step inside `(` / climb out to enclosing parent `(`. |
| **Slurp & Barf** | `C-)` / `C-}` | `paredit-forward-slurp/barf-sexp` | Pull next sibling into list / push last element out of list. |
| **Wrap / Splice / Raise** | `M-(` / `M-s` / `M-r` | `wrap-round` / `splice` / `raise` | Wrap in `()` / dissolve outer `()` / replace parent with sexp at point! |
| **Balanced Kill** | `C-M-k` | `kill-sexp` | Cut the entire balanced S-expression after point. |
| **Geiser Eval** | `C-M-x` / `C-x C-e` | `geiser-eval-definition` / `last-sexp` | Evaluate top-level `define` around point / sexp before point. |
| **Geiser Buffer/REPL** | `C-c C-b` / `C-c C-z` | `geiser-eval-buffer` / `switch-to-repl` | Load entire buffer into Guile / toggle between code and REPL. |
| **Geiser Docs/Macros** | `C-c C-d C-d` / `C-c C-m C-e` | `doc-symbol-at-point` / `expand-last-sexp` | Inspect procedure signature & docstring / macroexpand form. |
| **`emacs-guix`** | `M-x guix` / `C-c . b` | `guix` / `guix-devel-build-package...` | Open Guix dashboard / build package definition at point. |

### Cheat Sheet 4: Org-Mode, Org-Babel & Tangling

| Category | Key Chord / Syntax | Command / Meaning |
| :--- | :--- | :--- |
| **Visibility Cycling** | `TAB` / `S-TAB` | Cycle current subtree (`FOLDED` $\rightarrow$ `CHILDREN` $\rightarrow$ `SUBTREE`) / cycle whole file. |
| **Subtree Restructuring** | `M-UP` / `M-DOWN` / `M-LEFT` / `M-RIGHT` | Move entire subtree up/down; promote/demote heading (`M-S-LEFT/RIGHT` for subtree). |
| **Tasks & Timestamps** | `C-c C-t` / `C-c C-s` / `C-c C-d` | Cycle `TODO` state / set `SCHEDULED:` date / set `DEADLINE:` date. |
| **Capture & Links** | `C-c c` / `C-c l` / `C-c C-l` / `C-c C-o` | `org-capture` / store link at point / insert link / open link at point. |
| **Tables & Formulas** | `TAB` / `C-c *` / `C-u C-c *` | Align table / recalculate current row / recalculate entire `#+TBLFM:` spreadsheet. |
| **Babel Execution** | `C-c C-c` / `C-c '` | Execute `#+begin_src` block at point / edit block in native major-mode buffer. |
| **Tangling & Export** | `C-c C-v t` / `C-c C-e` | `org-babel-tangle` (extract code to target files) / `org-export-dispatch` (PDF/HTML). |
| **Org-roam Graph** | `C-c n f` / `C-c n i` / `C-c n l` | Find or create node / insert `[[id:UUID][Title]]` link / toggle backlinks side buffer. |
| **Key Header Args** | `:results value\|output` `:session *s*` `:var x=blk` `:noweb yes` `:tangle /path` `:mkdirp yes` | Configure how Org-Babel executes, pipes data into, and tangles a source block. |

---

## 6.4 The 7-Day Capstone Curriculum: From Zero to Reproducible Literate Guix Workstation

**[Concept]** Do not passively read this course. Execute this 7-day engineering sprint in sequence. Every day ends with an explicit **Verification Gate** (following the Three Iron Laws of Section 6.1) that must exit `0` before you advance to the next day.

| Day | Focus Area | Concrete Engineering Deliverable | Mandatory Verification Gate (Must Exit `0`) |
| :--- | :--- | :--- | :--- |
| **Day 1** | **Guile Foundations & Functional Core** | Write `~/lab/day1-manifest-audit.scm`: a tail-recursive Guile script using `(srfi srfi-1)`, `(ice-9 match)`, and `(srfi srfi-9)` records that parses a list of files, computes their sizes, and prints a formatted ASCII report. | `guild compile -Wunbound-variable -Warity-mismatch ~/lab/day1-manifest-audit.scm && ./day1-manifest-audit.scm` |
| **Day 2** | **Emacs, Paredit & Live Geiser REPL** | Configure `~/.config/emacs/init.el` with `init-guix-ide.el` (Section 4.5). Refactor 10 nested S-expressions using only `C-)`, `C-}`, `M-s`, and `M-r` (without pressing Backspace on a parenthesis once!), and evaluate them over `geiser-connect-local`. | Run `emacs --batch -Q -l ~/.config/emacs/lisp/init-guix-ide.el --eval '(message "[OK] Emacs IDE loaded cleanly")'` |
| **Day 3** | **Store Internals, Derivations & G-Expressions** | Write `~/lab/day3-gexp.scm` using `computed-file`, `with-imported-modules`, `#~`, and `#$` that builds a `/gnu/store` derivation generating a validated configuration file and running a self-test at build time. | `guix build -f ~/lab/day3-gexp.scm` produces a `/gnu/store/...` output path and `cat $(guix build -f ~/lab/day3-gexp.scm)` displays the verified config. |
| **Day 4** | **Custom Package Engineering & Channels** | Create a local Git-tracked Guix channel in `~/lab/my-channel/` defining a custom `gnu-build-system` package and a custom Shepherd service module. | `guix build -L ~/lab/my-channel my-custom-pkg && guix lint -L ~/lab/my-channel my-custom-pkg` |
| **Day 5** | **Declarative Guix Home (`home.scm`)** | Declare your shell (`zsh`), environment variables, `~/.guile` (with `isatty?` guard!), and `emacs --fg-daemon` Shepherd user service in `home.scm`. Test it safely inside a container before reconfiguring your user profile. | `guix home container ~/.config/guix/home.scm -- emacs --version && guix home reconfigure ~/.config/guix/home.scm` |
| **Day 6** | **Literate Org-Babel Tangling & `org-roam`** | Consolidate your `channels.scm`, `/etc/config.scm`, and `home.scm` into a single `guix-workstation.org` using `:noweb yes` and byte-exact `tangle.el` (Section 5.4). Create 5 interlinked `org-roam` architecture notes (`[[id:...]]`). | `emacs --batch -Q -l tangle.el guix-workstation.org` tangles all 3 files with 0 warnings, and `diff -u` confirms byte-exact output. |
| **Day 7** | **Full OS VM Rehearsal & Fail-Closed Gate Suite** | Wire `verify-guix-repo.sh` (Section 6.1) + `guix time-machine -C channels.scm -- system vm /etc/config.scm` into a single `Makefile` (`make check`). Intentionally break one shebang and one Scheme variable to prove the gate fails red, then fix and build the bootable QEMU system VM. | `./verify-guix-repo.sh && guix time-machine -C ~/.config/guix/channels.scm -- system build /etc/config.scm` exits `0`. |
