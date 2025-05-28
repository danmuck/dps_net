# File of Size Generator

A Rust utility that generates files of specified sizes filled with random data. Useful for testing file handling systems, storage systems, or any scenario where you need sample files of specific sizes.

## Features

- Creates files of any specified size
- Fills files with cryptographically secure random data
- Efficient buffered writing
- Simple command-line interface

## Installation

1. Ensure you have Rust installed ([rustup.rs](https://rustup.rs))
2. Clone or download this repository
3. Build the project:
```bash
cargo build --release
```

## Usage

Basic syntax:
```bash
cargo run -- <filename> <size_in_bytes>
```

Examples:
```bash
# Create a 1MB file
cargo run -- test.bin 1048576

# Create a 50MB file
cargo run -- large.dat 52428800

# Create a 1GB file
cargo run -- huge.bin 1073741824
```

Common size references:
- 1 KB = 1,024 bytes
- 1 MB = 1,048,576 bytes
- 1 GB = 1,073,741,824 bytes
- 1 TB = 1,099,511,627,776 bytes

## Performance

The program uses:
- 8KB buffer for efficient writing
- System's cryptographic random number generator
- Buffered file I/O

## Error Handling

The program will show error messages for:
- Invalid size arguments
- File system permission issues
- Insufficient disk space
- Other I/O errors

## Examples with Output

```bash
# Create a 1MB test file
$ cargo run -- test.bin 1048576
Successfully created test.bin with 1048576 bytes.

# Invalid size argument
$ cargo run -- test.bin abc
Error: Size must be a positive integer.
```

## Building for Release

For better performance, build and run the release version:
```bash
# Build
cargo build --release

# Run
./target/release/file_of_size test.bin 1048576
```

## Contributing

Feel free to submit issues and enhancement requests!

## License

This project is licensed under the MIT License - see the LICENSE file for details.
