#![no_std]
//! Comments or strings mentioning contractmeta! are not authored metadata.
use soroban_sdk::{contract, contractimpl, Env};

const README_HINT: &str = "add contractmeta! later";

#[contract]
pub struct Vault; //~ contractmeta-missing

#[contractimpl]
impl Vault {
    pub fn ping(_env: Env) -> &'static str {
        README_HINT
    }
}
