#![no_std]
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey { Admin, Owner, Config, Value }

#[contract]
pub struct Safe;

#[contractimpl]
impl Safe {
    pub fn __constructor(env: Env, admin: Address) {
        env.storage().instance().set(&DataKey::Admin, &admin);
    }

    pub fn init(env: Env, owner: Address) {
        if env.storage().instance().has(&DataKey::Owner) {
            panic!("already initialized");
        }
        env.storage().instance().set(&DataKey::Owner, &owner);
    }

    pub fn configure(env: Env, value: i128) {
        if env.storage().instance().get(&DataKey::Config).is_some() {
            return;
        }
        env.storage().instance().set(&DataKey::Config, &value);
    }

    pub fn set_value(env: Env, value: i128) {
        env.storage().instance().set(&DataKey::Value, &value);
    }
}
