#![no_std]
//! Every marked expression can carry a token-sized integer past its bounds.
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Balance(Address),
}

#[contract]
pub struct Vault;

#[contractimpl]
impl Vault {
    /// Both operands are i128 parameters.
    pub fn transfer(env: Env, from: Address, to: Address, amount: i128) {
        from.require_auth();
        let from_balance: i128 = env
            .storage()
            .persistent()
            .get(&DataKey::Balance(from.clone()))
            .unwrap_or(0);
        let to_balance: i128 = env
            .storage()
            .persistent()
            .get(&DataKey::Balance(to.clone()))
            .unwrap_or(0);
        env.storage()
            .persistent()
            .set(&DataKey::Balance(from), &(from_balance - amount)); //~ unchecked-arithmetic
        env.storage()
            .persistent()
            .set(&DataKey::Balance(to), &(to_balance + amount)); //~ unchecked-arithmetic
    }

    /// A compound assignment overflows just as readily as a binary operator.
    pub fn accumulate(_env: Env, amount: i128) -> i128 {
        let mut total = 0i128;
        total += amount; //~ unchecked-arithmetic
        total
    }

    /// The width is inferred through an unannotated binding: `fee` is i128
    /// because `amount` is.
    pub fn charge(_env: Env, amount: i128) -> i128 {
        let fee = amount * 3; //~ unchecked-arithmetic
        amount - fee //~ unchecked-arithmetic
    }

    /// A cast makes the width explicit.
    pub fn widen(_env: Env, amount: u64) -> i128 {
        (amount as i128) * 1000 //~ unchecked-arithmetic
    }

    /// Neither operand can be resolved, so the expression is reported rather
    /// than assumed safe.
    pub fn opaque(env: Env, who: Address) -> i128 {
        Self::rate(&env) * Self::weight(&env, who) //~ unchecked-arithmetic
    }

    fn rate(_env: &Env) -> i128 {
        1
    }

    fn weight(_env: &Env, _who: Address) -> i128 {
        1
    }
}
