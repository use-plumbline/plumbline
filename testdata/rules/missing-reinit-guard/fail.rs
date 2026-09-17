#![no_std]
//! Every initializer below writes storage without a one-shot guard, so
//! missing-reinit-guard must fire on each.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey { Admin, Owner, Config }

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    /// initialize with no guard — anyone can call it again and replace the admin.
    pub fn initialize(env: Env, admin: Address) { //~ missing-reinit-guard
        env.storage().instance().set(&DataKey::Admin, &admin);
    }

    /// init with no guard — same problem, different name.
    pub fn init(env: Env, owner: Address) { //~ missing-reinit-guard
        env.storage().instance().set(&DataKey::Owner, &owner);
    }

    /// setup with no guard — the third conventional initializer name.
    pub fn setup(env: Env, value: i128) { //~ missing-reinit-guard
        env.storage().instance().update(&DataKey::Config, value);
    }
}
