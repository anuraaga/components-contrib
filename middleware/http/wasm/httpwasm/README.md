## Basic WebAssembly Middleware

WebAssembly is a way to safely run code compiled in other languages. Runtimes
execute WebAssembly Modules (Wasm), which are most often binaries with a
`.wasm` extension.

This component allows you to handle a server request with custom logic compiled
to a Wasm using the [http-wasm handler protocol][1].

Please see the [documentation][2] for general configuration.

### Generating Wasm

To compile your wasm, you must compile source using a http-wasm guest SDK such
as [TinyGo][3]. You can also make a copy of [hello.go](./example/example.go)
and replace the `handler.HandleFn` function with your custom logic.

If using TinyGo, compile like so and set the `path` attribute to the output:
```bash
tinygo build -o example.wasm -scheduler=none --no-debug -target=wasi example.go`
```

### Notes

* This is an alpha feature, so configuration is subject to change.
* This module implements the host side of the http-wasm handler protocol.
* This uses [wazero][4] for the WebAssembly runtime as it has no dependencies,
  nor relies on CGO. This allows installation without shared libraries.
* Many WebAssembly compilers leave memory unbounded and/or set to 16MB. Do not
  set a large pool size without considering memory amplification.

[1]: https://github.com/http-wasm/http-wasm-abi
[2]: https://github.com/dapr/docs/blob/v1.8/daprdocs/content/en/reference/components-reference/supported-middleware/middleware-wasm.md
[3]: https://github.com/http-wasm/http-wasm-guest-tinygo
[4]: https://wazero.io
 