use rand::Rng;
use std::env;
use std::fs::File;
use std::io::{self, Write};

/// Generate a file with random data of the specified size.
///
/// # Arguments
/// - `filename`: The name of the output file.
/// - `size`: The size of the file in bytes.
///
/// # Returns
/// - `Result<(), io::Error>`: Success or failure.
fn generate_random_file(filename: &str, size: usize) -> io::Result<()> {
    let mut file = File::create(filename)?;
    let mut rng = rand::thread_rng();
    const BUFFER_SIZE: usize = 8 * 1024; // 8 KB buffer
    let mut buffer = [0u8; BUFFER_SIZE];

    let mut remaining = size;

    while remaining > 0 {
        let write_size = remaining.min(BUFFER_SIZE);
        rng.fill(&mut buffer[..write_size]);
        file.write_all(&buffer[..write_size])?;
        remaining -= write_size;
    }

    Ok(())
}

fn main() {
    // Parse command-line arguments
    let args: Vec<String> = env::args().collect();
    if args.len() < 3 {
        eprintln!("Usage: {} <file_name> <size_in_bytes>", args[0]);
        std::process::exit(1);
    }

    let filename = &args[1];
    let size: usize = match args[2].parse() {
        Ok(s) => s,
        Err(_) => {
            eprintln!("Error: Size must be a positive integer.");
            std::process::exit(1);
        }
    };
    // - 1 KB = 1,024 bytes
    // - 1 MB = 1,048,576 bytes
    // - 1 GB = 1,073,741,824 bytes
    // - 1 TB = 1,099,511,627,776 bytes
    let mut modifier: u64 = 1;
    if args.len() == 4 {
        modifier = match args[3].to_lowercase().as_str() {
            "kb" => 1024,
            "mb" => 1024_u64.pow(2),
            "gb" => 1024_u64.pow(3),
            _ => 1,
        };
    }
    let mut write_size = u64::try_from(size).unwrap() * modifier;
    if write_size > (2 * 1024_u64.pow(3)) {
        write_size = 2 * 1024_u64.pow(3)
    }

    // Generate the random file
    match generate_random_file(filename, usize::try_from(write_size).unwrap()) {
        Ok(_) => println!("Successfully created {} with {} bytes.", filename, size),
        Err(e) => eprintln!("Error: {}", e),
    }
}
