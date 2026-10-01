package domain_test

import (
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestRegisterInputValidate(t *testing.T) {
	const component, method = "RegisterInput", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-AUTH-REG-01", Method: method,
		Title:     "accepts valid input and normalizes email",
		Given:     "a register input whose email has surrounding spaces and mixed case",
		When:      "Validate is called",
		Then:      "no error is returned and the email is trimmed and lowercased",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "  Listener@Example.COM  "},
	}, func(t *testing.T, r testkit.Report) {
		var in domain.RegisterInput
		var err error
		r.Arrange(func(t *testing.T) {
			in = testkit.AuthMother().WithEmail("  Listener@Example.COM  ").BuildRegisterInput()
		})
		r.Act(func(t *testing.T) { err = in.Validate() })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, "listener@example.com", in.Email)
		})
	})

	for _, tc := range []struct {
		id, title, email, password string
		ok                         bool
		technique                  string
	}{
		{"DOM-AUTH-REG-02", "accepts password at minimum length 8", "listener@example.com", "12345678", true, testkit.TechniqueBoundary},
		{"DOM-AUTH-REG-03", "rejects password of length 7", "listener@example.com", "1234567", false, testkit.TechniqueBoundary},
		{"DOM-AUTH-REG-04", "rejects empty password", "listener@example.com", "", false, testkit.TechniqueBoundary},
		{"DOM-AUTH-REG-05", "rejects email without at sign", "listener.example.com", "password123", false, testkit.TechniqueEquivalence},
		{"DOM-AUTH-REG-06", "rejects blank email", "   ", "password123", false, testkit.TechniqueBoundary},
		{"DOM-AUTH-REG-07", "rejects invalid email before checking password", "invalid", "short", false, testkit.TechniqueDecisionTable},
	} {
		then := "no error is returned"
		if !tc.ok {
			then = "an invalid error is returned"
		}
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a register input with the given email and password",
			When:      "Validate is called",
			Then:      then,
			Technique: tc.technique,
			Params:    map[string]string{"email": tc.email, "password length": itoa(len(tc.password))},
		}, func(t *testing.T, r testkit.Report) {
			var in domain.RegisterInput
			var err error
			r.Arrange(func(t *testing.T) {
				in = testkit.AuthMother().WithEmail(tc.email).WithPassword(tc.password).BuildRegisterInput()
			})
			r.Act(func(t *testing.T) { err = in.Validate() })
			r.Assert(func(t *testing.T) {
				if tc.ok {
					assertNoError(t, err)
					return
				}
				assertErrorCode(t, domain.CodeInvalid, err)
			})
		})
	}
}

func TestRegisterInputUser(t *testing.T) {
	const component, method = "RegisterInput", "User"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-AUTH-USER-01", Method: method,
		Title:     "maps email and name",
		Given:     "a valid register input",
		When:      "User is called",
		Then:      "the user has the same email and name",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var in domain.RegisterInput
		var got domain.User
		r.Arrange(func(t *testing.T) { in = testkit.Auth.ValidRegister() })
		r.Act(func(t *testing.T) { got = in.User() })
		r.Assert(func(t *testing.T) {
			assertEqual(t, in.Email, got.Email)
			assertEqual(t, in.Name, got.Name)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-AUTH-USER-02", Method: method,
		Title:     "does not populate an id or leak the password",
		Given:     "a register input with password secret-value",
		When:      "User is called",
		Then:      "the user id is empty and no user field contains the password",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var in domain.RegisterInput
		var got domain.User
		r.Arrange(func(t *testing.T) {
			in = testkit.AuthMother().WithPassword("secret-value").BuildRegisterInput()
		})
		r.Act(func(t *testing.T) { got = in.User() })
		r.Assert(func(t *testing.T) {
			assertEqual(t, "", got.ID)
			assertEqual(t, false, got.Email == in.Password || got.Name == in.Password)
		})
	})
}

func TestLoginInputValidate(t *testing.T) {
	const component, method = "LoginInput", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-AUTH-LOGIN-01", Method: method,
		Title:     "accepts credentials and normalizes email",
		Given:     "a login input whose email has surrounding spaces and mixed case",
		When:      "Validate is called",
		Then:      "no error is returned and the email is trimmed and lowercased",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "  Listener@Example.COM  "},
	}, func(t *testing.T, r testkit.Report) {
		var in domain.LoginInput
		var err error
		r.Arrange(func(t *testing.T) {
			in = testkit.AuthMother().WithEmail("  Listener@Example.COM  ").BuildLoginInput()
		})
		r.Act(func(t *testing.T) { err = in.Validate() })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, "listener@example.com", in.Email)
		})
	})

	for _, tc := range []struct {
		id, title, email, password string
		ok                         bool
		technique                  string
	}{
		{"DOM-AUTH-LOGIN-02", "accepts a one-character password", "listener@example.com", "x", true, testkit.TechniqueBoundary},
		{"DOM-AUTH-LOGIN-03", "rejects blank email after normalization", "   ", "password123", false, testkit.TechniqueBoundary},
		{"DOM-AUTH-LOGIN-04", "rejects empty password", "listener@example.com", "", false, testkit.TechniqueBoundary},
	} {
		then := "no error is returned"
		if !tc.ok {
			then = "an invalid error is returned"
		}
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a login input with the given email and password",
			When:      "Validate is called",
			Then:      then,
			Technique: tc.technique,
			Params:    map[string]string{"email": tc.email, "password length": itoa(len(tc.password))},
		}, func(t *testing.T, r testkit.Report) {
			var in domain.LoginInput
			var err error
			r.Arrange(func(t *testing.T) {
				in = testkit.AuthMother().WithEmail(tc.email).WithPassword(tc.password).BuildLoginInput()
			})
			r.Act(func(t *testing.T) { err = in.Validate() })
			r.Assert(func(t *testing.T) {
				if tc.ok {
					assertNoError(t, err)
					return
				}
				assertErrorCode(t, domain.CodeInvalid, err)
			})
		})
	}
}
