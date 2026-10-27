#![no_std]
use soroban_sdk::{contract, contractimpl, symbol_short, Env};

#[contract]
pub struct Counter;

#[contractimpl]
impl Counter {
    // v1: one persistent read and one persistent write.
    pub fn bump(env: Env) -> u32 {
        let key = symbol_short!("COUNT");
        let n: u32 = env.storage().persistent().get(&key).unwrap_or(0) + 1;
        env.storage().persistent().set(&key, &n);
        n
    }
}
