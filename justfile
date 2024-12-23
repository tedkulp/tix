build:
  cargo build

build_release:
  cargo build --release

install: build_release
  cargo install --path .
