//! Port of `utils/file_handler.go`.

use qovery_core::context::go_path_error;
use std::path::Path;

/// Port of `utils.GetFullPath(clusterId)`: `/tmp/qovery_<clusterId>/` (Windows: `C:\Users\Default\AppData\Local\Temp\qovery_<id>\`).
pub fn get_full_path(cluster_id: &str) -> String {
    if cfg!(windows) {
        format!("C:\\Users\\Default\\AppData\\Local\\Temp\\qovery_{cluster_id}\\")
    } else {
        format!("/tmp/qovery_{cluster_id}/")
    }
}

/// Port of `utils.WriteInFile(clusterId, fileName, content)`: writes `content` in
/// `GetFullPath(clusterId) + fileName` (folder created with mode 0777, file with 0777) and returns the path.
/// Logs `Couldn't create folder : ...` and exits 1 when the folder cannot be created; logs
/// `Couldn't write file : ...` and returns `""` when the file cannot be written.
pub fn write_in_file(cluster_id: &str, file_name: &str, content: &[u8]) -> String {
    let full_path = get_full_path(cluster_id);
    let dir = Path::new(&full_path);
    if std::fs::metadata(dir).is_err_and(|e| e.kind() == std::io::ErrorKind::NotFound)
        && let Err(e) = mkdir_0777(dir)
    {
        qovery_log::logrus::error(format!(
            "Couldn't create folder : {}",
            go_path_error("mkdir", dir, &e)
        ));
        qovery_core::exit(1);
    }

    let file = format!("{full_path}{file_name}");
    if let Err(e) = write_0777(Path::new(&file), content) {
        qovery_log::logrus::error(format!(
            "Couldn't write file : {}",
            go_path_error("open", Path::new(&file), &e)
        ));
        return String::new();
    }
    file
}

#[cfg(unix)]
fn mkdir_0777(dir: &Path) -> std::io::Result<()> {
    use std::os::unix::fs::DirBuilderExt;
    std::fs::DirBuilder::new().mode(0o777).create(dir)
}

#[cfg(not(unix))]
fn mkdir_0777(dir: &Path) -> std::io::Result<()> {
    std::fs::create_dir(dir)
}

#[cfg(unix)]
fn write_0777(path: &Path, content: &[u8]) -> std::io::Result<()> {
    use std::io::Write;
    use std::os::unix::fs::OpenOptionsExt;
    let mut f = std::fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .mode(0o777)
        .open(path)?;
    f.write_all(content)
}

#[cfg(not(unix))]
fn write_0777(path: &Path, content: &[u8]) -> std::io::Result<()> {
    std::fs::write(path, content)
}

/// Port of `utils.DeleteFile(path)`: `os.Remove`, logging the error (logrus error level).
pub fn delete_file(path: &str) {
    let p = Path::new(path);
    let r = match std::fs::symlink_metadata(p) {
        Ok(m) if m.is_dir() => std::fs::remove_dir(p),
        _ => std::fs::remove_file(p),
    };
    if let Err(e) = r {
        qovery_log::logrus::error(go_path_error("remove", p, &e));
    }
}

/// Port of `utils.DeleteFolder(path)`: `os.RemoveAll` (a missing path is not an error), logging the error.
pub fn delete_folder(path: &str) {
    let p = Path::new(path);
    let r = match std::fs::symlink_metadata(p) {
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Ok(m) if m.is_dir() => std::fs::remove_dir_all(p),
        _ => std::fs::remove_file(p),
    };
    if let Err(e) = r {
        qovery_log::logrus::error(go_path_error("unlinkat", p, &e));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn write_and_delete() {
        let id = format!("services-test-{}", crate::util::rand_string_bytes(10));
        let path = write_in_file(&id, "kubeconfig", b"hello");
        assert_eq!(path, format!("{}kubeconfig", get_full_path(&id)));
        assert_eq!(std::fs::read(&path).unwrap(), b"hello");
        delete_file(&path);
        assert!(!Path::new(&path).exists());
        delete_folder(&get_full_path(&id));
        assert!(!Path::new(&get_full_path(&id)).exists());
        delete_folder(&get_full_path(&id));
    }
}
