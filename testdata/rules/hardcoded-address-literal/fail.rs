#![no_std]
use soroban_sdk::{contract, contractimpl, Address, Env, String};

const ADMIN: &str = "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"; //~ hardcoded-address-literal

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    pub fn admin(env: Env) -> Address {
        Address::from_str(&env, ADMIN)
    }

    pub fn token(env: Env) -> Address {
        Address::from_string(&String::from_str(&env, "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")) //~ hardcoded-address-literal
    }

    pub fn recipient() -> &'static str {
        "MAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" //~ hardcoded-address-literal
    }
}
