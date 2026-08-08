#![no_std]
//! Every entry point marked below writes storage with no authorization check
//! on any path through it, so anyone can call it.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Paused,
    Balance(Address),
}

#[contract]
pub struct Vault;

/// A helper that reads the admin but never authorizes it. Reaching this from
/// an entry point must not satisfy the rule.
fn load_admin(env: &Env) -> Address {
    env.storage()
        .instance()
        .get(&DataKey::Admin)
        .unwrap_or_else(|| panic!("not initialized"))
}

#[contractimpl]
impl Vault {
    /// Anyone can seize the contract by reassigning the admin.
    pub fn set_admin(env: Env, new_admin: Address) { //~ missing-auth
        env.storage().instance().set(&DataKey::Admin, &new_admin);
    }

    /// Debits an arbitrary address with no proof the caller controls it.
    pub fn withdraw(env: Env, from: Address, amount: i128) { //~ missing-auth
        let balance: i128 = env
            .storage()
            .persistent()
            .get(&DataKey::Balance(from.clone()))
            .unwrap_or(0);
        env.storage()
            .persistent()
            .set(&DataKey::Balance(from), &(balance - amount));
    }

    /// Loading the admin is not the same as authorizing it.
    pub fn set_paused(env: Env, paused: bool) { //~ missing-auth
        let _admin = load_admin(&env);
        env.storage().instance().set(&DataKey::Paused, &paused);
    }

    /// Deleting state is a mutation too.
    pub fn purge(env: Env, who: Address) { //~ missing-auth
        env.storage().persistent().remove(&DataKey::Balance(who));
    }
}
