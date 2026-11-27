Feature: Mandatory email second factor and manual password lifecycle
  Background:
    Given a fresh technical account

  Scenario: Password alone grants no protected access
    When I submit the correct password
    Then a challenge is returned without a token
    And protected access is denied
    When I confirm the emailed login code
    Then protected access is allowed

  Scenario: Invalid password and invalid second factor
    When I submit a wrong password
    Then authentication is rejected
    When I submit the correct password
    And I confirm an incorrect login code
    Then authentication is rejected

  Scenario: Expired codes cannot be used
    When I submit the correct password
    And the challenge expires
    And I confirm the emailed login code
    Then authentication is rejected

  Scenario: A second factor is single use
    When I submit the correct password
    And I confirm the emailed login code
    Then protected access is allowed
    When I confirm the emailed login code
    Then authentication is rejected

  Scenario: Four password errors still allow login and a successful login resets attempts
    When I submit a wrong password 4 times
    And I sign in with both factors
    Then protected access is allowed
    When I submit a wrong password 4 times
    And I sign in with both factors
    Then protected access is allowed

  Scenario: Four second factor errors still allow the correct code
    When I submit the correct password
    And I confirm an incorrect login code 4 times
    And I confirm the emailed login code
    Then protected access is allowed

  Scenario: Password and second factor errors share the same limit
    When I submit a wrong password 3 times
    And I submit the correct password
    Then a challenge is returned without a token
    When I confirm an incorrect login code 2 times
    And I confirm the emailed login code
    Then authentication is rejected
    When I submit the correct password
    Then authentication is rejected

  Scenario: Fifth password error locks the account and recovery unlocks it
    When I submit a wrong password 5 times
    And I submit the correct password
    Then authentication is rejected
    When I recover with an emailed code and a new password
    Then no session is issued
    And the old password is rejected
    When I sign in with the new password and email code
    Then protected access is allowed

  Scenario: Recovery revokes a previously issued token
    When I sign in with both factors
    And I submit a wrong password 5 times
    And I recover with an emailed code and a new password
    Then no session is issued
    And the old token is rejected
    When I sign in with the new password and email code
    Then protected access is allowed

  Scenario: Fifth second factor error locks the account
    When I submit the correct password
    And I confirm an incorrect login code 5 times
    And I confirm the emailed login code
    Then authentication is rejected
    When I submit the correct password
    Then authentication is rejected

  Scenario: Manual password change revokes previous sessions
    When I sign in with both factors
    And I change the password with the current password and email code
    Then no session is issued
    And the old token is rejected
    And the old password is rejected
    When I sign in with the new password and email code
    Then protected access is allowed

  Scenario: Password change requires current password and a different new password
    When I sign in with both factors
    And I request a password change with a wrong current password
    Then authentication is rejected
    When I attempt to reuse the current password
    Then the request is invalid
    When I attempt to change the password through the profile
    Then the request is invalid

  Scenario: Recovery has the same public contract for an unknown email
    When I request recovery for an unknown email
    Then recovery gives a generic response
