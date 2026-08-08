#![cfg(test)]

use super::*;
use soroban_sdk::{
    testutils::{Address as _, MockAuth, MockAuthInvoke},
    token::StellarAssetClient,
    Env, IntoVal,
};

struct Setup {
    env: Env,
    admin: Address,
    user: Address,
    token: Address,
    vault: VaultClient<'static>,
}

fn setup() -> Setup {
    let env = Env::default();
    env.mock_all_auths();

    let admin = Address::generate(&env);
    let user = Address::generate(&env);

    let asset = env.register_stellar_asset_contract_v2(admin.clone());
    let token = asset.address();
    StellarAssetClient::new(&env, &token).mint(&user, &1_000);

    let contract_id = env.register(Vault, (admin.clone(), token.clone()));
    let vault = VaultClient::new(&env, &contract_id);

    Setup {
        env,
        admin,
        user,
        token,
        vault,
    }
}

#[test]
fn deposit_credits_the_depositor_and_moves_the_tokens() {
    let s = setup();
    assert_eq!(s.vault.deposit(&s.user, &400), 400);
    assert_eq!(s.vault.balance(&s.user), 400);

    let token = token::TokenClient::new(&s.env, &s.token);
    assert_eq!(token.balance(&s.user), 600);
    assert_eq!(token.balance(&s.vault.address), 400);
}

#[test]
fn deposits_accumulate() {
    let s = setup();
    s.vault.deposit(&s.user, &400);
    assert_eq!(s.vault.deposit(&s.user, &100), 500);
}

#[test]
fn withdraw_returns_the_tokens() {
    let s = setup();
    s.vault.deposit(&s.user, &400);
    assert_eq!(s.vault.withdraw(&s.user, &150), 250);
    assert_eq!(token::TokenClient::new(&s.env, &s.token).balance(&s.user), 750);
}

#[test]
fn withdrawing_more_than_the_balance_is_a_typed_error() {
    let s = setup();
    s.vault.deposit(&s.user, &100);
    assert_eq!(
        s.vault.try_withdraw(&s.user, &101),
        Err(Ok(Error::InsufficientBalance))
    );
}

#[test]
fn non_positive_amounts_are_rejected() {
    let s = setup();
    assert_eq!(
        s.vault.try_deposit(&s.user, &0),
        Err(Ok(Error::InvalidAmount))
    );
    assert_eq!(
        s.vault.try_withdraw(&s.user, &-1),
        Err(Ok(Error::InvalidAmount))
    );
}

#[test]
fn balance_of_a_stranger_is_zero() {
    let s = setup();
    assert_eq!(s.vault.balance(&Address::generate(&s.env)), 0);
}

#[test]
fn admin_can_hand_over_the_vault() {
    let s = setup();
    let next = Address::generate(&s.env);
    s.vault.set_admin(&next);
    assert_eq!(s.vault.admin(), next);
}

/// The point of require_admin: authorization is checked against the stored
/// admin, so an arbitrary caller cannot take the vault.
#[test]
fn a_stranger_cannot_take_the_vault() {
    let env = Env::default();
    let admin = Address::generate(&env);
    let stranger = Address::generate(&env);
    let asset = env.register_stellar_asset_contract_v2(admin.clone());

    let contract_id = env.register(Vault, (admin.clone(), asset.address()));
    let vault = VaultClient::new(&env, &contract_id);

    let attempt = vault
        .mock_auths(&[MockAuth {
            address: &stranger,
            invoke: &MockAuthInvoke {
                contract: &contract_id,
                fn_name: "set_admin",
                args: (stranger.clone(),).into_val(&env),
                sub_invokes: &[],
            },
        }])
        .try_set_admin(&stranger);

    assert!(attempt.is_err(), "a stranger reassigned the admin");
}
