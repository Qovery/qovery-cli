//! cobra positional arguments validators (`cobra.NoArgs`, `cobra.ExactArgs(n)`, ...).

/// Positional args rule of a command. `Legacy` is cobra's behaviour when `Args` is nil.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum ArgsRule {
    /// `Args: nil` (default): arbitrary args, except on the root command which reports unknown commands.
    #[default]
    Legacy,
    /// `cobra.ArbitraryArgs`
    Arbitrary,
    /// `cobra.NoArgs`
    NoArgs,
    /// `cobra.ExactArgs(n)`
    Exact(usize),
    /// `cobra.MinimumNArgs(n)`
    Minimum(usize),
    /// `cobra.MaximumNArgs(n)`
    Maximum(usize),
    /// `cobra.RangeArgs(min, max)`
    Range(usize, usize),
}

impl ArgsRule {
    /// Validate positional args; returns cobra's error message.
    pub(crate) fn validate(&self, args: &[String], command_path: &str) -> Result<(), String> {
        let n = args.len();
        match *self {
            ArgsRule::Legacy | ArgsRule::Arbitrary => Ok(()),
            ArgsRule::NoArgs => {
                if n > 0 {
                    Err(format!(
                        "unknown command {} for {}",
                        qovery_log::gofmt::quote(&args[0]),
                        qovery_log::gofmt::quote(command_path)
                    ))
                } else {
                    Ok(())
                }
            }
            ArgsRule::Exact(e) => {
                if n != e {
                    Err(format!("accepts {e} arg(s), received {n}"))
                } else {
                    Ok(())
                }
            }
            ArgsRule::Minimum(m) => {
                if n < m {
                    Err(format!("requires at least {m} arg(s), only received {n}"))
                } else {
                    Ok(())
                }
            }
            ArgsRule::Maximum(m) => {
                if n > m {
                    Err(format!("accepts at most {m} arg(s), received {n}"))
                } else {
                    Ok(())
                }
            }
            ArgsRule::Range(a, b) => {
                if n < a || n > b {
                    Err(format!("accepts between {a} and {b} arg(s), received {n}"))
                } else {
                    Ok(())
                }
            }
        }
    }
}
