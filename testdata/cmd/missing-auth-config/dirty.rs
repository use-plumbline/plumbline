#![no_std]
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
}

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    pub fn set_admin(env: Env, new_admin: Address) { //~ missing-auth
        env.storage().persistent().set(&DataKey::Admin, &new_admin);
    }
}
