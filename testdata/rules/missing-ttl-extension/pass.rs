#![no_std]
//! Every contract here either extends the TTL of the persistent entries it
//! writes, or writes no persistent entry at all, so missing-ttl-extension must
//! stay silent.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

const DAY: u32 = 17_280;

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Counter,
    Balance(Address),
}

#[contract]
pub struct Vault;

/// The idiomatic shape: the write and the extension live together in a helper
/// that entry points call, so the extension is nowhere near the entry point.
/// The whole file is searched for exactly this reason.
fn write_balance(env: &Env, who: &Address, amount: i128) {
    let key = DataKey::Balance(who.clone());
    env.storage().persistent().set(&key, &amount);
    env.storage().persistent().extend_ttl(&key, 60 * DAY, 90 * DAY);
}

#[contractimpl]
impl Vault {
    pub fn __constructor(env: Env, admin: Address) {
        env.storage().instance().set(&DataKey::Admin, &admin);
    }

    /// Writes persistent storage only through the helper above.
    pub fn credit(env: Env, to: Address, amount: i128) {
        to.require_auth();
        write_balance(&env, &to, amount);
    }

    /// The extension reached through a turbofish call. `extend_ttl::<K>(..)`
    /// parses as a generic_function wrapping the field expression, so the
    /// method name sits a level deeper than a plain call — and a rule that
    /// cannot see it reports a contract that is properly extending its TTL.
    /// The corpus has 81 turbofish storage calls, so this shape is ordinary.
    pub fn credit_turbofish(env: Env, to: Address, amount: i128) {
        to.require_auth();
        let key = DataKey::Balance(to);
        env.storage().persistent().set(&key, &amount);
        env.storage()
            .persistent()
            .extend_ttl::<DataKey>(&key, 60 * DAY, 90 * DAY);
    }

    /// Instance storage is not persistent storage; this rule is about the
    /// entries that go to the Expired State Stack.
    pub fn bump_counter(env: Env, count: u32) {
        env.storage().instance().set(&DataKey::Counter, &count);
        env.storage().instance().extend_ttl(30 * DAY, 60 * DAY);
    }

    /// Reading persistent storage does not start a rent clock, so a
    /// read-only contract needs no extension.
    pub fn balance(env: Env, who: Address) -> i128 {
        env.storage()
            .persistent()
            .get(&DataKey::Balance(who))
            .unwrap_or(0)
    }
}
