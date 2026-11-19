//! `qovery admin download-s3-archive`. Port of `cmd/admin_s3_archive_dowload.go` and `pkg/download_s3_archive.go`.

use crate::goutil::path_join;
use qovery_cobra::{Command, Ctx, Flag};
use regex::Regex;
use std::cell::RefCell;
use std::io::Read;
use std::rc::Rc;
use std::sync::LazyLock;

pub const DEFAULT_ARCHIVE_PROFILE: &str = "qovery-archive-reader";
const ARCHIVE_BUCKET: &str = "s3-qovery-user-logs";
const ARCHIVE_REGION: &str = "us-east-2";
const ENGINE_REQUEST_FILE_NAME: &str = "engine-request.json";

pub fn command() -> Command {
    Command::new("download-s3-archive")
        .short("Download S3 archive by execution id, with an AWS profile that assumes the archive-reader role")
        .flag(Flag::string("exec-id", "Execution id (required)").short('e'))
        .flag(Flag::string("directory", "Directory where the archive will be downloaded").short('d').default_value("."))
        .flag(Flag::bool(
            "request-only",
            "Print only the engine request of the archive on stdout instead of writing the archive",
        ))
        .flag(
            Flag::string("profile", "AWS profile that assumes the archive-reader role")
                .short('p')
                .default_value(DEFAULT_ARCHIVE_PROFILE),
        )
        .run(run)
}

fn run(ctx: &Ctx) {
    let args = Args {
        exec_id: ctx.string("exec-id"),
        directory: ctx.string("directory"),
        request_only: ctx.bool("request-only"),
        directory_changed: ctx.changed("directory"),
        profile: ctx.string("profile"),
    };
    if let Err(e) = download_s3_archive(&args) {
        qovery_log::println_error_to_stderr(e);
        qovery_core::exit(1);
    }
}

struct Args {
    exec_id: String,
    directory: String,
    request_only: bool,
    directory_changed: bool,
    profile: String,
}

/// `downloadS3Archive(cmd)`.
fn download_s3_archive(a: &Args) -> Result<(), String> {
    if a.exec_id.is_empty() {
        return Err("--exec-id is required".to_string());
    }
    if a.profile.is_empty() {
        return Err("--profile must not be empty".to_string());
    }
    if a.request_only && a.directory_changed {
        return Err("--directory cannot be used with --request-only".to_string());
    }
    if a.request_only {
        print_s3_archive_engine_request(&a.exec_id, &a.profile)
    } else {
        download(&a.exec_id, &a.directory, &a.profile)
    }
}

const UUID_PATTERN: &str = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";

static EXECUTION_ID_WITH_TIMESTAMP: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(&format!(r"^({UUID_PATTERN}(?:-\d+)?)-\d{{10}}$")).unwrap());
static EXECUTION_ID: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(&format!(r"^{UUID_PATTERN}(?:-\d+)?$")).unwrap());

/// The key the archive is stored under, and whether the id has a recognised shape. A trailing 10 digit
/// Unix timestamp is dropped; any shorter numeric suffix belongs to the key and is kept.
fn normalize_execution_id(execution_id: &str) -> (String, bool) {
    if let Some(c) = EXECUTION_ID_WITH_TIMESTAMP.captures(execution_id) {
        return (c[1].to_string(), true);
    }
    (
        execution_id.to_string(),
        EXECUTION_ID.is_match(execution_id),
    )
}

/// `pkg.DownloadS3Archive`.
fn download(execution_id: &str, directory: &str, profile: &str) -> Result<(), String> {
    let (file_name, archive) = fetch_archive(execution_id, profile)?;
    let target = path_join(&[directory, &file_name]);
    let location = if target.starts_with('/') {
        target.clone()
    } else {
        format!("./{target}")
    };
    qovery_log::println_info(format!(
        "Would you like to write the file in '{location}' ?"
    ));
    if !qovery_core::select::validate("") {
        return Ok(());
    }
    write_private_file(&target, &archive)
        .map_err(|e| format!("failed to write archive to file: {e}"))?;
    qovery_log::println_info(format!(
        "File '{location}' has been written. It holds customer secrets: delete it once done"
    ));
    Ok(())
}

/// `pkg.PrintS3ArchiveEngineRequest`.
fn print_s3_archive_engine_request(execution_id: &str, profile: &str) -> Result<(), String> {
    let (file_name, archive) = fetch_archive(execution_id, profile)?;
    let request = read_engine_request(&archive)
        .map_err(|e| format!("could not read the engine request of {file_name}: {e}"))?;
    use std::io::Write;
    let mut out = std::io::stdout().lock();
    let mut data = request;
    data.push(b'\n');
    out.write_all(&data)
        .and_then(|_| out.flush())
        .map_err(|e| e.to_string())
}

fn fetch_archive(execution_id: &str, profile: &str) -> Result<(String, Vec<u8>), String> {
    let (normalized, ok) = normalize_execution_id(execution_id);
    if !ok {
        return Err(format!(
            "invalid execution id format: '{execution_id}'. Expected '<uuid>' or '<uuid>-<id>', optionally followed by a '-<timestamp>'"
        ));
    }
    let file_name = format!("{normalized}.tgz");
    let archive = get_archive(profile, &file_name)
        .map_err(|e| describe_archive_error(&e, profile, &file_name))?;
    if normalized != execution_id {
        qovery_log::println_info_to_stderr(format!(
            "Execution id '{execution_id}' ends with a timestamp, using archive {file_name}"
        ));
    }
    Ok((file_name, archive))
}

const ERR_ENGINE_REQUEST_NOT_IN_ARCHIVE: &str = "no engine-request.json at the archive root: archives produced before the engine stored the request do not have it";

/// Go `gzip` error texts.
fn gzip_error(e: &std::io::Error) -> String {
    let msg = e.to_string().to_lowercase();
    if e.kind() == std::io::ErrorKind::UnexpectedEof {
        "unexpected EOF".to_string()
    } else if msg.contains("checksum") || msg.contains("crc") {
        "gzip: invalid checksum".to_string()
    } else if msg.contains("header") {
        "gzip: invalid header".to_string()
    } else if msg.contains("deflate") {
        "flate: corrupt input before offset 0".to_string()
    } else {
        e.to_string()
    }
}

/// The gzip stream read by the tar reader, remembering the gzip error (Go text) to tell it apart from tar errors:
/// Go's `tar.Reader` returns the errors of its underlying `gzip.Reader` as they are.
struct GzReader<'a> {
    inner: flate2::read::MultiGzDecoder<&'a [u8]>,
    err: Rc<RefCell<Option<String>>>,
}

impl Read for GzReader<'_> {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        self.inner.read(buf).inspect_err(|e| {
            self.err.borrow_mut().get_or_insert_with(|| gzip_error(e));
        })
    }
}

/// Go `path.Clean`-based lookup of `<dir>/engine-request.json` (a regular file directly under the root directory).
/// Like Go, the archive is decompressed while the tar entries are walked (not in memory first), and the rest of
/// the gzip stream is drained after the match so that a corrupt or truncated archive is still reported.
fn read_engine_request(archive: &[u8]) -> Result<Vec<u8>, String> {
    if archive.is_empty() {
        return Err("EOF".to_string());
    }
    if archive.len() < 10 || archive[0] != 0x1f || archive[1] != 0x8b || archive[2] != 8 {
        if archive.len() < 10 {
            return Err("unexpected EOF".to_string());
        }
        return Err("gzip: invalid header".to_string());
    }
    let gz_err = Rc::new(RefCell::new(None));
    let mut tar = tar::Archive::new(GzReader {
        inner: flate2::read::MultiGzDecoder::new(archive),
        err: gz_err.clone(),
    });
    let fail = |e: std::io::Error, tar_prefix: bool| {
        gz_err.borrow_mut().take().unwrap_or_else(|| {
            if tar_prefix {
                format!("archive/tar: {e}")
            } else {
                e.to_string()
            }
        })
    };
    let mut request = None;
    for entry in tar.entries().map_err(|e| fail(e, false))? {
        let mut entry = entry.map_err(|e| fail(e, true))?;
        let raw_name = String::from_utf8_lossy(&entry.path_bytes()).to_string();
        let name = crate::goutil::path_clean(raw_name.strip_prefix("./").unwrap_or(&raw_name));
        let base = name.rsplit('/').next().unwrap_or("");
        let is_reg = entry.header().entry_type() == tar::EntryType::Regular;
        if is_reg && base == ENGINE_REQUEST_FILE_NAME && name.matches('/').count() == 1 {
            let mut data = Vec::new();
            entry.read_to_end(&mut data).map_err(|e| fail(e, false))?;
            request = Some(data);
            break;
        }
    }
    let request = request.ok_or_else(|| ERR_ENGINE_REQUEST_NOT_IN_ARCHIVE.to_string())?;
    // io.Copy(io.Discard, gz)
    std::io::copy(&mut tar.into_inner(), &mut std::io::sink()).map_err(|e| fail(e, false))?;
    Ok(request)
}

use qovery_core::gosys::path_error as go_path_error;

fn write_private_file(path: &str, data: &[u8]) -> Result<(), String> {
    use std::io::Write;
    let mut opts = std::fs::OpenOptions::new();
    opts.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        opts.mode(0o600);
    }
    let mut file = opts
        .open(path)
        .map_err(|e| go_path_error("open", path, &e))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        file.set_permissions(std::fs::Permissions::from_mode(0o600))
            .map_err(|e| go_path_error("chmod", path, &e))?;
    }
    file.write_all(data)
        .map_err(|e| go_path_error("write", path, &e))
}

/// Errors of the archive download, classified like `describeArchiveError`.
#[derive(Debug)]
enum ArchiveError {
    /// `config.SharedConfigProfileNotExistError` (the missing profile name)
    MissingProfile(String),
    /// error code of an S3 API error (`AccessDenied`, ...) with the full error text
    Api(Option<String>, String),
    Other(String),
}

/// Go `config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))` fails before any request when the
/// profile, or the `source_profile` it links to, is not defined in the shared config files.
fn check_profile(profile: &str) -> Result<(), ArchiveError> {
    let fs = aws_types::os_shim_internal::Fs::real();
    let env = aws_types::os_shim_internal::Env::real();
    #[allow(
        deprecated,
        reason = "the default shared config/credentials profile files, like the Go AWS SDK"
    )]
    let files = aws_config::profile::profile_file::ProfileFiles::default();
    let rt = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|e| ArchiveError::Other(e.to_string()))?;
    let set = rt
        .block_on(aws_config::profile::load(
            &fs,
            &env,
            &files,
            Some(profile.to_string().into()),
        ))
        .map_err(|e| ArchiveError::Other(e.to_string()))?;
    let mut name = profile.to_string();
    let mut seen: Vec<String> = Vec::new();
    loop {
        let Some(p) = set.get_profile(&name) else {
            return Err(ArchiveError::MissingProfile(name));
        };
        let has_keys = p.get("aws_access_key_id").is_some_and(|k| !k.is_empty())
            && p.get("aws_secret_access_key")
                .is_some_and(|k| !k.is_empty());
        if !seen.is_empty() && has_keys {
            return Ok(());
        }
        if seen.contains(&name) {
            return Ok(());
        }
        seen.push(name.clone());
        match p.get("source_profile") {
            Some(src) if !src.is_empty() && src != name => name = src.to_string(),
            _ => return Ok(()),
        }
    }
}

fn get_archive(profile: &str, key: &str) -> Result<Vec<u8>, ArchiveError> {
    check_profile(profile)?;
    let rt = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|e| ArchiveError::Other(e.to_string()))?;
    rt.block_on(async {
        let cfg = aws_config::defaults(aws_config::BehaviorVersion::latest())
            .profile_name(profile)
            .region(aws_config::Region::new(ARCHIVE_REGION))
            .load()
            .await;
        let client = aws_sdk_s3::Client::new(&cfg);
        let object = client
            .get_object()
            .bucket(ARCHIVE_BUCKET)
            .key(key)
            .send()
            .await
            .map_err(|e| {
                use aws_sdk_s3::error::ProvideErrorMetadata;
                let code = e.code().map(|c| c.to_string());
                ArchiveError::Api(code, aws_sdk_s3::error::DisplayErrorContext(&e).to_string())
            })?;
        let has_checksum = object.checksum_crc32().is_some()
            || object.checksum_crc32_c().is_some()
            || object.checksum_crc64_nvme().is_some()
            || object.checksum_sha1().is_some()
            || object.checksum_sha256().is_some();
        if !has_checksum {
            // debug log of the Go SDK response checksum validation (written on stderr by the Go CLI)
            eprintln!(
                "SDK {} DEBUG Response has no supported checksum. Not validating response payload.",
                chrono::Local::now().format("%Y/%m/%d %H:%M:%S")
            );
        }
        let body = object
            .body
            .collect()
            .await
            .map_err(|e| ArchiveError::Other(e.to_string()))?;
        Ok(body.into_bytes().to_vec())
    })
}

fn describe_archive_error(err: &ArchiveError, profile: &str, key: &str) -> String {
    let text = match err {
        ArchiveError::MissingProfile(missing) => {
            if missing != profile {
                return format!(
                    "AWS profile '{missing}' not found in ~/.aws/config. It is the 'source_profile' of '{profile}' and must be your SSO profile of the qovery-prod account"
                );
            }
            return format!(
                "AWS profile '{profile}' not found in ~/.aws/config. Add it with 'role_arn' set to the archive-reader role and 'source_profile' set to your SSO profile of the qovery-prod account, or pass your own profile with --profile <name>"
            );
        }
        ArchiveError::Api(code, text) => {
            if is_sso_session_error(code.as_deref(), text) {
                return "No valid AWS SSO session: run 'aws sso login --profile <your SSO profile of the qovery-prod account>', then retry".to_string();
            }
            if code.as_deref() == Some("AccessDenied") {
                return format!(
                    "Access denied to archive {key} with AWS profile '{profile}': either the archive does not exist (archives expire after 30 days) or the profile does not assume the archive-reader role"
                );
            }
            text.clone()
        }
        ArchiveError::Other(text) => {
            if is_sso_session_error(None, text) {
                return "No valid AWS SSO session: run 'aws sso login --profile <your SSO profile of the qovery-prod account>', then retry".to_string();
            }
            text.clone()
        }
    };
    format!("Could not download archive {key} with AWS profile '{profile}': {text}")
}

fn is_sso_session_error(code: Option<&str>, text: &str) -> bool {
    if matches!(
        code,
        Some("InvalidGrantException") | Some("UnauthorizedException")
    ) {
        return true;
    }
    text.contains("SSO token") || (text.contains("SSO") && text.to_lowercase().contains("token"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use flate2::write::GzEncoder;
    use std::io::Write;

    #[test]
    fn rejects_invalid_flags() {
        let base = || Args {
            exec_id: "exec-id".into(),
            directory: ".".into(),
            request_only: true,
            directory_changed: false,
            profile: DEFAULT_ARCHIVE_PROFILE.into(),
        };
        let mut a = base();
        a.exec_id = String::new();
        assert_eq!(
            download_s3_archive(&a).unwrap_err(),
            "--exec-id is required"
        );
        let mut a = base();
        a.profile = String::new();
        assert_eq!(
            download_s3_archive(&a).unwrap_err(),
            "--profile must not be empty"
        );
        let mut a = base();
        a.directory = "/tmp".into();
        a.directory_changed = true;
        assert_eq!(
            download_s3_archive(&a).unwrap_err(),
            "--directory cannot be used with --request-only"
        );
    }

    #[test]
    fn normalize() {
        let cases: &[(&str, Option<&str>)] = &[
            (
                "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211-1791383662",
                Some("0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211"),
            ),
            (
                "ef83256b-5df5-40f0-87a2-a30b62998836-1791386238",
                Some("ef83256b-5df5-40f0-87a2-a30b62998836"),
            ),
            (
                "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211",
                Some("0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211"),
            ),
            (
                "0781cf11-05c2-4844-8f4e-d72f73a37bfa",
                Some("0781cf11-05c2-4844-8f4e-d72f73a37bfa"),
            ),
            ("not-an-execution-id", None),
            ("0781cf11-05c2-4844-8f4e-d72f73a37bfa-abc", None),
            ("0781CF11-05c2-4844-8f4e-d72f73a37bfa-1211", None),
            (
                "0781cf11-05c2-4844-8f4e-d72f73a37bfa-123456789",
                Some("0781cf11-05c2-4844-8f4e-d72f73a37bfa-123456789"),
            ),
            (
                "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211-1791383662-42",
                None,
            ),
        ];
        for (input, want) in cases {
            let (got, ok) = normalize_execution_id(input);
            assert_eq!(ok, want.is_some(), "{input}");
            if let Some(w) = want {
                assert_eq!(&got, w);
            }
        }
    }

    #[test]
    fn describe_errors() {
        let p = DEFAULT_ARCHIVE_PROFILE;
        let d = |e: ArchiveError| describe_archive_error(&e, p, "key.tgz");
        assert!(d(ArchiveError::MissingProfile(p.into())).contains("AWS profile 'qovery-archive-reader' not found in ~/.aws/config. Add it with 'role_arn' set to the archive-reader role and 'source_profile' set to your SSO profile of the qovery-prod account, or pass your own profile with --profile <name>"));
        assert!(d(ArchiveError::MissingProfile("qovery-prdo".into()))
            .contains("AWS profile 'qovery-prdo' not found in ~/.aws/config. It is the 'source_profile' of 'qovery-archive-reader'"));
        assert!(
            d(ArchiveError::Other(
                "failed to read cached SSO token file".into()
            ))
            .contains("No valid AWS SSO session")
        );
        assert!(
            d(ArchiveError::Api(
                Some("InvalidGrantException".into()),
                String::new()
            ))
            .contains("No valid AWS SSO session")
        );
        assert!(
            d(ArchiveError::Api(
                Some("AccessDenied".into()),
                String::new()
            ))
            .contains("Access denied to archive key.tgz")
        );
        assert_eq!(
            d(ArchiveError::Other("boom".into())),
            "Could not download archive key.tgz with AWS profile 'qovery-archive-reader': boom"
        );
    }

    #[test]
    fn write_private_file_restricts_an_existing_file() {
        use std::os::unix::fs::PermissionsExt;
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("archive.tgz");
        std::fs::write(&path, "old").unwrap();
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o644)).unwrap();
        write_private_file(path.to_str().unwrap(), b"new").unwrap();
        assert_eq!(
            std::fs::metadata(&path).unwrap().permissions().mode() & 0o777,
            0o600
        );
        assert_eq!(std::fs::read_to_string(&path).unwrap(), "new");
    }

    enum Kind {
        Reg,
        Symlink,
    }

    fn make_archive(entries: &[(&str, Kind, &str)], level: flate2::Compression) -> Vec<u8> {
        let mut builder = tar::Builder::new(Vec::new());
        let mut dir = tar::Header::new_gnu();
        dir.set_entry_type(tar::EntryType::Directory);
        dir.set_mode(0o755);
        dir.set_size(0);
        builder
            .append_data(&mut dir, "exec-id/", std::io::empty())
            .unwrap();
        for (name, kind, content) in entries {
            let mut h = tar::Header::new_gnu();
            h.set_mode(0o644);
            match kind {
                Kind::Reg => {
                    h.set_entry_type(tar::EntryType::Regular);
                    h.set_size(content.len() as u64);
                    // set_path refuses "./" prefixes: write the raw name
                    let n = name.as_bytes();
                    h.as_old_mut().name[..n.len()].copy_from_slice(n);
                    h.set_cksum();
                    builder.append(&h, content.as_bytes()).unwrap();
                }
                Kind::Symlink => {
                    h.set_entry_type(tar::EntryType::Symlink);
                    h.set_size(0);
                    builder.append_link(&mut h, name, content).unwrap();
                }
            }
        }
        let tar = builder.into_inner().unwrap();
        let mut gz = GzEncoder::new(Vec::new(), level);
        gz.write_all(&tar).unwrap();
        gz.finish().unwrap()
    }

    #[test]
    fn reads_engine_request() {
        let root = (
            "exec-id/engine-request.json",
            Kind::Reg,
            "{\"id\":\"exec-id\"}",
        );
        let nested = || {
            (
                "exec-id/charts/engine-request.json",
                Kind::Reg,
                "{\"nested\":true}",
            )
        };
        let level = flate2::Compression::default();
        for entries in [
            vec![nested(), (root.0, Kind::Reg, root.2)],
            vec![(root.0, Kind::Reg, root.2), nested()],
            vec![(
                "./exec-id/engine-request.json",
                Kind::Reg,
                "{\"id\":\"exec-id\"}",
            )],
        ] {
            assert_eq!(
                read_engine_request(&make_archive(&entries, level)).unwrap(),
                b"{\"id\":\"exec-id\"}"
            );
        }
    }

    #[test]
    fn engine_request_missing() {
        let level = flate2::Compression::default();
        for entries in [
            vec![("exec-id/charts/engine-request.json", Kind::Reg, "{}")],
            vec![("exec-id/engine-request.json", Kind::Symlink, "/etc/passwd")],
        ] {
            assert_eq!(
                read_engine_request(&make_archive(&entries, level)).unwrap_err(),
                ERR_ENGINE_REQUEST_NOT_IN_ARCHIVE
            );
        }
    }

    #[test]
    fn not_gzip() {
        assert!(read_engine_request(b"not an archive").is_err());
    }

    #[test]
    fn corrupt() {
        let mut archive = make_archive(
            &[(
                "exec-id/engine-request.json",
                Kind::Reg,
                "{\"secret\":\"value\"}",
            )],
            flate2::Compression::none(),
        );
        let at = archive.windows(6).position(|w| w == b"secret").unwrap();
        archive[at] = b'X';
        assert_eq!(
            read_engine_request(&archive).unwrap_err(),
            "gzip: invalid checksum"
        );
    }

    #[test]
    fn truncated() {
        let content = "x".repeat(300000);
        let archive = make_archive(
            &[("exec-id/engine-request.json", Kind::Reg, &content)],
            flate2::Compression::default(),
        );
        assert!(read_engine_request(&archive[..archive.len() / 2]).is_err());
    }

    /// Go drains the gzip stream after the match: a truncated archive is reported even when the engine request
    /// itself was read whole.
    #[test]
    fn truncated_after_match() {
        let mut seed: u32 = 1;
        let noise: String = (0..200000)
            .map(|_| {
                seed = seed.wrapping_mul(1103515245).wrapping_add(12345);
                char::from(b'a' + ((seed >> 16) % 26) as u8)
            })
            .collect();
        let archive = make_archive(
            &[
                ("exec-id/engine-request.json", Kind::Reg, "{\"id\":1}"),
                ("exec-id/logs.txt", Kind::Reg, &noise),
            ],
            flate2::Compression::default(),
        );
        assert_eq!(
            read_engine_request(&archive[..archive.len() / 2]).unwrap_err(),
            "unexpected EOF"
        );
    }
}
