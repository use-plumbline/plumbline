#![no_std]
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
#[derive(Clone)]
pub enum DataKey { Admin, Owner, Config, Value }

#[contract]
pub struct Unsafe;

#[contractimpl]
impl Unsafe {
    pub fn initialize(env: Env, admin: Address) {
        env.storage().instance().set(&DataKey::Admin, &admin); //~ missing-reinit-guard
    }

    pub fn set_owner(env: Env, owner: Address) {
        env.storage().instance().set(&DataKey::Owner, &owner); //~ missing-reinit-guard
    }

    pub fn update_config(env: Env, value: i128) {
        env.storage().instance().update(&DataKey::Config, value); //~ missing-reinit-guard
    }

    pub fn setup(env: Env, value: i128) {
        env.storage().persistent().set(&DataKey::Value, &value); //~ missing-reinit-guard
    }

    pub fn write_value(env: Env, value: i128) {
        env.storage().instance().set(&DataKey::Value, &value);
    }
}