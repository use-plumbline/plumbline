#![no_std]
//! A deposit-and-withdraw vault over a SEP-41 token.
//!
//! This is the contract Plumbline lints in its own CI. It is written the way
//! the rules want contracts written — every mutating entry point authorizes an
//! address, every fallible path returns a typed contract error, and every
//! balance calculation is checked — so a finding here means Plumbline has
//! started reporting things that are not defects.

use soroban_sdk::{
    contract, contracterror, contractevent, contractimpl, contractmeta, contracttype, token,
    Address, Env,
};

/// Ledgers close about every five seconds, so this is roughly one day.
const DAY_IN_LEDGERS: u32 = 17_280;
const INSTANCE_TTL_THRESHOLD: u32 = 30 * DAY_IN_LEDGERS;
const INSTANCE_TTL_EXTEND_TO: u32 = 60 * DAY_IN_LEDGERS;
const BALANCE_TTL_THRESHOLD: u32 = 60 * DAY_IN_LEDGERS;
const BALANCE_TTL_EXTEND_TO: u32 = 90 * DAY_IN_LEDGERS;

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Token,
    /// One entry per depositor, so unrelated depositors do not contend for the
    /// same ledger entry.
    Balance(Address),
}

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum Error {
    NotInitialized = 1,
    InvalidAmount = 2,
    InsufficientBalance = 3,
    BalanceOverflow = 4,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Deposited {
    #[topic]
    pub from: Address,
    pub amount: i128,
    pub balance: i128,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Withdrawn {
    #[topic]
    pub to: Address,
    pub amount: i128,
    pub balance: i128,
}

contractmeta!(
    key = "desc",
    val = "Deposit-and-withdraw vault over a SEP-41 token"
);

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    /// Runs once, atomically, at deploy time. The deployer authorizes this by
    /// deploying, so there is no separate auth check.
    pub fn __constructor(env: Env, admin: Address, token: Address) {
        env.storage().instance().set(&DataKey::Admin, &admin);
        env.storage().instance().set(&DataKey::Token, &token);
    }

    /// Moves `amount` from the depositor into the vault and credits them.
    pub fn deposit(env: Env, from: Address, amount: i128) -> Result<i128, Error> {
        from.require_auth();
        if amount <= 0 {
            return Err(Error::InvalidAmount);
        }

        let balance = read_balance(&env, &from);
        let updated = balance.checked_add(amount).ok_or(Error::BalanceOverflow)?;

        token::TokenClient::new(&env, &read_token(&env)?).transfer(
            &from,
            &env.current_contract_address(),
            &amount,
        );
        write_balance(&env, &from, updated);
        extend_instance_ttl(&env);

        Deposited {
            from,
            amount,
            balance: updated,
        }
        .publish(&env);
        Ok(updated)
    }

    /// Returns `amount` to the withdrawer and debits their balance.
    pub fn withdraw(env: Env, to: Address, amount: i128) -> Result<i128, Error> {
        to.require_auth();
        if amount <= 0 {
            return Err(Error::InvalidAmount);
        }

        let balance = read_balance(&env, &to);
        // checked_sub only catches wrap-around; a balance that is merely too
        // small produces a negative result, so it is rejected separately.
        let remaining = balance.checked_sub(amount).ok_or(Error::BalanceOverflow)?;
        if remaining < 0 {
            return Err(Error::InsufficientBalance);
        }

        write_balance(&env, &to, remaining);
        token::TokenClient::new(&env, &read_token(&env)?).transfer(
            &env.current_contract_address(),
            &to,
            &amount,
        );
        extend_instance_ttl(&env);

        Withdrawn {
            to,
            amount,
            balance: remaining,
        }
        .publish(&env);
        Ok(remaining)
    }

    /// Hands the vault to a new administrator.
    pub fn set_admin(env: Env, new_admin: Address) -> Result<(), Error> {
        require_admin(&env)?;
        env.storage().instance().set(&DataKey::Admin, &new_admin);
        extend_instance_ttl(&env);
        Ok(())
    }

    /// Public state. Reads need no authorization.
    pub fn balance(env: Env, who: Address) -> i128 {
        read_balance(&env, &who)
    }

    pub fn admin(env: Env) -> Result<Address, Error> {
        env.storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(Error::NotInitialized)
    }

    pub fn token(env: Env) -> Result<Address, Error> {
        read_token(&env)
    }
}

/// Authorizes the stored administrator, not an address the caller supplied.
fn require_admin(env: &Env) -> Result<(), Error> {
    let admin: Address = env
        .storage()
        .instance()
        .get(&DataKey::Admin)
        .ok_or(Error::NotInitialized)?;
    admin.require_auth();
    Ok(())
}

fn read_token(env: &Env) -> Result<Address, Error> {
    env.storage()
        .instance()
        .get(&DataKey::Token)
        .ok_or(Error::NotInitialized)
}

/// An absent entry means the depositor has never deposited, which is a balance
/// of zero rather than an error.
fn read_balance(env: &Env, who: &Address) -> i128 {
    let key = DataKey::Balance(who.clone());
    let balance = env.storage().persistent().get(&key).unwrap_or(0);
    if balance != 0 {
        env.storage()
            .persistent()
            .extend_ttl(&key, BALANCE_TTL_THRESHOLD, BALANCE_TTL_EXTEND_TO);
    }
    balance
}

fn write_balance(env: &Env, who: &Address, balance: i128) {
    let key = DataKey::Balance(who.clone());
    env.storage().persistent().set(&key, &balance);
    env.storage()
        .persistent()
        .extend_ttl(&key, BALANCE_TTL_THRESHOLD, BALANCE_TTL_EXTEND_TO);
}

fn extend_instance_ttl(env: &Env) {
    env.storage()
        .instance()
        .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);
}

#[cfg(test)]
mod test;
