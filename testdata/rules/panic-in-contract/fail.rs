#![no_std]
//! Every marked line aborts the invocation with an opaque host error that a
//! client cannot match on.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Balance(Address),
}

#[contract]
pub struct Vault;

/// Not an entry point. This rule does not yet follow calls into helpers, so
/// the unwrap here is not reported.
fn load_admin(env: &Env) -> Address {
    env.storage().instance().get(&DataKey::Admin).unwrap()
}

#[contractimpl]
impl Vault {
    pub fn withdraw(env: Env, from: Address, amount: i128) -> i128 {
        from.require_auth();
        let balance: i128 = env
            .storage()
            .persistent()
            .get(&DataKey::Balance(from.clone()))
            .unwrap(); //~ panic-in-contract
        if balance < amount {
            panic!("insufficient balance"); //~ panic-in-contract
        }
        let remaining = balance - amount;
        env.storage()
            .persistent()
            .set(&DataKey::Balance(from), &remaining);
        remaining
    }

    pub fn admin(env: Env) -> Address {
        env.storage()
            .instance()
            .get(&DataKey::Admin)
            .expect("not initialized") //~ panic-in-contract
    }

    /// A panic inside a closure still aborts the invocation.
    pub fn balance(env: Env, who: Address) -> i128 {
        env.storage()
            .persistent()
            .get(&DataKey::Balance(who))
            .unwrap_or_else(|| panic!("no balance")) //~ panic-in-contract
    }
}
