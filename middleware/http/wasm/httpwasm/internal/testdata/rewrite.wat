;; This example module is written in WebAssembly Text Format to show the
;; how a handler works and that it is decoupled from other ABI such as WASI.
;; Most users will prefer a higher-level language such as C, Rust or TinyGo.
(module $router
  ;; get_path writes the request path value to memory, if it isn't larger than
  ;; the buffer size limit. The result is the actual path length in bytes.
  (import "http-handler" "get_path" (func $get_path
    (param $buf i32) (param $buf_limit i32)
    (result (; path_len ;) i32)))

  ;; set_path overwrites the request path with one read from memory.
  (import "http-handler" "set_path" (func $set_path
    (param $path i32) (param $path_len i32)))

  ;; next dispatches control to the next handler on the host.
  (import "http-handler" "next" (func $next))

  ;; http-wasm guests are required to export "memory", so that imported
  ;; functions like "log" can read memory.
  (memory (export "memory") 1 (; 1 page==64KB ;))

  ;; define the path we expect to rewrite
  (global $match_path i32 (i32.const 0))
  (data (i32.const 0) "/v1.0/hi")
  (global $match_path_len i32 (i32.const 8))

  ;; define the path we expect to rewrite
  (global $new_path i32 (i32.const 16))
  (data (i32.const 16) "/v1.0/hello")
  (global $new_path_len i32 (i32.const 11))

  ;; buf is an arbitrary area to write data.
  (global $buf i32 (i32.const 1024))

  ;; clear_path clears any memory that may have been written.
  (func $clear_buf
    (memory.fill
      (global.get $buf)
      (global.get $match_path_len)
      (i32.const  0)))

  ;; handle rewrites the HTTP request path "/v1.0/hi" to "/v1.0/hello".
  (func $handle (export "handle")

    (local $path_len i32)

    ;; First, read the path into memory if not larger than our limit.

    ;; path_len = get_path(path, match_path_len)
    (local.set $path_len
      (call $get_path (global.get $buf) (global.get $match_path_len)))

    ;; Next, if the length read is the same as our match path, check to see if
    ;; the characters are the same.

    ;; if path_len != match_path_len { next() }
    (if (i32.eq (local.get $path_len) (global.get $match_path_len))
      (then (if (call $memeq ;; path == match_path
                  (global.get $buf)
                  (global.get $match_path)
                  (global.get $match_path_len)) (then

        ;; Call the imported function that sets the HTTP path.
        (call $set_path ;; path = new_path
          (global.get $new_path)
          (global.get $new_path_len))))))

    ;; dispatch with the possibly rewritten path.
    (call $clear_buf)
    (call $next))

  ;; memeq is like memcmp except it returns 0 (ne) or 1 (eq)
  (func $memeq (param $ptr1 i32) (param $ptr2 i32) (param $len i32) (result i32)
    (local $i1 i32)
    (local $i2 i32)
    (local.set $i1 (local.get $ptr1)) ;; i1 := ptr1
    (local.set $i2 (local.get $ptr2)) ;; i2 := ptr1

    (loop $len_gt_zero
      ;; if mem[i1] != mem[i2]
      (if (i32.ne (i32.load8_u (local.get $i1)) (i32.load8_u (local.get $i2)))
        (then (return (i32.const 0)))) ;; return 0

      (local.set $i1  (i32.add (local.get $i1)  (i32.const 1))) ;; i1++
      (local.set $i2  (i32.add (local.get $i2)  (i32.const 1))) ;; i2++
      (local.set $len (i32.sub (local.get $len) (i32.const 1))) ;; $len--

      ;; if $len > 0 { continue } else { break }
      (br_if $len_gt_zero (i32.gt_s (local.get $len) (i32.const 0))))

    (i32.const 1)) ;; return 1
)
