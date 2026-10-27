#![no_std]
use soroban_sdk::{contract, contractimpl, symbol_short, Bytes, Env};

#[contract]
pub struct Counter;

#[contractimpl]
impl Counter {
    // v2: same counter, plus three extra persistent writes and three sha256 host calls.
    pub fn bump(env: Env) -> u32 {
        let key = symbol_short!("COUNT");
        let n: u32 = env.storage().persistent().get(&key).unwrap_or(0) + 1;
        env.storage().persistent().set(&key, &n);

        let mut data = Bytes::from_array(&env, &n.to_be_bytes());
        for i in 0..3u32 {
            env.storage().persistent().set(&(symbol_short!("EXTRA"), i), &n);
            data = env.crypto().sha256(&data).into();
        }
        n
    }
}
