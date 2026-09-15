#![no_std]
//! Every function here either has a one-shot guard or is not named like an
//! initializer, so missing-reinit-guard must stay silent.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey { Admin, Owner, Config, Score }

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    /// __constructor is excluded by name.
    pub fn __constructor(env: Env, admin: Address) {
        env.storage().instance().set(&DataKey::Admin, &admin);
    }

    /// init with the canonical has guard — the canonical passing case.
    pub fn init(env: Env, owner: Address) {
        if env.storage().instance().has(&DataKey::Owner) {
            panic!("already initialized");
        }
        env.storage().instance().set(&DataKey::Owner, &owner);
    }

    /// setup with the alternative get().is_some() guard.
    pub fn setup(env: Env, value: i128) {
        if env.storage().instance().get(&DataKey::Config).is_some() {
            return;
        }
        env.storage().instance().set(&DataKey::Config, &value);
    }

    /// initialize with a has guard and early return.
    pub fn initialize(env: Env, admin: Address) -> Result<(), Error> {
        if env.storage().instance().has(&DataKey::Admin) {
            return Err(Error::AlreadyInitialized);
        }
        env.storage().instance().set(&DataKey::Admin, &admin);
        Ok(())
    }

    /// configure with the negated !get().is_none() guard — semantically
    /// equivalent to get().is_some(), seen in scout-soroban-examples/amm.
    pub fn configure(env: Env, value: i128) {
        if !env.storage().instance().get(&DataKey::Config).is_none() {
            panic!("already configured");
        }
        env.storage().instance().set(&DataKey::Config, &value);
    }

    /// set_admin writes to Admin but is NOT an initializer — it is a
    /// legitimate setter called repeatedly by the admin. This is the exact
    /// false positive that caused the revert of PR #28. The rule must not
    /// fire on it.
    pub fn set_admin(env: Env, new_admin: Address) {
        env.storage().instance().set(&DataKey::Admin, &new_admin);
    }

    /// set_owner writes to Owner but is not named like an initializer.
    pub fn set_owner(env: Env, owner: Address) {
        env.storage().instance().set(&DataKey::Owner, &owner);
    }

    /// update_config writes to Config but is not named like an initializer.
    pub fn update_config(env: Env, value: i128) {
        env.storage().instance().update(&DataKey::Config, value);
    }

    /// set_score writes storage with no guard and no initializer name.
    /// The missing-auth rule covers its authorization; this rule does not fire.
    pub fn set_score(env: Env, points: i128) {
        env.storage().persistent().set(&DataKey::Score, &points);
    }
}
