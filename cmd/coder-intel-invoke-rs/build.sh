#!/bin/bash

# Before this works you need to:
# $ rustup toolchain install nightly
# $ rustup component add rust-src --toolchain nightly

RUSTFLAGS="-Zlocation-detail=none" \
cargo +nightly build \
	-Z build-std=std,panic_abort \
	-Z build-std-features=panic_immediate_abort \
	--target x86_64-unknown-linux-gnu \
	--release

# cargo +nightly build \
# 	-Z build-std=std,panic_abort \
# 	-Z build-std-features=panic_immediate_abort \
# 	--target x86_64-pc-windows-msvc \
# 	--release

# cargo +nightly build \
# 	-Z build-std=std,panic_abort \
# 	-Z build-std-features=panic_immediate_abort \
# 	--target x86_64-apple-darwin \
# 	--release
