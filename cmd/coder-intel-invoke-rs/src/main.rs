use chrono::prelude::*;
use lazy_static::lazy_static;
use prost::Message;
#[cfg(not(target_os = "windows"))]
use std::os::unix::net::UnixDatagram;
use std::{
    env,
    error::Error,
    io::{Read, Write},
    net::{TcpStream, ToSocketAddrs},
    process::{Command, Stdio},
    time::Duration,
};
use which::which;

mod inteld {
    include!(concat!(env!("OUT_DIR"), "/inteld.rs"));
}

lazy_static! {
    static ref DEBUG_ENABLED: bool = {
        if let Ok(_) = env::var("CODER_INTEL_INVOKE_DEBUG") {
            true
        } else {
            false
        }
    };
}

macro_rules! debug {
    ($($arg:tt)*) => ({
        if *DEBUG_ENABLED {
            println!($($arg)*);
        }
    });
}

fn main() -> Result<(), Box<dyn Error>> {
    if let Ok(_) = env::var("CODER_INTEL_INVOKE_DEBUG") {}
    run()
}

fn run() -> Result<(), Box<dyn Error>> {
    let path = env::var("PATH")?;
    let mut args = env::args();

    // binary_path is the path that was used to invoke the binary. It could be
    // "ls", "/usr/bin/ls", "cargo", etc.
    let binary_path = args.next().unwrap();
    debug!("binary_path: {:?}", binary_path);

    // binary_name is the file name from the binary_path. It turns "/usr/bin/ls"
    // into "ls".
    let binary_name = std::path::PathBuf::from(&binary_path);
    let binary_name = binary_name.file_name().unwrap();
    debug!("binary_name: {:?}", binary_name);

    // bin_path tries to find the full path of the binary. It accomplishes the
    // same thing as `which ls`.
    let bin_path = which(&binary_path)?;
    debug!("bin_path: {:?}", bin_path);

    // bin_path_parent is the parent directory of the bin_path. We will remove
    // this directory from the path so we don't infinitely call ourselves, we
    // want to now execute the real binary.
    let bin_path_parent = bin_path.parent().unwrap();
    debug!("bin_path_parent: {:?}", bin_path_parent);
    let paths: Vec<&str> = path
        .split(':')
        .filter(|p| *p != bin_path_parent.to_str().unwrap())
        .collect();
    debug!("paths: {:?}", paths);
    env::set_var("PATH", paths.join(":"));

    // working_dir is the dir that the command is being executed from.
    let working_dir = env::current_dir()?;

    // new_bin_path is the path to the binary that we actually want to execute.
    let new_bin_path = which(&binary_path)?;
    let new_bin_path = new_bin_path.to_str().unwrap();
    debug!("new_bin_path: {:?}", new_bin_path);

    // current_exec is the path to us.
    let current_exec = env::current_exe()?;
    let current_exec = current_exec.to_str().unwrap();
    debug!("current_exec: {:?}", current_exec);

    // if we aren't being executed from a symlink, error out to prevent infinite
    // recursion.
    if new_bin_path == current_exec {
        return Err(format!(
            "supposed to be linked; bin_path: {}, current_exec: {}",
            new_bin_path, current_exec
        )
        .into());
    }

    // collect the rest of the args, not including the binary name.
    let flags = args.collect::<Vec<_>>();
    debug!("flags: {:?}", flags);
    let mut command = Command::new(binary_name);

    // attach std in/out/err to us so it functions as if we don't exist.
    command
        .args(&flags)
        .stdin(Stdio::inherit())
        .stdout(Stdio::inherit())
        .stderr(Stdio::inherit());

    // run the command and track how long it took.
    let start: DateTime<Utc> = Utc::now();
    let status = command.status()?;
    let end: DateTime<Utc> = Utc::now();

    // report all the data we collected.
    report_invocation(inteld::ReportInvocationRequest {
        executable_path: new_bin_path.to_string(),
        arguments: flags,
        duration_ms: (end - start).num_milliseconds(),
        exit_code: status.code().unwrap_or(99),
        working_directory: working_dir.to_str().unwrap().to_string(),
    })?;

    Ok(())
}

fn should_unix() -> bool {
    true
}

#[cfg(not(target_os = "windows"))]
fn report_invocation(req: inteld::ReportInvocationRequest) -> Result<(), Box<dyn Error>> {
    let stream: Box<dyn ReadWrite> = if should_unix() {
        make_unix()?
    } else {
        make_tcp()?
    };

    write_to(stream, req)?;
    Ok(())
}

#[cfg(not(target_os = "windows"))]
fn make_unix() -> Result<Box<dyn ReadWrite>, Box<dyn Error>> {
    let socket_path = env::temp_dir().join(".coder-intel.sock");
    let unix_stream = UnixDatagram::unbound()?;
    unix_stream.connect(socket_path.to_str().unwrap())?;
    unix_stream.set_read_timeout(Some(Duration::from_millis(100)))?;
    unix_stream.set_write_timeout(Some(Duration::from_millis(100)))?;
    Ok(Box::new(unix_stream))
}

fn make_tcp() -> Result<Box<dyn ReadWrite>, Box<dyn Error>> {
    let address = "127.0.0.1:15532";
    let addr = address.to_socket_addrs()?.next().unwrap();
    let tcp_stream = TcpStream::connect_timeout(&addr, Duration::from_millis(100))?;
    tcp_stream.set_read_timeout(Some(Duration::from_millis(100)))?;
    tcp_stream.set_write_timeout(Some(Duration::from_millis(100)))?;
    Ok(Box::new(tcp_stream))
}

#[cfg(target_os = "windows")]
fn report_invocation(req: inteld::ReportInvocationRequest) -> Result<(), Box<dyn Error>> {
    let stream = make_tcp()?;

    write_to(stream, req)?;
    Ok(())
}

fn write_to(
    mut stream: Box<dyn ReadWrite>,
    req: inteld::ReportInvocationRequest,
) -> Result<(), Box<dyn Error>> {
    let mut buf = bytes::BytesMut::with_capacity(req.encoded_len());
    req.encode(&mut buf)?;

    stream.write(buf.as_ref())?;
    let mut response = Vec::new();
    stream.read(&mut response)?;

    Ok(())
}

trait ReadWrite {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<()>;
    fn read(&mut self, buf: &mut Vec<u8>) -> std::io::Result<usize>;
}
impl ReadWrite for TcpStream {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<()> {
        self.write_all(buf)
    }
    fn read(&mut self, buf: &mut Vec<u8>) -> std::io::Result<usize> {
        self.read_to_end(buf)
    }
}

#[cfg(not(target_os = "windows"))]
impl ReadWrite for UnixDatagram {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<()> {
        self.send(buf)?;
        Ok(())
    }
    fn read(&mut self, _: &mut Vec<u8>) -> std::io::Result<usize> {
        Ok(0)
    }
}
