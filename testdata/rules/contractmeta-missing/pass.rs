#![no_std]
//! These cases all have authored metadata or no #[contract] declaration in
//! this file, so contractmeta-missing must stay silent.
use soroban_sdk::{contract, contractimpl, contractmeta, Env};

contractmeta!(
    key = "desc",
    val = "Documented vault contract"
);

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    pub fn ping(_env: Env) {}
}

/// Bare helper code is not a contract file.
fn helper_label() -> &'static str {
    "helper mentions contractmeta! only as text"
}

/// Implementation-only files are deliberately out of scope: a multi-file crate
/// can keep #[contract] and contractmeta! in another file.
#[contractimpl]
impl ExternalContract {
    pub fn pong(_env: Env) {}
}
