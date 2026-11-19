//! Port of `handleBracketedPasteFragmentation` / `isPotentialBracketedPastePrefix` (`pkg/shell.go`).

const BRACKETED_PASTE_START: [u8; 6] = [0x1b, b'[', b'2', b'0', b'0', b'~'];
const BRACKETED_PASTE_END: [u8; 6] = [0x1b, b'[', b'2', b'0', b'1', b'~'];

/// Ensures bracketed paste sequences (`ESC[200~` / `ESC[201~`) are sent complete: a potentially incomplete
/// sequence at the end of `data` is kept for the next read. Returns `(data to send, pending bytes)`.
pub fn handle_bracketed_paste_fragmentation(data: &[u8]) -> (Vec<u8>, Vec<u8>) {
    if data.is_empty() {
        return (data.to_vec(), Vec::new());
    }
    let mut check_len = 1;
    while check_len < 6 && check_len <= data.len() {
        let tail = &data[data.len() - check_len..];
        if is_potential_bracketed_paste_prefix(tail) {
            return (data[..data.len() - check_len].to_vec(), tail.to_vec());
        }
        check_len += 1;
    }
    (data.to_vec(), Vec::new())
}

/// Whether `data` could be the start of a bracketed paste sequence.
pub fn is_potential_bracketed_paste_prefix(data: &[u8]) -> bool {
    if data.is_empty() || data.len() >= 6 {
        return false;
    }
    let matches_start = data
        .iter()
        .zip(BRACKETED_PASTE_START.iter())
        .all(|(a, b)| a == b);
    let matches_end = data
        .iter()
        .zip(BRACKETED_PASTE_END.iter())
        .all(|(a, b)| a == b);
    matches_start || matches_end
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn prefixes() {
        assert!(is_potential_bracketed_paste_prefix(b"\x1b"));
        assert!(is_potential_bracketed_paste_prefix(b"\x1b[20"));
        assert!(is_potential_bracketed_paste_prefix(b"\x1b[201"));
        assert!(!is_potential_bracketed_paste_prefix(b"\x1b[200~"));
        assert!(!is_potential_bracketed_paste_prefix(b"\x1b[3"));
        assert!(!is_potential_bracketed_paste_prefix(b""));
        assert!(!is_potential_bracketed_paste_prefix(b"a"));
    }

    #[test]
    fn fragmentation() {
        assert_eq!(handle_bracketed_paste_fragmentation(b""), (vec![], vec![]));
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"ls\r"),
            (b"ls\r".to_vec(), vec![])
        );
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"abc\x1b[2"),
            (b"abc".to_vec(), b"\x1b[2".to_vec())
        );
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"\x1b[200~x\x1b[20"),
            (b"\x1b[200~x".to_vec(), b"\x1b[20".to_vec())
        );
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"x\x1b[201~"),
            (b"x\x1b[201~".to_vec(), vec![])
        );
        // the shortest matching tail wins
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"\x1b"),
            (vec![], b"\x1b".to_vec())
        );
        // an arrow key is not a paste prefix
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"\x1b[A"),
            (b"\x1b[A".to_vec(), vec![])
        );
    }
}
