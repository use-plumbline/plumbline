#![no_std]
//! Every entry point here fails through a typed contract error or handles
//! absence explicitly, so panic-in-contract must stay silent on this file.
use soroban_sdk::{contract, contracterror, contractimpl, contracttype, panic_with_error, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Balance(Address),
}

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum Error {
    NotInitialized = 1,
    InsufficientBalance = 2,
}

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    /// Returns a typed error the caller can match on through try_withdraw.
    pub fn withdraw(env: Env, from: Address, amount: i128) -> Result<i128, Error> {
        from.require_auth();
        let balance: i128 = env
            .storage()
            .persistent()
            .get(&DataKey::Balance(from.clone()))
            .ok_or(Error::NotInitialized)?;
        if balance < amount {
            return Err(Error::InsufficientBalance);
        }
        let remaining = balance - amount;
        env.storage()
            .persistent()
            .set(&DataKey::Balance(from), &remaining);
        Ok(remaining)
    }

    /// panic_with_error! carries a contract error code, unlike a bare panic.
    pub fn admin(env: Env) -> Address {
        match env.storage().instance().get(&DataKey::Admin) {
            Some(admin) => admin,
            None => panic_with_error!(&env, Error::NotInitialized),
        }
    }

    /// unwrap_or is the intended way to read a possibly-absent entry.
    pub fn balance(env: Env, who: Address) -> i128 {
        env.storage()
            .persistent()
            .get(&DataKey::Balance(who))
            .unwrap_or(0)
    }

    /// unwrap_or_default and unwrap_or_else are equally fine.
    pub fn balance_or_default(env: Env, who: Address) -> i128 {
        env.storage()
            .persistent()
            .get(&DataKey::Balance(who))
            .unwrap_or_default()
    }
}
