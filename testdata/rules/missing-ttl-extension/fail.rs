#![no_std]
//! This contract writes persistent storage and never extends a persistent
//! TTL. When the rent runs out the entry leaves the ledger, and the SDK is
//! explicit that it can only be restored, not recreated.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

const DAY: u32 = 17_280;

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Score(Address),
}

#[contract]
pub struct Leaderboard;

#[contractimpl]
impl Leaderboard {
    pub fn __constructor(env: Env, admin: Address) {
        env.storage().instance().set(&DataKey::Admin, &admin);
    }

    /// Extending the *instance* TTL does not keep a persistent entry alive,
    /// so this contract is still reported.
    pub fn set_score(env: Env, user: Address, points: u32) {
        user.require_auth();
        env.storage()
            .persistent()
            .set(&DataKey::Score(user), &points); //~ missing-ttl-extension
        env.storage().instance().extend_ttl(30 * DAY, 60 * DAY);
    }

    pub fn score(env: Env, user: Address) -> u32 {
        env.storage()
            .persistent()
            .get(&DataKey::Score(user))
            .unwrap_or(0)
    }
}
