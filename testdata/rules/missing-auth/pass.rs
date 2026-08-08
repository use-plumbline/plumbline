#![no_std]
//! Every state-mutating entry point here has authorization on its path, so
//! missing-auth must stay silent on this file.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env, IntoVal};

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Paused,
    Balance(Address),
}

#[contract]
pub struct Vault;

/// The idiomatic shared check: auth lives in a helper, not in the entry point.
fn require_admin(env: &Env) {
    let admin: Address = env
        .storage()
        .instance()
        .get(&DataKey::Admin)
        .unwrap_or_else(|| panic!("not initialized"));
    admin.require_auth();
}

#[contractimpl]
impl Vault {
    /// Runs once at deploy time. The deployer authorizes it by deploying, so
    /// the canonical constructor carries no require_auth.
    pub fn __constructor(env: Env, admin: Address) {
        env.storage().instance().set(&DataKey::Admin, &admin);
    }

    /// Auth called directly on the address whose balance is debited.
    pub fn withdraw(env: Env, from: Address, amount: i128) {
        from.require_auth();
        let balance: i128 = env
            .storage()
            .persistent()
            .get(&DataKey::Balance(from.clone()))
            .unwrap_or(0);
        env.storage()
            .persistent()
            .set(&DataKey::Balance(from), &(balance - amount));
    }

    /// Auth reached through a helper declared beside the contract.
    pub fn set_paused(env: Env, paused: bool) {
        require_admin(&env);
        env.storage().instance().set(&DataKey::Paused, &paused);
    }

    /// Auth bound to a subset of the arguments.
    pub fn approve(env: Env, owner: Address, spender: Address, amount: i128) {
        owner.require_auth_for_args((&spender, amount).into_val(&env));
        env.storage()
            .persistent()
            .set(&DataKey::Balance(spender), &amount);
    }

    /// Read-only. Nothing is mutated, so no authorization is required.
    pub fn balance(env: Env, who: Address) -> i128 {
        env.storage()
            .persistent()
            .get(&DataKey::Balance(who))
            .unwrap_or(0)
    }

    /// Extending a TTL is not authority-bearing: any account can extend any
    /// entry with ExtendFootprintTTLOp without the contract's involvement.
    pub fn bump(env: Env) {
        env.storage().instance().extend_ttl(120 * 17280, 180 * 17280);
    }
}
