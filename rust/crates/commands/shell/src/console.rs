//! Port of the parts of `github.com/containerd/console` used by `pkg.ExecShell`: `console.Current()` (the first
//! of stderr, stdout, stdin that is a terminal), `SetRaw()` (cfmakeraw, but **keeping OPOST** so that `\n` is
//! still output as `\r\n`), `Size()`, `Reset()`, and reads / writes on that terminal.

use std::io::{Read, Write};

/// The current console.
pub struct Console {
    #[cfg(unix)]
    fd: i32,
    #[cfg(unix)]
    original: Option<libc::termios>,
}

#[cfg(unix)]
fn file(fd: i32) -> std::mem::ManuallyDrop<std::fs::File> {
    use std::os::fd::FromRawFd;
    // SAFETY: the standard file descriptors stay open for the whole process; ManuallyDrop never closes them.
    std::mem::ManuallyDrop::new(unsafe { std::fs::File::from_raw_fd(fd) })
}

#[cfg(unix)]
impl Console {
    /// `console.Current()`; `None` when no standard stream is a terminal.
    pub fn current() -> Option<Console> {
        [2, 1, 0]
            .into_iter()
            .find(|fd| unsafe { libc::isatty(*fd) } == 1)
            .map(|fd| Console { fd, original: None })
    }

    fn get(&self) -> std::io::Result<libc::termios> {
        let mut t: libc::termios = unsafe { std::mem::zeroed() };
        if unsafe { libc::tcgetattr(self.fd, &mut t) } != 0 {
            return Err(std::io::Error::last_os_error());
        }
        Ok(t)
    }

    /// `SetRaw()`.
    pub fn set_raw(&mut self) -> std::io::Result<()> {
        let mut t = self.get()?;
        if self.original.is_none() {
            self.original = Some(t);
        }
        t.c_iflag &= !(libc::IGNBRK
            | libc::BRKINT
            | libc::PARMRK
            | libc::ISTRIP
            | libc::INLCR
            | libc::IGNCR
            | libc::ICRNL
            | libc::IXON);
        t.c_oflag &= !libc::OPOST;
        t.c_lflag &= !(libc::ECHO | libc::ECHONL | libc::ICANON | libc::ISIG | libc::IEXTEN);
        t.c_cflag &= !(libc::CSIZE | libc::PARENB);
        t.c_cflag |= libc::CS8;
        t.c_cc[libc::VMIN] = 1;
        t.c_cc[libc::VTIME] = 0;
        t.c_oflag |= libc::OPOST;
        if unsafe { libc::tcsetattr(self.fd, libc::TCSANOW, &t) } != 0 {
            return Err(std::io::Error::last_os_error());
        }
        Ok(())
    }

    /// `Size()`: (width, height).
    pub fn size(&self) -> std::io::Result<(u16, u16)> {
        let mut ws: libc::winsize = unsafe { std::mem::zeroed() };
        if unsafe { libc::ioctl(self.fd, libc::TIOCGWINSZ, &mut ws) } != 0 {
            return Err(std::io::Error::last_os_error());
        }
        Ok((ws.ws_col, ws.ws_row))
    }

    /// `Reset()`: restore the terminal mode.
    pub fn reset(&self) {
        if let Some(t) = &self.original {
            unsafe { libc::tcsetattr(self.fd, libc::TCSANOW, t) };
        }
    }

    /// A reader / writer of the console (`Console` implements `io.Reader` and `io.Writer` in Go).
    pub fn handle(&self) -> ConsoleHandle {
        ConsoleHandle { fd: self.fd }
    }
}

#[cfg(not(unix))]
impl Console {
    pub fn current() -> Option<Console> {
        use is_terminal::IsTerminal;
        std::io::stdin().is_terminal().then_some(Console {})
    }
    pub fn set_raw(&mut self) -> std::io::Result<()> {
        crossterm::terminal::enable_raw_mode()
    }
    pub fn size(&self) -> std::io::Result<(u16, u16)> {
        crossterm::terminal::size()
    }
    pub fn reset(&self) {
        let _ = crossterm::terminal::disable_raw_mode();
    }
    pub fn handle(&self) -> ConsoleHandle {
        ConsoleHandle {}
    }
}

/// Reads / writes on the console.
#[derive(Clone, Copy)]
pub struct ConsoleHandle {
    #[cfg(unix)]
    fd: i32,
}

impl ConsoleHandle {
    /// The path Go prints in the errors of this file.
    pub fn name(&self) -> &'static str {
        #[cfg(unix)]
        let name = match self.fd {
            0 => "/dev/stdin",
            1 => "/dev/stdout",
            _ => "/dev/stderr",
        };
        #[cfg(not(unix))]
        let name = "/dev/stdin";
        name
    }

    pub fn read(&self, buf: &mut [u8]) -> std::io::Result<usize> {
        #[cfg(unix)]
        return file(self.fd).read(buf);
        #[cfg(not(unix))]
        std::io::stdin().read(buf)
    }

    pub fn write_all(&self, data: &[u8]) -> std::io::Result<()> {
        #[cfg(unix)]
        return file(self.fd).write_all(data);
        #[cfg(not(unix))]
        {
            let mut out = std::io::stdout().lock();
            out.write_all(data)?;
            out.flush()
        }
    }
}
