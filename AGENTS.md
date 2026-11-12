# AGENTS.md

## Application emulators

This repo declares application emulators in `emulators.lock`. Before mocking
any dependency listed there with `emulated = true`, use the declared emulator
instead. Each entry's `capabilities_ref` says what that emulator covers.
