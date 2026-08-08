#![no_std]
//! Every arithmetic expression here is either checked, saturating, or on a
//! type this rule deliberately leaves alone.
use soroban_sdk::{contract, contracterror, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Balance(Address),
    Calls,
}

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum Error {
    Overflow = 1,
}

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    /// checked_* returns None instead of overflowing.
    pub fn credit(env: Env, to: Address, amount: i128) -> Result<i128, Error> {
        let balance: i128 = env
            .storage()
            .persistent()
            .get(&DataKey::Balance(to.clone()))
            .unwrap_or(0);
        let updated = balance.checked_add(amount).ok_or(Error::Overflow)?;
        env.storage()
            .persistent()
            .set(&DataKey::Balance(to), &updated);
        Ok(updated)
    }

    /// saturating_* clamps, which is a deliberate choice rather than a bug.
    pub fn scale(_env: Env, amount: i128, factor: i128) -> i128 {
        amount.saturating_mul(factor)
    }

    /// A u32 counter is out of scope: this rule targets token-sized integers.
    pub fn bump_calls(env: Env) -> u32 {
        let calls: u32 = env.storage().instance().get(&DataKey::Calls).unwrap_or(0);
        let next = calls + 1;
        env.storage().instance().set(&DataKey::Calls, &next);
        next
    }

    /// Inference through an unannotated binding: `count` is u32 because the
    /// parameter is, so `count * 2` stays out of scope too.
    pub fn double_count(_env: Env, count: u32) -> u32 {
        let doubled = count * 2;
        doubled
    }

    /// A constant expression is folded by the compiler, which rejects an
    /// overflowing one at build time.
    pub fn bump_ttl(env: Env) {
        env.storage().instance().extend_ttl(120 * 17280, 180 * 17280);
    }
}
