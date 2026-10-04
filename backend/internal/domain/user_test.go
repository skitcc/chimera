package domain_test

import (
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestUserIDValidate(t *testing.T) {
	const component, method = "UserID", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-ID-01", Method: method,
		Title:     "accepts non-empty id",
		Given:     "a user id user-123",
		When:      "Validate is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": "user-123"},
	}, func(t *testing.T, r testkit.Report) {
		var id domain.UserID
		var err error
		r.Arrange(func(t *testing.T) { id = domain.UserID("user-123") })
		r.Act(func(t *testing.T) { err = id.Validate() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-ID-02", Method: method,
		Title:     "rejects empty id",
		Given:     "an empty user id",
		When:      "Validate is called",
		Then:      `an invalid error "user id is required" is returned`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var id domain.UserID
		var err error
		r.Arrange(func(t *testing.T) { id = domain.UserID("") })
		r.Act(func(t *testing.T) { err = id.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "user id is required", err)
		})
	})
}

func TestUserWriteValidate(t *testing.T) {
	const component, method = "UserWrite", "Validate"

	validCases := []struct {
		id, title, given, email, want string
		technique                     string
	}{
		{"DOM-USER-VAL-01", "accepts valid write and normalizes email", "a user write whose email has surrounding spaces and mixed case", "  Listener@Example.COM  ", "listener@example.com", testkit.TechniqueEquivalence},
		{"DOM-USER-VAL-02", "normalizes shortest padded email", "a user write with the shortest padded uppercase email", " A@B.C ", "a@b.c", testkit.TechniqueBoundary},
		{"DOM-USER-VAL-03", "accepts bare at sign as email", "a user write whose email is a single at sign", "@", "@", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range validCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "Validate is called",
			Then:      "no error is returned and the email becomes " + tc.want,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"email": tc.email},
		}, func(t *testing.T, r testkit.Report) {
			var write domain.UserWrite
			var err error
			r.Arrange(func(t *testing.T) { write = testkit.UserMother().WithEmail(tc.email).BuildWrite() })
			r.Act(func(t *testing.T) { err = write.Validate() })
			r.Assert(func(t *testing.T) {
				assertNoError(t, err)
				assertEqual(t, tc.want, write.Email)
			})
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-VAL-04", Method: method,
		Title:     "does not require password for update",
		Given:     "a user write with a valid email and an empty password",
		When:      "Validate is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"password length": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var write domain.UserWrite
		var err error
		r.Arrange(func(t *testing.T) { write = testkit.UserMother().WithPassword("").BuildWrite() })
		r.Act(func(t *testing.T) { err = write.Validate() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	invalidCases := []struct {
		id, title, email string
		technique        string
	}{
		{"DOM-USER-VAL-05", "rejects email without at sign", "listener.example.com", testkit.TechniqueEquivalence},
		{"DOM-USER-VAL-06", "rejects empty email", "", testkit.TechniqueBoundary},
		{"DOM-USER-VAL-07", "rejects single-character email", "a", testkit.TechniqueBoundary},
		{"DOM-USER-VAL-08", "rejects whitespace-only email", "   ", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range invalidCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a user write with the given email",
			When:      "Validate is called",
			Then:      `an invalid error "valid email is required" is returned`,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"email": tc.email},
		}, func(t *testing.T, r testkit.Report) {
			var write domain.UserWrite
			var err error
			r.Arrange(func(t *testing.T) { write = testkit.UserMother().WithEmail(tc.email).BuildWrite() })
			r.Act(func(t *testing.T) { err = write.Validate() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, "valid email is required", err)
			})
		})
	}
}

func TestUserWriteValidateCreate(t *testing.T) {
	const component, method = "UserWrite", "ValidateCreate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-CREATE-01", Method: method,
		Title:     "accepts password at minimum length 8",
		Given:     "a user write with a valid email and an 8-character password",
		When:      "ValidateCreate is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueBoundary,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"password length": "8"},
	}, func(t *testing.T, r testkit.Report) {
		var write domain.UserWrite
		var err error
		r.Arrange(func(t *testing.T) { write = testkit.UserMother().WithPassword("12345678").BuildWrite() })
		r.Act(func(t *testing.T) { err = write.ValidateCreate() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	cases := []struct {
		id, title, password string
	}{
		{"DOM-USER-CREATE-02", "rejects password of length 7", "1234567"},
		{"DOM-USER-CREATE-03", "rejects empty password", ""},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a user write with a valid email and a password shorter than 8 characters",
			When:      "ValidateCreate is called",
			Then:      `an invalid error "password must be at least 8 characters" is returned`,
			Technique: testkit.TechniqueBoundary,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"password length": itoa(len(tc.password))},
		}, func(t *testing.T, r testkit.Report) {
			var write domain.UserWrite
			var err error
			r.Arrange(func(t *testing.T) { write = testkit.UserMother().WithPassword(tc.password).BuildWrite() })
			r.Act(func(t *testing.T) { err = write.ValidateCreate() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, "password must be at least 8 characters", err)
			})
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-CREATE-04", Method: method,
		Title:     "normalizes valid email",
		Given:     "a user write whose email has surrounding spaces and mixed case and a valid password",
		When:      "ValidateCreate is called",
		Then:      "no error is returned and the email is trimmed and lowercased",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "  Listener@Example.COM  "},
	}, func(t *testing.T, r testkit.Report) {
		var write domain.UserWrite
		var err error
		r.Arrange(func(t *testing.T) {
			write = testkit.UserMother().WithEmail("  Listener@Example.COM  ").BuildWrite()
		})
		r.Act(func(t *testing.T) { err = write.ValidateCreate() })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, "listener@example.com", write.Email)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-CREATE-05", Method: method,
		Title:     "rejects invalid email before password",
		Given:     "a user write with an empty email and an empty password",
		When:      "ValidateCreate is called",
		Then:      `the email check wins: an invalid error "valid email is required" is returned`,
		Technique: testkit.TechniqueDecisionTable,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "", "password length": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var write domain.UserWrite
		var err error
		r.Arrange(func(t *testing.T) {
			write = testkit.UserMother().WithEmail("").WithPassword("").BuildWrite()
		})
		r.Act(func(t *testing.T) { err = write.ValidateCreate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "valid email is required", err)
		})
	})
}

func TestUserWriteUser(t *testing.T) {
	const component, method = "UserWrite", "User"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-MAP-01", Method: method,
		Title:     "maps write fields and supplied id",
		Given:     "a valid user write",
		When:      "User is called with id new-user",
		Then:      "the user has id new-user and the write's email and name",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": "new-user"},
	}, func(t *testing.T, r testkit.Report) {
		var write domain.UserWrite
		var got domain.User
		r.Arrange(func(t *testing.T) { write = testkit.UserMother().BuildWrite() })
		r.Act(func(t *testing.T) { got = write.User("new-user") })
		r.Assert(func(t *testing.T) {
			assertEqual(t, "new-user", got.ID)
			assertEqual(t, write.Email, got.Email)
			assertEqual(t, write.Name, got.Name)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-USER-MAP-02", Method: method,
		Title:     "allows an empty supplied id without leaking password",
		Given:     "a user write with password secret-value",
		When:      "User is called with an empty id",
		Then:      "the user id is empty and no user field contains the password",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var write domain.UserWrite
		var got domain.User
		r.Arrange(func(t *testing.T) { write = testkit.UserMother().WithPassword("secret-value").BuildWrite() })
		r.Act(func(t *testing.T) { got = write.User("") })
		r.Assert(func(t *testing.T) {
			assertEqual(t, "", got.ID)
			assertEqual(t, false, got.Email == write.Password || got.Name == write.Password)
		})
	})
}
